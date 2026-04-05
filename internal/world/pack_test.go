package world

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	_ "github.com/go-sql-driver/mysql"

	"github.com/codefatherllc/wypas-lib/gamedata"
)

func testDSN() string {
	dsn := os.Getenv("TEST_DSN")
	if dsn == "" {
		dsn = "root:test@tcp(127.0.0.1:13306)/otconv_test"
	}
	return dsn
}

func TestSeedTiles(t *testing.T) {
	db, err := sql.Open("mysql", testDSN())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		t.Skipf("db not available: %v", err)
	}

	// Build synthetic tiles with rich items
	tiles := []gamedata.MapTile{
		{
			X: 100, Y: 200, Z: 7,
			GroundID: 4526, Flags: 0, HouseID: 0,
			Items: []uint16{2400, 2401}, // legacy blob items
			RichItems: []gamedata.RichItem{
				{ServerID: 2400, Count: 1},
				{ServerID: 2401, Count: 1, ActionID: 100},
			},
		},
		{
			X: 101, Y: 200, Z: 7,
			GroundID: 4526, Flags: 1, HouseID: 0,
			Items: []uint16{1387},
			RichItems: []gamedata.RichItem{
				{
					ServerID: 1387, Count: 1,
					TeleDestX: 500, TeleDestY: 600, TeleDestZ: 7,
				},
			},
		},
		{
			X: 102, Y: 200, Z: 7,
			GroundID: 4526, Flags: 0, HouseID: 42,
			Items: []uint16{2590},
			RichItems: []gamedata.RichItem{
				{
					ServerID: 2590, Count: 1,
					DoorID:   3,
					Text:     "Welcome home",
				},
			},
		},
		{
			X: 103, Y: 200, Z: 7,
			GroundID: 4526, Flags: 0, HouseID: 0,
			Items: []uint16{1988},
			RichItems: []gamedata.RichItem{
				{
					ServerID: 1988, Count: 1,
					SubItems: []gamedata.RichItem{
						{ServerID: 2160, Count: 50}, // gold
						{ServerID: 2152, Count: 3, Charges: 100}, // rune
					},
				},
			},
		},
	}

	tileCount, itemCount, err := seedTiles(db, tiles)
	if err != nil {
		t.Fatalf("seedTiles: %v", err)
	}

	if tileCount != 4 {
		t.Errorf("expected 4 tiles, got %d", tileCount)
	}
	// 5 top-level items (2+1+1+1) + 2 sub-items = 7
	if itemCount != 7 {
		t.Errorf("expected 7 items, got %d", itemCount)
	}

	// Verify map table
	var mapCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM `map`").Scan(&mapCount); err != nil {
		t.Fatalf("count map: %v", err)
	}
	if mapCount != 4 {
		t.Errorf("expected 4 map rows, got %d", mapCount)
	}

	// Verify items table
	var totalItems int
	if err := db.QueryRow("SELECT COUNT(*) FROM `items` WHERE owner_type = 'map'").Scan(&totalItems); err != nil {
		t.Fatalf("count map items: %v", err)
	}
	if totalItems != 5 {
		t.Errorf("expected 5 map-owned items, got %d", totalItems)
	}

	var containerItems int
	if err := db.QueryRow("SELECT COUNT(*) FROM `items` WHERE owner_type = 'container'").Scan(&containerItems); err != nil {
		t.Fatalf("count container items: %v", err)
	}
	if containerItems != 2 {
		t.Errorf("expected 2 container items, got %d", containerItems)
	}

	// Verify teleport attributes
	teleOwnerID := packPos(101, 200, 7)
	var attrs sql.NullString
	if err := db.QueryRow("SELECT attributes FROM `items` WHERE owner_type = 'map' AND owner_id = ?", teleOwnerID).Scan(&attrs); err != nil {
		t.Fatalf("query tele item: %v", err)
	}
	if !attrs.Valid {
		t.Fatal("expected attributes for teleport item")
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(attrs.String), &m); err != nil {
		t.Fatalf("parse attributes JSON: %v", err)
	}
	teleDest, ok := m["tele_dest"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected tele_dest in attributes, got %v", m)
	}
	if teleDest["x"] != float64(500) || teleDest["y"] != float64(600) || teleDest["z"] != float64(7) {
		t.Errorf("wrong tele_dest: %v", teleDest)
	}

	// Verify door item with text attribute
	doorOwnerID := packPos(102, 200, 7)
	if err := db.QueryRow("SELECT attributes FROM `items` WHERE owner_type = 'map' AND owner_id = ?", doorOwnerID).Scan(&attrs); err != nil {
		t.Fatalf("query door item: %v", err)
	}
	if err := json.Unmarshal([]byte(attrs.String), &m); err != nil {
		t.Fatalf("parse door attrs: %v", err)
	}
	if m["door_id"] != float64(3) {
		t.Errorf("wrong door_id: %v", m["door_id"])
	}
	if m["text"] != "Welcome home" {
		t.Errorf("wrong text: %v", m["text"])
	}

	fmt.Printf("PASS: %d tiles, %d items (5 map + 2 container)\n", tileCount, itemCount)
}

