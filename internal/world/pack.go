package world

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
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
	workers := runtime.NumCPU()
	if workers < 4 {
		workers = 4
	}
	db.SetMaxOpenConns(workers + 2)
	db.SetMaxIdleConns(workers)
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

// packPos encodes a tile position into a single uint64.
// Layout: x (bits 20-35), y (bits 4-19), z (bits 0-3). 36 bits in BIGINT.
func packPos(x, y uint16, z uint8) uint64 {
	return uint64(x)<<20 | uint64(y)<<4 | uint64(z&0x0F)
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
	// map_items table is created by migrations-v2, no need to create here.
	// otconv only touches map + map_items, never items (game domain).
	if _, err := db.Exec("DELETE FROM `map_items`"); err != nil {
		return 0, 0, fmt.Errorf("clear map_items: %w", err)
	}
	if _, err := db.Exec("DELETE FROM `map`"); err != nil {
		return 0, 0, fmt.Errorf("clear map: %w", err)
	}

	// Group tiles by floor (z-level).
	floors := make(map[uint8][]gamedata.MapTile)
	for _, t := range tiles {
		floors[t.Z] = append(floors[t.Z], t)
	}

	const chunkSize = 10000

	workers := runtime.NumCPU()
	if workers < 4 {
		workers = 4
	}

	var totalItems atomic.Int64
	var done atomic.Int64
	var firstErr error
	var errOnce sync.Once

	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup

	// Count total chunks across all floors for progress reporting.
	totalChunks := 0
	for _, floorTiles := range floors {
		totalChunks += (len(floorTiles) + chunkSize - 1) / chunkSize
	}

	// Launch one goroutine per floor level (z=0..15).
	for z := uint8(0); z <= 15; z++ {
		floorTiles, ok := floors[z]
		if !ok || len(floorTiles) == 0 {
			continue
		}

		// Within each floor, batch tiles into chunks.
		for i := 0; i < len(floorTiles); i += chunkSize {
			end := i + chunkSize
			if end > len(floorTiles) {
				end = len(floorTiles)
			}
			chunkTiles := floorTiles[i:end]
			chunkIdx := int(z)*1000 + i/chunkSize // unique label per chunk

			sem <- struct{}{} // acquire worker slot
			wg.Add(1)
			go func(ci int, batch []gamedata.MapTile) {
				defer wg.Done()
				defer func() { <-sem }() // release worker slot

				tx, err := db.Begin()
				if err != nil {
					errOnce.Do(func() { firstErr = fmt.Errorf("begin chunk %d: %w", ci, err) })
					return
				}

				n, err := insertTileChunk(tx, batch)
				if err != nil {
					tx.Rollback()
					errOnce.Do(func() { firstErr = fmt.Errorf("insert chunk %d: %w", ci, err) })
					return
				}

				if err := tx.Commit(); err != nil {
					errOnce.Do(func() { firstErr = fmt.Errorf("commit chunk %d: %w", ci, err) })
					return
				}

				totalItems.Add(int64(n))
				d := done.Add(1)
				fmt.Printf("  tiles: %d / %d chunks\n", d, totalChunks)
			}(chunkIdx, chunkTiles)
		}
	}

	wg.Wait()

	if firstErr != nil {
		return 0, 0, firstErr
	}

	return len(tiles), int(totalItems.Load()), nil
}

// insertTileChunk inserts tiles into the `map` table and their items into the `items` table.
func insertTileChunk(tx *sql.Tx, tiles []gamedata.MapTile) (int, error) {
	if len(tiles) == 0 {
		return 0, nil
	}

	if _, err := tx.Exec("SET FOREIGN_KEY_CHECKS=0, UNIQUE_CHECKS=0"); err != nil {
		return 0, err
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
		args := make([]interface{}, 0, len(batch)*3)
		for j := range batch {
			rows[j] = "(?,?,?)"
			args = append(args,
				packPos(batch[j].X, batch[j].Y, batch[j].Z),
				batch[j].Flags, batch[j].HouseID,
			)
		}

		q := "INSERT INTO `map` (pos, flags, house_id) VALUES " +
			strings.Join(rows, ",") +
			" ON DUPLICATE KEY UPDATE flags=VALUES(flags), house_id=VALUES(house_id)"

		if _, err := tx.Exec(q, args...); err != nil {
			return err
		}
	}
	return nil
}

