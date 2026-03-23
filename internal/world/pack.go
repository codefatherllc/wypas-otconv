package world

import (
	"encoding/xml"
	"flag"
	"fmt"
	"os"

	"github.com/codefatherllc/wypas-lib/otbm"
	libotw "github.com/codefatherllc/wypas-lib/otw"
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

func Pack(args []string) error {
	fs := flag.NewFlagSet("map pack", flag.ExitOnError)
	otbmPath := fs.String("otbm", "", "path to Map.otbm")
	spawnsPath := fs.String("spawns", "", "path to spawns.xml (optional)")
	housesPath := fs.String("houses", "", "path to houses.xml (optional)")
	outPath := fs.String("o", "", "output .otw file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *otbmPath == "" || *outPath == "" {
		return fmt.Errorf("--otbm and -o are required")
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

	wm := buildWorldMap(gm, spawns, houses)

	if err := libotw.WriteFile(*outPath, wm); err != nil {
		return fmt.Errorf("write otw: %w", err)
	}

	totalTiles := 0
	for _, ta := range wm.TileAreas {
		totalTiles += len(ta.Tiles)
	}
	fi, _ := os.Stat(*outPath)
	fmt.Printf("packed %d tiles, %d spawns, %d houses → %s (%d bytes)\n",
		totalTiles, len(wm.Spawns), len(wm.Houses), *outPath, fi.Size())
	return nil
}

type areaKey struct {
	x, y uint16
	z    uint8
}

func buildWorldMap(gm *otbm.GameMap, spawns []xmlSpawn, houses []xmlHouse) *libotw.WorldMap {
	wm := &libotw.WorldMap{
		Version: 1,
		Width:   gm.MaxX - gm.MinX,
		Height:  gm.MaxY - gm.MinY,
	}

	areas := make(map[areaKey]*libotw.TileArea)

	for key, tile := range gm.Tiles {
		x := uint16(key >> 24)
		y := uint16((key >> 8) & 0xFFFF)
		z := uint8(key & 0xFF)

		ak := areaKey{x: x & 0xFF00, y: y & 0xFF00, z: z}
		area, ok := areas[ak]
		if !ok {
			area = &libotw.TileArea{BaseX: ak.x, BaseY: ak.y, BaseZ: z}
			areas[ak] = area
		}

		t := libotw.Tile{
			OffsetX: uint8(x & 0xFF),
			OffsetY: uint8(y & 0xFF),
			Flags:   tile.Flags,
			HouseID: tile.HouseID,
		}

		for _, itemID := range tile.Items {
			t.Items = append(t.Items, libotw.MapItem{ServerID: itemID})
		}
		for _, ri := range tile.RichItems {
			mi := libotw.MapItem{
				ServerID:    ri.ID,
				ActionID:    ri.ActionID,
				UniqueID:    ri.UniqueID,
				DoorID:      ri.DoorID,
				DepotID:     ri.DepotID,
				Text:        ri.Text,
				Description: ri.Description,
				Charges:     ri.Charges,
				RuneCharges: ri.RuneCharges,
				Count:       ri.Count,
			}
			if ri.TeleDest != nil {
				mi.TeleDestX = ri.TeleDest.X
				mi.TeleDestY = ri.TeleDest.Y
				mi.TeleDestZ = ri.TeleDest.Z
			}
			t.Items = append(t.Items, mi)
		}

		area.Tiles = append(area.Tiles, t)
	}

	for _, area := range areas {
		wm.TileAreas = append(wm.TileAreas, *area)
	}

	for _, t := range gm.Towns {
		wm.Towns = append(wm.Towns, libotw.Town{
			ID:      t.ID,
			Name:    t.Name,
			TempleX: t.X,
			TempleY: t.Y,
			TempleZ: t.Z,
		})
	}

	for _, wp := range gm.Waypoints {
		wm.Waypoints = append(wm.Waypoints, libotw.Waypoint{
			Name: wp.Name,
			X:    wp.X,
			Y:    wp.Y,
			Z:    wp.Z,
		})
	}

	for _, s := range spawns {
		sp := libotw.Spawn{
			CenterX: s.CenterX,
			CenterY: s.CenterY,
			CenterZ: s.CenterZ,
			Radius:  s.Radius,
		}
		for _, m := range s.Monsters {
			sp.Creatures = append(sp.Creatures, libotw.SpawnCreature{
				Name:      m.Name,
				OffsetX:   m.X,
				OffsetY:   m.Y,
				SpawnTime: m.SpawnTime,
				Direction: m.Direction,
			})
		}
		wm.Spawns = append(wm.Spawns, sp)
	}

	for _, h := range houses {
		wm.Houses = append(wm.Houses, libotw.House{
			ID:     h.ID,
			Name:   h.Name,
			EntryX: h.EntryX,
			EntryY: h.EntryY,
			EntryZ: h.EntryZ,
			Size:   h.Size,
			Rent:   h.Rent,
			TownID: h.TownID,
		})
	}

	return wm
}
