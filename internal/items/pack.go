package items

import (
	"encoding/xml"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/codefatherllc/wypas-lib/otbm"
	"github.com/codefatherllc/wypas-lib/oti"
)

type xmlItems struct {
	Items []xmlItem `xml:"item"`
}

type xmlItem struct {
	ID      uint16    `xml:"id,attr"`
	FromID  uint16    `xml:"fromid,attr"`
	ToID    uint16    `xml:"toid,attr"`
	Name    string    `xml:"name,attr"`
	Article string    `xml:"article,attr"`
	Plural  string    `xml:"plural,attr"`
	Attrs   []xmlAttr `xml:"attribute"`
}

type xmlAttr struct {
	Key   string `xml:"key,attr"`
	Value string `xml:"value,attr"`
}

func Pack(args []string) error {
	fs := flag.NewFlagSet("items pack", flag.ExitOnError)
	otbPath := fs.String("otb", "", "path to items.otb")
	xmlPath := fs.String("xml", "", "path to items.xml")
	outPath := fs.String("o", "", "output .oti file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *otbPath == "" || *xmlPath == "" || *outPath == "" {
		return fmt.Errorf("--otb, --xml, and -o are required")
	}

	otb, err := otbm.ParseOTB(*otbPath)
	if err != nil {
		return fmt.Errorf("parse otb: %w", err)
	}

	xmlData, err := os.ReadFile(*xmlPath)
	if err != nil {
		return fmt.Errorf("read xml: %w", err)
	}
	var xi xmlItems
	if err := xml.Unmarshal(xmlData, &xi); err != nil {
		return fmt.Errorf("parse xml: %w", err)
	}

	xmlMap := buildXMLMap(xi.Items)
	db := buildItemDatabase(otb, xmlMap)

	if err := oti.WriteFile(*outPath, db); err != nil {
		return fmt.Errorf("write oti: %w", err)
	}

	fi, _ := os.Stat(*outPath)
	fmt.Printf("packed %d items → %s (%d bytes)\n", len(db.Items), *outPath, fi.Size())
	return nil
}

func buildXMLMap(xmlItems []xmlItem) map[uint16]*xmlItem {
	m := make(map[uint16]*xmlItem, len(xmlItems))
	for i := range xmlItems {
		item := &xmlItems[i]
		if item.ID != 0 {
			m[item.ID] = item
		} else if item.FromID != 0 {
			for id := item.FromID; id <= item.ToID; id++ {
				clone := *item
				clone.ID = id
				m[id] = &clone
			}
		}
	}
	return m
}

func buildItemDatabase(otb *otbm.OTB, xmlMap map[uint16]*xmlItem) *oti.ItemDatabase {
	db := &oti.ItemDatabase{Version: 1}

	for serverID, clientID := range otb.ServerToClient {
		it := oti.ItemType{
			ServerID: serverID,
			ClientID: clientID,
		}

		if xi, ok := xmlMap[serverID]; ok {
			it.Name = xi.Name
			it.Article = xi.Article
			it.Plural = xi.Plural
			applyXMLAttrs(&it, xi.Attrs)
		}

		db.Items = append(db.Items, it)
	}

	return db
}