func TestSeedSpawns(t *testing.T) {
	db, err := sql.Open("mysql", testDSN())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		t.Skipf("db not available: %v", err)
	}

	spawns := []gamedata.Spawn{
		{
			CenterX: 958, CenterY: 703, CenterZ: 0, Radius: 1,
			Creatures: []gamedata.SpawnCreature{
				{Name: "Anworb", Type: "npc", OffsetX: 0, OffsetY: -1, SpawnTime: 60, Direction: 2},
			},
		},
		{
			CenterX: 637, CenterY: 258, CenterZ: 1, Radius: 3,
			Creatures: []gamedata.SpawnCreature{
				{Name: "Orc Leader", Type: "monster", OffsetX: -3, OffsetY: -2, SpawnTime: 60},
				{Name: "Orc Warrior", Type: "monster", OffsetX: 1, OffsetY: -1, SpawnTime: 60},
			},
		},
	}

	count, err := seedSpawns(db, spawns)
	if err != nil {
		t.Fatalf("seedSpawns: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 spawns, got %d", count)
	}

	var spawnCount int
	db.QueryRow("SELECT COUNT(*) FROM `spawns`").Scan(&spawnCount)
	if spawnCount != 2 {
		t.Errorf("expected 2 spawns rows, got %d", spawnCount)
	}

	var entryCount int
	db.QueryRow("SELECT COUNT(*) FROM `spawn`").Scan(&entryCount)
	if entryCount != 3 {
		t.Errorf("expected 3 spawn entries, got %d", entryCount)
	}

	// Verify NPC type
	var spawnType string
	db.QueryRow("SELECT type FROM `spawn` WHERE name = 'Anworb'").Scan(&spawnType)
	if spawnType != "npc" {
		t.Errorf("expected type 'npc', got %q", spawnType)
	}

	fmt.Printf("PASS: %d spawns, %d entries\n", spawnCount, entryCount)
}

func TestSeedHouses(t *testing.T) {
	db, err := sql.Open("mysql", testDSN())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		t.Skipf("db not available: %v", err)
	}

	houses := []gamedata.House{
		{ID: 1, Name: "Test House", EntryX: 100, EntryY: 200, EntryZ: 7, Rent: 5000, TownID: 1, Size: 20, Guildhall: false},
		{ID: 2, Name: "Guild Hall", EntryX: 150, EntryY: 250, EntryZ: 7, Rent: 50000, TownID: 1, Size: 100, Guildhall: true},
	}

	count, err := seedHouses(db, houses)
	if err != nil {
		t.Fatalf("seedHouses: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 houses, got %d", count)
	}

	var houseCount int
	db.QueryRow("SELECT COUNT(*) FROM `houses`").Scan(&houseCount)
	if houseCount != 2 {
		t.Errorf("expected 2 houses, got %d", houseCount)
	}

	var guildhall bool
	db.QueryRow("SELECT guildhall FROM `houses` WHERE id = 2").Scan(&guildhall)
	if !guildhall {
		t.Error("expected guildhall=true for house 2")
	}

	fmt.Printf("PASS: %d houses\n", houseCount)
}

func TestSeedTowns(t *testing.T) {
	db, err := sql.Open("mysql", testDSN())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		t.Skipf("db not available: %v", err)
	}

	towns := []gamedata.Town{
		{ID: 1, Name: "Thais", EntryX: 32369, EntryY: 32241, EntryZ: 7},
		{ID: 2, Name: "Carlin", EntryX: 32360, EntryY: 31782, EntryZ: 7},
	}

	count, err := seedTowns(db, towns)
	if err != nil {
		t.Fatalf("seedTowns: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 towns, got %d", count)
	}

	fmt.Printf("PASS: %d towns\n", count)
}

func TestSeedWaypoints(t *testing.T) {
	db, err := sql.Open("mysql", testDSN())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		t.Skipf("db not available: %v", err)
	}

	waypoints := []gamedata.Waypoint{
		{Name: "thais-temple", X: 32369, Y: 32241, Z: 7},
		{Name: "carlin-depot", X: 32360, Y: 31782, Z: 7},
	}

	count, err := seedWaypoints(db, waypoints)
	if err != nil {
		t.Fatalf("seedWaypoints: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 waypoints, got %d", count)
	}

	fmt.Printf("PASS: %d waypoints\n", count)
}
