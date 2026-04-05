package world

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/codefatherllc/wypas-lib/gamedata"
	"github.com/codefatherllc/wypas-lib/otbm"
)

func Seed(args []string) error {
	fs := flag.NewFlagSet("map seed", flag.ExitOnError)
	otbmPath := fs.String("otbm", "", "path to Map.otbm")
	spawnsPath := fs.String("spawns", "", "path to spawns.xml")
	housesPath := fs.String("houses", "", "path to houses.xml")
	dsn := fs.String("dsn", "", "database DSN")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *otbmPath == "" || *dsn == "" {
		return fmt.Errorf("--otbm and --dsn are required")
	}

	var opts []otbm.WorldOption
	if *spawnsPath != "" {
		opts = append(opts, otbm.WithSpawns(*spawnsPath))
	}
	if *housesPath != "" {
		opts = append(opts, otbm.WithHouses(*housesPath))
	}

	world, err := otbm.LoadWorld(*otbmPath, opts...)
	if err != nil {
		return fmt.Errorf("load world: %w", err)
	}

	db, err := sql.Open("mysql", *dsn)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return fmt.Errorf("ping db: %w", err)
	}

	tileCount, itemCount, err := seedTiles(db, world.Tiles)
	if err != nil {
		return fmt.Errorf("seed tiles: %w", err)
	}

	spawnCount, err := seedSpawns(db, world.Spawns)
	if err != nil {
		return fmt.Errorf("seed spawns: %w", err)
	}

	houseCount, err := seedHouses(db, world.Houses)
	if err != nil {
		return fmt.Errorf("seed houses: %w", err)
	}

	townCount, err := seedTowns(db, world.Towns)
	if err != nil {
		return fmt.Errorf("seed towns: %w", err)
	}

	wpCount, err := seedWaypoints(db, world.Waypoints)
	if err != nil {
		return fmt.Errorf("seed waypoints: %w", err)
	}

	fmt.Printf("seeded: %d tiles, %d items, %d spawns, %d houses, %d towns, %d waypoints\n",
		tileCount, itemCount, spawnCount, houseCount, townCount, wpCount)
	return nil
}

// packPos encodes a tile position into a single uint64 for use as owner_id.
func packPos(x, y uint16, z uint8) uint64 {
	return uint64(x)<<24 | uint64(y)<<8 | uint64(z)
}

// richItemAttrs builds a JSON attribute map from a RichItem, omitting zero/empty values.
func richItemAttrs(ri *gamedata.RichItem) *string {
	m := make(map[string]interface{})

	if ri.ActionID != 0 {
		m["action_id"] = ri.ActionID
	}
	if ri.UniqueID != 0 {
		m["unique_id"] = ri.UniqueID
	}
	if ri.TeleDestX != 0 || ri.TeleDestY != 0 || ri.TeleDestZ != 0 {
		m["tele_dest"] = map[string]interface{}{
			"x": ri.TeleDestX,
			"y": ri.TeleDestY,
			"z": ri.TeleDestZ,
		}
	}
	if ri.DoorID != 0 {
		m["door_id"] = ri.DoorID
	}
	if ri.DepotID != 0 {
		m["depot_id"] = ri.DepotID
	}
	if ri.Text != "" {
		m["text"] = ri.Text
	}
	if ri.Description != "" {
		m["description"] = ri.Description
	}
	if ri.Charges != 0 {
		m["charges"] = ri.Charges
	}
	if ri.RuneCharges != 0 {
		m["rune_charges"] = ri.RuneCharges
	}
	if ri.Duration != 0 {
		m["duration"] = ri.Duration
	}
	if ri.WrittenDate != 0 {
		m["written_date"] = ri.WrittenDate
	}
	if ri.WrittenBy != "" {
		m["written_by"] = ri.WrittenBy
	}
	if ri.SleeperGUID != 0 {
		m["sleeper_guid"] = ri.SleeperGUID
	}
	if ri.SleepStart != 0 {
		m["sleep_start"] = ri.SleepStart
	}

	if len(m) == 0 {
		return nil
	}

	b, err := json.Marshal(m)
	if err != nil {
		return nil
	}
	s := string(b)
	return &s
}