func applyXMLAttrs(it *oti.ItemType, attrs []xmlAttr) {
	for _, a := range attrs {
		key := strings.ToLower(a.Key)
		val := a.Value
		switch key {
		case "weight":
			it.Weight = float32(atoi(val)) / 100.0
		case "attack":
			it.Attack = int32(atoi(val))
		case "defense":
			it.Defense = int32(atoi(val))
		case "extradefense", "extradef":
			it.ExtraDefense = int32(atoi(val))
		case "armor":
			it.Armor = int32(atoi(val))
		case "rotateto":
			it.RotateTo = int32(atoi(val))
		case "containersize":
			it.ContainerSize = uint16(atoi(val))
		case "charges":
			it.Charges = uint32(atoi(val))
		case "decayto":
			it.DecayTo = int32(atoi(val))
		case "duration":
			it.Duration = uint32(atoi(val))
			it.DecayTime = uint32(atoi(val))
		case "transformequipto", "onequipto":
			it.TransformEquipTo = uint16(atoi(val))
		case "transformdeequipto", "ondeequipto":
			it.TransformDeEquipTo = uint16(atoi(val))
		case "transformuseto", "transformto", "onuseto":
			it.TransformUseTo = uint16(atoi(val))
		case "maxhitchance":
			it.MaxHitChance = int32(atoi(val))
		case "hitchance":
			it.HitChance = int32(atoi(val))
		case "worth":
			it.Worth = uint32(atoi(val))
		case "speed":
			if it.Abilities == nil {
				it.Abilities = &oti.Abilities{}
			}
			it.Abilities.Speed = int32(atoi(val))
		case "shootrange", "range":
			it.ShootRange = uint32(atoi(val))
		case "breakchance":
			it.BreakChance = int32(atoi(val))
		case "leveldoor":
			it.LevelDoor = uint32(atoi(val))
		case "wareid":
			it.WareID = uint16(atoi(val))
		case "maxtextlen", "maxtextlength":
			it.MaxTextLength = uint16(atoi(val))
		case "writeonceitemid":
			it.WriteOnceItemID = uint16(atoi(val))
		case "attackspeed":
			it.AttackSpeed = uint32(atoi(val))
		case "extraattack", "extraatk":
			it.ExtraAttack = int32(atoi(val))
		case "description":
			it.Description = val
		case "runespellname":
			it.RuneSpellName = val
		case "showduration":
			it.ShowDuration = val == "1" || val == "true"
		case "showcharges":
			it.ShowCharges = val == "1" || val == "true"
		case "showcount":
			it.ShowCount = val == "1" || val == "true"
		case "showattributes":
			it.ShowAttributes = val == "1" || val == "true"
		case "forceserialize", "forceserialization", "forcesave":
			it.ForceSerialize = val == "1" || val == "true"
		case "dualwield":
			it.DualWield = val == "1" || val == "true"
		case "specialdoor":
			it.SpecialDoor = val == "1" || val == "true"
		case "closingdoor":
			it.ClosingDoor = val == "1" || val == "true"
		case "blocksolid", "blocking":
			it.BlockSolid = val != "0"
		case "blockprojectile":
			it.BlockProjectile = val != "0"
		case "blockpathfind", "blockpathing", "blockpath":
			it.BlockPathFind = val != "0"
		case "allowdistread", "allowdistanceread":
			it.AllowDistRead = val != "0"
		case "movable", "moveable":
			it.Movable = val != "0"
		case "pickupable":
			it.Pickupable = val != "0"
		case "allowpickupable":
			it.AllowPickupable = val != "0"
		case "vertical", "isvertical":
			it.IsVertical = val != "0"
		case "horizontal", "ishorizontal":
			it.IsHorizontal = val != "0"
		case "walkstack":
			it.WalkStack = val != "0"
		case "replacable", "replaceable":
			it.Replaceable = val != "0"
		case "writeable", "writable":
			it.CanWriteText = val != "0"
			it.CanReadText = val != "0"
		case "readable":
			it.CanReadText = val != "0"
		case "stopduration":
			it.StopTime = val != "0"
		case "cache":
			it.Cache = val != "0"
		case "lightlevel":
			it.LightLevel = int32(atoi(val))
		case "lightcolor":
			it.LightColor = int32(atoi(val))
		case "floorchange":
			applyFloorchange(it, strings.ToLower(val))
		case "weapontype":
			it.WeaponType = weaponTypeVal(strings.ToLower(val))
		case "ammotype":
			it.AmmoType = ammoTypeVal(strings.ToLower(val))
		case "shoottype":
			it.ShootType = shootTypeVal(strings.ToLower(val))
		case "effect":
			it.MagicEffect = magicEffectVal(strings.ToLower(val))
		case "slottype":
			applySlotType(it, strings.ToLower(val))
		case "corpsetype":
			it.CorpseType = corpseTypeVal(strings.ToLower(val))
		case "fluidsource":
			it.FluidSource = fluidTypeVal(strings.ToLower(val))
		case "partnerdirection":
			it.BedPartnerDir = uint8(atoi(val))
		case "maletransformto":
			it.MaleTransformTo = uint16(atoi(val))
		case "femaletransformto":
			it.FemaleTransformTo = uint16(atoi(val))
		case "malelooktype":
			it.MaleLooktype = uint16(atoi(val))
		case "femalelooktype":
			it.FemaleLooktype = uint16(atoi(val))
		case "invisible":
			if it.Abilities == nil {
				it.Abilities = &oti.Abilities{}
			}
			it.Abilities.Invisible = val != "0"
		case "healthgain":
			ensureAbilities(it).HealthGain = int32(atoi(val))
			ensureAbilities(it).Regeneration = true
		case "healthticks":
			ensureAbilities(it).HealthTicks = int32(atoi(val))
			ensureAbilities(it).Regeneration = true
		case "managain":
			ensureAbilities(it).ManaGain = int32(atoi(val))
			ensureAbilities(it).Regeneration = true
		case "manaticks":
			ensureAbilities(it).ManaTicks = int32(atoi(val))
			ensureAbilities(it).Regeneration = true
		case "manashield":
			ensureAbilities(it).ManaShield = val != "0"
		case "skillsword":
			ensureAbilities(it).SkillSword = int32(atoi(val))
		case "skillaxe":
			ensureAbilities(it).SkillAxe = int32(atoi(val))
		case "skillclub":
			ensureAbilities(it).SkillClub = int32(atoi(val))
		case "skilldist":
			ensureAbilities(it).SkillDist = int32(atoi(val))
		case "skillfish":
			ensureAbilities(it).SkillFish = int32(atoi(val))
		case "skillshield":
			ensureAbilities(it).SkillShield = int32(atoi(val))
		case "skillfist":
			ensureAbilities(it).SkillFist = int32(atoi(val))
		case "maxhealthpoints", "maxhitpoints":
			ensureAbilities(it).MaxHealthPoints = int32(atoi(val))
		case "maxmanapoints":
			ensureAbilities(it).MaxManaPoints = int32(atoi(val))
		case "magiclevelpoints", "magicpoints":
			ensureAbilities(it).MagicPoints = int32(atoi(val))
		case "absorbpercentall":
			a := ensureAbilities(it)
			v := int32(atoi(val))
			a.Absorb = oti.CombatValues{Physical: v, Energy: v, Earth: v, Fire: v, Ice: v, Holy: v, Death: v, LifeDrain: v, ManaDrain: v, Drown: v}
		case "absorbpercentphysical":
			ensureAbilities(it).Absorb.Physical = int32(atoi(val))
		case "absorbpercentenergy":
			ensureAbilities(it).Absorb.Energy = int32(atoi(val))
		case "absorbpercentfire":
			ensureAbilities(it).Absorb.Fire = int32(atoi(val))
		case "absorbpercentpoison", "absorbpercentearth":
			ensureAbilities(it).Absorb.Earth = int32(atoi(val))
		case "absorbpercentice":
			ensureAbilities(it).Absorb.Ice = int32(atoi(val))
		case "absorbpercentholy":
			ensureAbilities(it).Absorb.Holy = int32(atoi(val))
		case "absorbpercentdeath":
			ensureAbilities(it).Absorb.Death = int32(atoi(val))
		case "absorbpercentlifedrain":
			ensureAbilities(it).Absorb.LifeDrain = int32(atoi(val))
		case "absorbpercentmanadrain":
			ensureAbilities(it).Absorb.ManaDrain = int32(atoi(val))
		case "absorbpercentdrown":
			ensureAbilities(it).Absorb.Drown = int32(atoi(val))
		case "suppressenergy", "suppressshock":
			ensureAbilities(it).SuppressEnergy = true
		case "suppressfire", "suppressburn":
			ensureAbilities(it).SuppressFire = true
		case "suppresspoison", "suppressearth":
			ensureAbilities(it).SuppressPoison = true
		case "suppressice", "suppressfreeze":
			ensureAbilities(it).SuppressIce = true
		case "suppressholy", "suppressdazzle":
			ensureAbilities(it).SuppressHoly = true
		case "suppressdeath", "suppresscurse":
			ensureAbilities(it).SuppressDeath = true
		case "suppressdrown":
			ensureAbilities(it).SuppressDrown = true
		case "suppressphysical":
			ensureAbilities(it).SuppressPhysical = true
		case "suppressdrunk":
			ensureAbilities(it).SuppressDrunk = true
		case "preventloss":
			ensureAbilities(it).PreventLoss = val != "0"
		case "preventdrop":
			ensureAbilities(it).PreventDrop = val != "0"
		}
	}
}