func bulkInsertTileItems(tx *sql.Tx, tiles []gamedata.MapTile) (int, error) {
	const batchSize = 500

	// Pre-collect flat items (no SubItems) for multi-row INSERT.
	type flatItem struct {
		tilePos uint64
		typeID  uint16
		count   uint8
		slot    int
		attrs   *string
	}
	// Pre-collect container parents to insert sequentially (need LastInsertId).
	type containerItem struct {
		tilePos uint64
		ri      *gamedata.RichItem
		slot    int
		tileX   uint16
		tileY   uint16
		tileZ   uint8
	}

	flat := make([]flatItem, 0, len(tiles)*3)
	var containers []containerItem

	for i := range tiles {
		t := &tiles[i]
		tilePos := packPos(t.X, t.Y, t.Z)

		if t.GroundID != 0 {
			flat = append(flat, flatItem{tilePos, t.GroundID, 1, 0, nil})
		}

		for j := range t.RichItems {
			ri := &t.RichItems[j]
			slot := j + 1
			c := ri.Count
			if c == 0 {
				c = 1
			}
			if len(ri.SubItems) > 0 {
				containers = append(containers, containerItem{tilePos, ri, slot, t.X, t.Y, t.Z})
			} else {
				attrs := richItemAttrs(ri)
				flat = append(flat, flatItem{tilePos, ri.ServerID, c, slot, attrs})
			}
		}
	}

	count := 0

	// Pass 1: bulk insert all flat items.
	for i := 0; i < len(flat); i += batchSize {
		end := i + batchSize
		if end > len(flat) {
			end = len(flat)
		}
		batch := flat[i:end]

		rows := make([]string, len(batch))
		args := make([]interface{}, 0, len(batch)*5)
		for j := range batch {
			rows[j] = "(?,?,?,?,?)"
			args = append(args,
				batch[j].tilePos, batch[j].typeID,
				batch[j].count, batch[j].slot, batch[j].attrs,
			)
		}

		q := "INSERT INTO `map_items` (tile_pos, type_id, count, slot, attributes) VALUES " +
			strings.Join(rows, ",")

		if _, err := tx.Exec(q, args...); err != nil {
			return 0, err
		}
		count += len(batch)
	}

	// Pass 2: containers — insert parent (need LastInsertId), then batch children.
	if len(containers) > 0 {
		parentStmt, err := tx.Prepare(
			"INSERT INTO `map_items` (tile_pos, type_id, count, slot, attributes) VALUES (?,?,?,?,?)")
		if err != nil {
			return 0, err
		}
		defer parentStmt.Close()

		for _, ci := range containers {
			attrs := richItemAttrs(ci.ri)
			c := ci.ri.Count
			if c == 0 {
				c = 1
			}

			res, err := parentStmt.Exec(ci.tilePos, ci.ri.ServerID, c, ci.slot, attrs)
			if err != nil {
				return 0, fmt.Errorf("insert container at (%d,%d,%d) slot %d: %w",
					ci.tileX, ci.tileY, ci.tileZ, ci.slot, err)
			}
			count++

			parentID, _ := res.LastInsertId()
			subs := ci.ri.SubItems

			// Batch all sub-items of this container in one multi-row INSERT.
			if len(subs) > 0 {
				rows := make([]string, len(subs))
				args := make([]interface{}, 0, len(subs)*6)
				for s := range subs {
					rows[s] = "(?,?,?,?,?,?)"
					sc := subs[s].Count
					if sc == 0 {
						sc = 1
					}
					subAttrs := richItemAttrs(&subs[s])
					args = append(args,
						ci.tilePos, subs[s].ServerID, sc, s, parentID, subAttrs,
					)
				}

				q := "INSERT INTO `map_items` (tile_pos, type_id, count, slot, parent_id, attributes) VALUES " +
					strings.Join(rows, ",")

				if _, err := tx.Exec(q, args...); err != nil {
					return 0, fmt.Errorf("insert sub-items at (%d,%d,%d) parent %d: %w",
						ci.tileX, ci.tileY, ci.tileZ, parentID, err)
				}
				count += len(subs)
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
