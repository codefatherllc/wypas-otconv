package world

import (
	"database/sql"
	"flag"
	"fmt"
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

	tileCount, err := seedTiles(db, world.Tiles)
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

	fmt.Printf("seeded: %d tiles, %d spawns, %d houses, %d towns, %d waypoints\n",
		tileCount, spawnCount, houseCount, townCount, wpCount)
	return nil
}

func seedTiles(db *sql.DB, tiles []gamedata.MapTile) (int, error) {
	if _, err := db.Exec("DELETE FROM map_tiles"); err != nil {
		return 0, err
	}

	const chunkSize = 10000
	for i := 0; i < len(tiles); i += chunkSize {
		end := i + chunkSize
		if end > len(tiles) {
			end = len(tiles)
		}

		tx, err := db.Begin()
		if err != nil {
			return 0, fmt.Errorf("begin chunk %d: %w", i/chunkSize, err)
		}

		if err := gamedata.BulkInsertTiles(tx, tiles[i:end]); err != nil {
			tx.Rollback()
			return 0, fmt.Errorf("insert chunk %d: %w", i/chunkSize, err)
		}

		if err := tx.Commit(); err != nil {
			return 0, fmt.Errorf("commit chunk %d: %w", i/chunkSize, err)
		}

		fmt.Printf("  tiles: %d / %d\n", end, len(tiles))
	}

	return len(tiles), nil
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

	if _, err := tx.Exec("DELETE FROM map_spawn_entries"); err != nil {
		return 0, err
	}
	if _, err := tx.Exec("DELETE FROM map_spawns"); err != nil {
		return 0, err
	}

	spawnStmt, err := tx.Prepare("INSERT INTO map_spawns (center_x, center_y, center_z, radius) VALUES (?, ?, ?, ?)")
	if err != nil {
		return 0, err
	}
	defer spawnStmt.Close()

	entryStmt, err := tx.Prepare("INSERT INTO map_spawn_entries (spawn_id, name, type, offset_x, offset_y, offset_z, spawntime) VALUES (?, ?, ?, ?, ?, ?, ?)")
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
			if _, err := entryStmt.Exec(spawnID, c.Name, c.Type, c.OffsetX, c.OffsetY, c.OffsetZ, c.SpawnTime); err != nil {
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

	if _, err := tx.Exec("DELETE FROM map_houses"); err != nil {
		return 0, err
	}

	stmt, err := tx.Prepare("INSERT INTO map_houses (id, name, entry_x, entry_y, entry_z, rent, town_id, size) VALUES (?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	for _, h := range houses {
		if _, err := stmt.Exec(h.ID, h.Name, h.EntryX, h.EntryY, h.EntryZ, h.Rent, h.TownID, h.Size); err != nil {
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

	if _, err := tx.Exec("DELETE FROM map_towns"); err != nil {
		return 0, err
	}

	stmt, err := tx.Prepare("INSERT INTO map_towns (id, name, entry_x, entry_y, entry_z) VALUES (?, ?, ?, ?, ?)")
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	for _, t := range towns {
		if _, err := stmt.Exec(t.ID, t.Name, t.EntryX, t.EntryY, t.EntryZ); err != nil {
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

	if _, err := tx.Exec("DELETE FROM map_waypoints"); err != nil {
		return 0, err
	}

	stmt, err := tx.Prepare("INSERT INTO map_waypoints (name, x, y, z) VALUES (?, ?, ?, ?)")
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