func ensureAbilities(it *oti.ItemType) *oti.Abilities {
	if it.Abilities == nil {
		it.Abilities = &oti.Abilities{}
	}
	return it.Abilities
}

func applyFloorchange(it *oti.ItemType, val string) {
	switch val {
	case "down":
		it.FloorchangeDown = true
	case "north":
		it.FloorchangeNorth = true
	case "south":
		it.FloorchangeSouth = true
	case "east":
		it.FloorchangeEast = true
	case "west":
		it.FloorchangeWest = true
	case "northex":
		it.FloorchangeNorthEx = true
	case "southex":
		it.FloorchangeSouthEx = true
	case "eastex":
		it.FloorchangeEastEx = true
	case "westex":
		it.FloorchangeWestEx = true
	}
}

func applySlotType(it *oti.ItemType, val string) {
	switch val {
	case "head":
		it.SlotPosition |= 1 << 0
	case "body":
		it.SlotPosition |= 1 << 1
	case "legs":
		it.SlotPosition |= 1 << 2
	case "feet":
		it.SlotPosition |= 1 << 3
	case "backpack":
		it.SlotPosition |= 1 << 4
	case "two-handed":
		it.SlotPosition |= 1 << 5
	case "necklace":
		it.SlotPosition |= 1 << 6
	case "ring":
		it.SlotPosition |= 1 << 7
	case "ammo":
		it.SlotPosition |= 1 << 8
	}
}

