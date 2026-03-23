package world

import (
	"database/sql"
	"encoding/binary"
	"encoding/xml"
	"flag"
	"fmt"
	"os"

	_ "github.com/go-sql-driver/mysql"

	"github.com/codefatherllc/wypas-lib/otbm"
)

type xmlSpawns struct {
	Spawns []xmlSpawn `xml:"spawn"`
}

type xmlSpawn struct {
	CenterX  uint16       `xml:"centerx,attr"`
	CenterY  uint16       `xml:"centery,attr"`
	CenterZ  uint8        `xml:"centerz,attr"`
	Radius   int32        `xml:"radius,attr"`
	Monsters []xmlMonster `xml:"monster"`
}

type xmlMonster struct {
	Name      string `xml:"name,attr"`
	X         int16  `xml:"x,attr"`
	Y         int16  `xml:"y,attr"`
	SpawnTime int32  `xml:"spawntime,attr"`
	Direction uint8  `xml:"direction,attr"`
}

type xmlHouses struct {
	Houses []xmlHouse `xml:"house"`
}

type xmlHouse struct {
	ID     uint32 `xml:"houseid,attr"`
	Name   string `xml:"name,attr"`
	EntryX uint16 `xml:"entryx,attr"`
	EntryY uint16 `xml:"entryy,attr"`
	EntryZ uint8  `xml:"entryz,attr"`
	Rent   int32  `xml:"rent,attr"`
	TownID int32  `xml:"townid,attr"`
	Size   int32  `xml:"size,attr"`
}

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

	gm, err := otbm.ParseOTBM(*otbmPath)
	if err != nil {
		return fmt.Errorf("parse otbm: %w", err)
	}

	var spawns []xmlSpawn
	if *spawnsPath != "" {
		data, err := os.ReadFile(*spawnsPath)
		if err != nil {
			return fmt.Errorf("read spawns: %w", err)
		}
		var xs xmlSpawns
		if err := xml.Unmarshal(data, &xs); err != nil {
			return fmt.Errorf("parse spawns: %w", err)
		}
		spawns = xs.Spawns
	}

	var houses []xmlHouse
	if *housesPath != "" {
		data, err := os.ReadFile(*housesPath)
		if err != nil {
			return fmt.Errorf("read houses: %w", err)
		}
		var xh xmlHouses
		if err := xml.Unmarshal(data, &xh); err != nil {
			return fmt.Errorf("parse houses: %w", err)
		}
		houses = xh.Houses
	}

	db, err := sql.Open("mysql", *dsn)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return fmt.Errorf("ping db: %w", err)
	}

	tileCount, err := seedTiles(db, gm)
	if err != nil {
		return fmt.Errorf("seed tiles: %w", err)
	}

	spawnCount, err := seedSpawns(db, spawns)
	if err != nil {
		return fmt.Errorf("seed spawns: %w", err)
	}

	houseCount, err := seedHouses(db, houses)
	if err != nil {
		return fmt.Errorf("seed houses: %w", err)
	}

	townCount, err := seedTowns(db, gm.Towns)
	if err != nil {
		return fmt.Errorf("seed towns: %w", err)
	}

	wpCount, err := seedWaypoints(db, gm.Waypoints)
	if err != nil {
		return fmt.Errorf("seed waypoints: %w", err)
	}

	fmt.Printf("seeded: %d tiles, %d spawns, %d houses, %d towns, %d waypoints\n",
		tileCount, spawnCount, houseCount, townCount, wpCount)
	return nil
}

func seedTiles(db *sql.DB, gm *otbm.GameMap) (int, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM map_tiles"); err != nil {
		return 0, err
	}

	stmt, err := tx.Prepare("INSERT INTO map_tiles (x, y, z, ground_id, flags, house_id, items) VALUES (?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	count := 0
	for key, tile := range gm.Tiles {
		x := uint16(key >> 24)
		y := uint16((key >> 8) & 0xFFFF)
		z := uint8(key & 0xFF)

		var groundID uint16
		items := tile.Items
		if len(items) > 0 {
			groundID = items[0]
			items = items[1:]
		}

		var blob []byte
		if len(items) > 0 {
			blob = make([]byte, len(items)*2)
			for i, id := range items {
				binary.LittleEndian.PutUint16(blob[i*2:], id)
			}
		}

		if _, err := stmt.Exec(x, y, z, groundID, tile.Flags, tile.HouseID, blob); err != nil {
			return 0, fmt.Errorf("tile %d,%d,%d: %w", x, y, z, err)
		}
		count++
	}

	return count, tx.Commit()
}

func seedSpawns(db *sql.DB, spawns []xmlSpawn) (int, error) {
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

	entryStmt, err := tx.Prepare("INSERT INTO map_spawn_entries (spawn_id, name, offset_x, offset_y, spawntime) VALUES (?, ?, ?, ?, ?)")
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
		for _, m := range s.Monsters {
			if _, err := entryStmt.Exec(spawnID, m.Name, m.X, m.Y, m.SpawnTime); err != nil {
				return 0, err
			}
		}
	}

	return len(spawns), tx.Commit()
}

func seedHouses(db *sql.DB, houses []xmlHouse) (int, error) {
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

func seedTowns(db *sql.DB, towns []otbm.Town) (int, error) {
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
		if _, err := stmt.Exec(t.ID, t.Name, t.X, t.Y, t.Z); err != nil {
			return 0, err
		}
	}

	return len(towns), tx.Commit()
}

func seedWaypoints(db *sql.DB, waypoints []otbm.Waypoint) (int, error) {
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