func seedTiles(db *sql.DB, tiles []gamedata.MapTile) (int, int, error) {
	if _, err := db.Exec("DELETE FROM `items` WHERE owner_type IN ('map', 'container')"); err != nil {
		return 0, 0, fmt.Errorf("clear map/container items: %w", err)
	}
	if _, err := db.Exec("DELETE FROM `map`"); err != nil {
		return 0, 0, fmt.Errorf("clear map: %w", err)
	}

	totalItems := 0
	const chunkSize = 10000
	for i := 0; i < len(tiles); i += chunkSize {
		end := i + chunkSize
		if end > len(tiles) {
			end = len(tiles)
		}

		tx, err := db.Begin()
		if err != nil {
			return 0, 0, fmt.Errorf("begin chunk %d: %w", i/chunkSize, err)
		}

		n, err := insertTileChunk(tx, tiles[i:end])
		if err != nil {
			tx.Rollback()
			return 0, 0, fmt.Errorf("insert chunk %d: %w", i/chunkSize, err)
		}
		totalItems += n

		if err := tx.Commit(); err != nil {
			return 0, 0, fmt.Errorf("commit chunk %d: %w", i/chunkSize, err)
		}

		fmt.Printf("  tiles: %d / %d (%d items)\n", end, len(tiles), totalItems)
	}

	return len(tiles), totalItems, nil
}

// insertTileChunk inserts tiles into the `map` table and their items into the `items` table.
func insertTileChunk(tx *sql.Tx, tiles []gamedata.MapTile) (int, error) {
	if len(tiles) == 0 {
		return 0, nil
	}

	if err := bulkInsertMapTiles(tx, tiles); err != nil {
		return 0, err
	}

	itemCount, err := bulkInsertTileItems(tx, tiles)
	if err != nil {
		return 0, err
	}

	return itemCount, nil
}

func bulkInsertMapTiles(tx *sql.Tx, tiles []gamedata.MapTile) error {
	const batchSize = 500
	for i := 0; i < len(tiles); i += batchSize {
		end := i + batchSize
		if end > len(tiles) {
			end = len(tiles)
		}
		batch := tiles[i:end]

		rows := make([]string, len(batch))
		args := make([]interface{}, 0, len(batch)*6)
		for j := range batch {
			rows[j] = "(?,?,?,?,?,?)"
			args = append(args,
				batch[j].X, batch[j].Y, batch[j].Z,
				batch[j].GroundID, batch[j].Flags, batch[j].HouseID,
			)
		}

		q := "INSERT INTO `map` (x, y, z, ground_id, flags, house_id) VALUES " +
			strings.Join(rows, ",") +
			" ON DUPLICATE KEY UPDATE ground_id=VALUES(ground_id), flags=VALUES(flags), " +
			"house_id=VALUES(house_id)"

		if _, err := tx.Exec(q, args...); err != nil {
			return err
		}
	}
	return nil
}

func bulkInsertTileItems(tx *sql.Tx, tiles []gamedata.MapTile) (int, error) {
	stmt, err := tx.Prepare(
		"INSERT INTO `items` (type_id, count, owner_type, owner_id, slot, attributes) " +
			"VALUES (?, ?, 'map', ?, ?, ?)")
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	containerStmt, err := tx.Prepare(
		"INSERT INTO `items` (type_id, count, owner_type, owner_id, slot, attributes) " +
			"VALUES (?, ?, 'container', ?, ?, ?)")
	if err != nil {
		return 0, err
	}
	defer containerStmt.Close()

	count := 0
	for i := range tiles {
		t := &tiles[i]
		ownerID := packPos(t.X, t.Y, t.Z)

		for slot, ri := range t.RichItems {
			attrs := richItemAttrs(&ri)
			c := ri.Count
			if c == 0 {
				c = 1
			}

			res, err := stmt.Exec(ri.ServerID, c, ownerID, slot, attrs)
			if err != nil {
				return 0, fmt.Errorf("insert item at (%d,%d,%d) slot %d: %w", t.X, t.Y, t.Z, slot, err)
			}
			count++

			// Handle sub-items (items inside containers on the tile)
			if len(ri.SubItems) > 0 {
				parentID, _ := res.LastInsertId()
				for subSlot, sub := range ri.SubItems {
					subAttrs := richItemAttrs(&sub)
					sc := sub.Count
					if sc == 0 {
						sc = 1
					}
					if _, err := containerStmt.Exec(sub.ServerID, sc, parentID, 100+subSlot, subAttrs); err != nil {
						return 0, fmt.Errorf("insert sub-item at (%d,%d,%d) parent %d slot %d: %w",
							t.X, t.Y, t.Z, parentID, subSlot, err)
					}
					count++
				}
			}
		}
	}

	return count, nil
}