func weaponTypeVal(s string) uint8 {
	switch s {
	case "sword":
		return 1
	case "club":
		return 2
	case "axe":
		return 3
	case "shield":
		return 4
	case "distance", "dist":
		return 5
	case "wand", "rod":
		return 6
	case "ammunition", "ammo":
		return 7
	case "fist":
		return 8
	}
	return 0
}

func ammoTypeVal(s string) uint8 {
	m := map[string]uint8{
		"bolt": 1, "arrow": 2, "poisonarrow": 3, "burstarrow": 4,
		"throwingstar": 5, "throwingknife": 6, "smallstone": 7,
		"largerock": 8, "snowball": 9, "powerbolt": 10, "spear": 11,
	}
	return m[s]
}

func shootTypeVal(s string) uint8 {
	m := map[string]uint8{
		"spear": 1, "bolt": 2, "arrow": 3, "fire": 4, "energy": 5,
		"poisonarrow": 6, "burstarrow": 7, "throwingstar": 8,
		"throwingknife": 9, "smallstone": 10, "death": 11,
		"largerock": 12, "snowball": 13, "powerbolt": 14,
		"poison": 15, "infernalbolt": 16, "huntingspear": 17,
		"enchantedspear": 18, "assassinstar": 19, "piercingbolt": 20,
		"earth": 21, "ice": 22, "flamearrow": 23, "holy": 24,
		"etherealspear": 25, "flamingarrow": 26, "shiverarrow": 27,
		"eartharrow": 28, "explosion": 29, "cake": 30,
	}
	return m[s]
}

func magicEffectVal(s string) uint8 {
	m := map[string]uint8{
		"redspark": 1, "bluebubble": 2, "poff": 3, "yellowspark": 4,
		"explosionarea": 5, "explosiondamage": 6, "firearea": 7,
		"yellowrings": 8, "greenrings": 9, "hitarea": 10,
		"teleport": 11, "energydamage": 12, "energyarea": 13,
		"blackspark": 18, "blueshimmer": 27, "redshimmer": 28,
		"greenspark": 29, "mortarea": 30,
	}
	return m[s]
}

func corpseTypeVal(s string) uint8 {
	switch s {
	case "venom":
		return 1
	case "blood":
		return 2
	case "undead":
		return 3
	case "fire":
		return 4
	case "energy":
		return 5
	}
	return 0
}

func fluidTypeVal(s string) uint8 {
	m := map[string]uint8{
		"water": 1, "blood": 2, "beer": 3, "slime": 4,
		"lemonade": 5, "milk": 6, "mana": 7, "life": 8,
		"oil": 9, "urine": 10, "coconutmilk": 11, "wine": 12,
		"mud": 13, "fruitjuice": 14, "lava": 15, "rum": 16,
		"swamp": 17, "tea": 18, "mead": 19,
	}
	return m[s]
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