func seedSpawns(db *sql.DB, spawns []gamedata.Spawn) (int, error) {
	if len(spawns) == 0 {
		return 0, nil
	}

	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM `spawn`"); err != nil {
		return 0, err
	}
	if _, err := tx.Exec("DELETE FROM `spawns`"); err != nil {
		return 0, err
	}

	spawnStmt, err := tx.Prepare("INSERT INTO `spawns` (center_x, center_y, center_z, radius) VALUES (?, ?, ?, ?)")
	if err != nil {
		return 0, err
	}
	defer spawnStmt.Close()

	entryStmt, err := tx.Prepare(
		"INSERT INTO `spawn` (spawn_id, name, type, offset_x, offset_y, offset_z, spawntime, direction) " +
			"VALUES (?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		return 0, err
	}
	defer entryStmt.Close()

	for _, s := range spawns {
		res, err := spawnStmt.Exec(s.CenterX, s.CenterY, s.CenterZ, s.Radius)
		if err != nil {
			return 0, err
		}
		spawnID, _ := res.LastInsertId()
		for _, c := range s.Creatures {
			if _, err := entryStmt.Exec(spawnID, c.Name, c.Type, c.OffsetX, c.OffsetY, c.OffsetZ, c.SpawnTime, c.Direction); err != nil {
				return 0, err
			}
		}
	}

	return len(spawns), tx.Commit()
}

func seedHouses(db *sql.DB, houses []gamedata.House) (int, error) {
	if len(houses) == 0 {
		return 0, nil
	}

	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM `houses`"); err != nil {
		return 0, err
	}

	stmt, err := tx.Prepare(
		"INSERT INTO `houses` (id, name, entry_x, entry_y, entry_z, town_id, size, rent, guildhall) " +
			"VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	for _, h := range houses {
		if _, err := stmt.Exec(h.ID, h.Name, h.EntryX, h.EntryY, h.EntryZ, h.TownID, h.Size, h.Rent, h.Guildhall); err != nil {
			return 0, err
		}
	}

	return len(houses), tx.Commit()
}

func seedTowns(db *sql.DB, towns []gamedata.Town) (int, error) {
	if len(towns) == 0 {
		return 0, nil
	}

	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM `towns`"); err != nil {
		return 0, err
	}

	stmt, err := tx.Prepare("INSERT INTO `towns` (id, name, x, y, z) VALUES (?, ?, ?, ?, ?)")
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	for _, t := range towns {
		if _, err := stmt.Exec(t.ID, t.Name, t.X, t.Y, t.Z); err != nil {
			return 0, err
		}
	}

	return len(towns), tx.Commit()
}

func seedWaypoints(db *sql.DB, waypoints []gamedata.Waypoint) (int, error) {
	if len(waypoints) == 0 {
		return 0, nil
	}

	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM `waypoints`"); err != nil {
		return 0, err
	}

	stmt, err := tx.Prepare("INSERT INTO `waypoints` (name, x, y, z) VALUES (?, ?, ?, ?)")
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	for _, w := range waypoints {
		if _, err := stmt.Exec(w.Name, w.X, w.Y, w.Z); err != nil {
			return 0, err
		}
	}

	return len(waypoints), tx.Commit()
}
