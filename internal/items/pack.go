package items

import (
	"database/sql"
	"encoding/xml"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	_ "github.com/go-sql-driver/mysql"

	"github.com/codefatherllc/wypas-lib/otbm"
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

type itemRow struct {
	serverID          uint16
	clientID          uint16
	name, article     string
	plural            string
	description       string
	runeSpellName     string
	weight            float32
	armor             int
	defense           int
	extraDefense      int
	attack            int
	extraAttack       int
	attackSpeed       int
	rotateTo          int
	containerSize     int
	maxTextLength     int
	writeOnceItemID   int
	charges           int
	decayTo           int
	decayTime         int
	transformEquipTo  int
	transformDeEquipTo int
	transformUseTo    int
	duration          int
	showDuration      bool
	showCharges       bool
	showCount         bool
	showAttributes    bool
	breakChance       int
	hitChance         int
	maxHitChance      int
	dualWield         bool
	shootRange        int
	worth             int
	levelDoor         int
	specialDoor       bool
	closingDoor       bool
	wareID            int
	forceSerialize    bool
	weaponType        int
	ammoType          int
	ammoAction        int
	shootType         int
	magicEffect       int
	slotPosition      int
	wieldPosition     int
	fluidSource       int
	corpseType        int
	lightLevel        int
	lightColor        int
	blockSolid        bool
	blockProjectile   bool
	blockPathFind     bool
	allowDistRead     bool
	movable           bool
	pickupable        bool
	allowPickupable   bool
	isVertical        bool
	isHorizontal      bool
	walkStack         bool
	replaceable       bool
	canWriteText      bool
	canReadText       bool
	stopTime          bool
	floorchange       uint8
	bedPartnerDir     int
	maleTransformTo   int
	maleLooktype      int
	femaleTransformTo int
	femaleLooktype    int
	abilities         string // JSON
}

func Seed(args []string) error {
	fs := flag.NewFlagSet("items seed", flag.ExitOnError)
	otbPath := fs.String("otb", "", "path to items.otb")
	xmlPath := fs.String("xml", "", "path to items.xml")
	dsn := fs.String("dsn", "", "database DSN")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *otbPath == "" || *xmlPath == "" || *dsn == "" {
		return fmt.Errorf("--otb, --xml, and --dsn are required")
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

	db, err := sql.Open("mysql", *dsn)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return fmt.Errorf("ping db: %w", err)
	}

	xmlMap := buildXMLMap(xi.Items)
	rows := buildItemRows(otb, xmlMap)

	if err := insertItems(db, rows); err != nil {
		return fmt.Errorf("insert: %w", err)
	}

	fmt.Printf("seeded %d items into item_types\n", len(rows))
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

func buildItemRows(otb *otbm.OTB, xmlMap map[uint16]*xmlItem) []itemRow {
	var rows []itemRow
	for serverID, clientID := range otb.ServerToClient {
		r := itemRow{
			serverID: serverID,
			clientID: clientID,
			decayTo:  -1,
			movable:  true,
			walkStack: true,
		}
		if xi, ok := xmlMap[serverID]; ok {
			r.name = xi.Name
			r.article = xi.Article
			r.plural = xi.Plural
			applyAttrs(&r, xi.Attrs)
		}
		rows = append(rows, r)
	}
	return rows
}

func insertItems(db *sql.DB, rows []itemRow) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM item_types"); err != nil {
		return err
	}

	stmt, err := tx.Prepare(`INSERT INTO item_types (
		server_id, client_id, name, article, plural, description, rune_spell_name,
		weight, armor, defense, extra_defense, attack, extra_attack, attack_speed,
		rotate_to, container_size, max_text_length, write_once_item_id,
		charges, decay_to, decay_time, transform_equip_to, transform_deequip_to,
		transform_use_to, duration, show_duration, show_charges, show_count,
		show_attributes, break_chance, hit_chance, max_hit_chance, dual_wield,
		shoot_range, worth, level_door, special_door, closing_door, ware_id,
		force_serialize, weapon_type, ammo_type, ammo_action, shoot_type,
		magic_effect, slot_position, wield_position, fluid_source, corpse_type,
		light_level, light_color, block_solid, block_projectile, block_path_find,
		allow_dist_read, movable, pickupable, allow_pickupable, is_vertical,
		is_horizontal, walk_stack, replaceable, can_write_text, can_read_text,
		stop_time, floorchange, bed_partner_dir, male_transform_to, male_looktype,
		female_transform_to, female_looktype, abilities
	) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, r := range rows {
		var abilitiesVal interface{}
		if r.abilities != "" {
			abilitiesVal = r.abilities
		}
		_, err := stmt.Exec(
			r.serverID, r.clientID, r.name, r.article, r.plural, r.description, r.runeSpellName,
			r.weight, r.armor, r.defense, r.extraDefense, r.attack, r.extraAttack, r.attackSpeed,
			r.rotateTo, r.containerSize, r.maxTextLength, r.writeOnceItemID,
			r.charges, r.decayTo, r.decayTime, r.transformEquipTo, r.transformDeEquipTo,
			r.transformUseTo, r.duration, r.showDuration, r.showCharges, r.showCount,
			r.showAttributes, r.breakChance, r.hitChance, r.maxHitChance, r.dualWield,
			r.shootRange, r.worth, r.levelDoor, r.specialDoor, r.closingDoor, r.wareID,
			r.forceSerialize, r.weaponType, r.ammoType, r.ammoAction, r.shootType,
			r.magicEffect, r.slotPosition, r.wieldPosition, r.fluidSource, r.corpseType,
			r.lightLevel, r.lightColor, r.blockSolid, r.blockProjectile, r.blockPathFind,
			r.allowDistRead, r.movable, r.pickupable, r.allowPickupable, r.isVertical,
			r.isHorizontal, r.walkStack, r.replaceable, r.canWriteText, r.canReadText,
			r.stopTime, r.floorchange, r.bedPartnerDir, r.maleTransformTo, r.maleLooktype,
			r.femaleTransformTo, r.femaleLooktype, abilitiesVal,
		)
		if err != nil {
			return fmt.Errorf("item %d: %w", r.serverID, err)
		}
	}

	return tx.Commit()
}

func applyAttrs(r *itemRow, attrs []xmlAttr) {
	for _, a := range attrs {
		key := strings.ToLower(a.Key)
		val := a.Value
		switch key {
		case "weight":
			r.weight = float32(atoi(val)) / 100.0
		case "attack":
			r.attack = atoi(val)
		case "defense":
			r.defense = atoi(val)
		case "extradefense", "extradef":
			r.extraDefense = atoi(val)
		case "armor":
			r.armor = atoi(val)
		case "rotateto":
			r.rotateTo = atoi(val)
		case "containersize":
			r.containerSize = atoi(val)
		case "charges":
			r.charges = atoi(val)
		case "decayto":
			r.decayTo = atoi(val)
		case "duration":
			r.duration = atoi(val)
			r.decayTime = atoi(val)
		case "transformequipto", "onequipto":
			r.transformEquipTo = atoi(val)
		case "transformdeequipto", "ondeequipto":
			r.transformDeEquipTo = atoi(val)
		case "transformuseto", "transformto", "onuseto":
			r.transformUseTo = atoi(val)
		case "maxhitchance":
			r.maxHitChance = atoi(val)
		case "hitchance":
			r.hitChance = atoi(val)
		case "worth":
			r.worth = atoi(val)
		case "shootrange", "range":
			r.shootRange = atoi(val)
		case "breakchance":
			r.breakChance = atoi(val)
		case "leveldoor":
			r.levelDoor = atoi(val)
		case "wareid":
			r.wareID = atoi(val)
		case "maxtextlen", "maxtextlength":
			r.maxTextLength = atoi(val)
		case "writeonceitemid":
			r.writeOnceItemID = atoi(val)
		case "attackspeed":
			r.attackSpeed = atoi(val)
		case "extraattack", "extraatk":
			r.extraAttack = atoi(val)
		case "description":
			r.description = val
		case "runespellname":
			r.runeSpellName = val
		case "showduration":
			r.showDuration = val == "1" || val == "true"
		case "showcharges":
			r.showCharges = val == "1" || val == "true"
		case "showcount":
			r.showCount = val == "1" || val == "true"
		case "showattributes":
			r.showAttributes = val == "1" || val == "true"
		case "forceserialize", "forceserialization", "forcesave":
			r.forceSerialize = val == "1" || val == "true"
		case "dualwield":
			r.dualWield = val == "1" || val == "true"
		case "specialdoor":
			r.specialDoor = val == "1" || val == "true"
		case "closingdoor":
			r.closingDoor = val == "1" || val == "true"
		case "blocksolid", "blocking":
			r.blockSolid = val != "0"
		case "blockprojectile":
			r.blockProjectile = val != "0"
		case "blockpathfind", "blockpathing", "blockpath":
			r.blockPathFind = val != "0"
		case "allowdistread", "allowdistanceread":
			r.allowDistRead = val != "0"
		case "movable", "moveable":
			r.movable = val != "0"
		case "pickupable":
			r.pickupable = val != "0"
		case "allowpickupable":
			r.allowPickupable = val != "0"
		case "vertical", "isvertical":
			r.isVertical = val != "0"
		case "horizontal", "ishorizontal":
			r.isHorizontal = val != "0"
		case "walkstack":
			r.walkStack = val != "0"
		case "replacable", "replaceable":
			r.replaceable = val != "0"
		case "writeable", "writable":
			r.canWriteText = val != "0"
			r.canReadText = val != "0"
		case "readable":
			r.canReadText = val != "0"
		case "stopduration":
			r.stopTime = val != "0"
		case "lightlevel":
			r.lightLevel = atoi(val)
		case "lightcolor":
			r.lightColor = atoi(val)
		case "floorchange":
			r.floorchange |= floorchangeBit(strings.ToLower(val))
		case "weapontype":
			r.weaponType = weaponTypeVal(strings.ToLower(val))
		case "ammotype":
			r.ammoType = ammoTypeVal(strings.ToLower(val))
		case "shoottype":
			r.shootType = shootTypeVal(strings.ToLower(val))
		case "effect":
			r.magicEffect = magicEffectVal(strings.ToLower(val))
		case "slottype":
			r.slotPosition = slotTypeVal(strings.ToLower(val))
		case "corpsetype":
			r.corpseType = corpseTypeVal(strings.ToLower(val))
		case "fluidsource":
			r.fluidSource = fluidTypeVal(strings.ToLower(val))
		case "partnerdirection":
			r.bedPartnerDir = atoi(val)
		case "maletransformto":
			r.maleTransformTo = atoi(val)
		case "femaletransformto":
			r.femaleTransformTo = atoi(val)
		case "malelooktype":
			r.maleLooktype = atoi(val)
		case "femalelooktype":
			r.femaleLooktype = atoi(val)
		case "speed":
			r.abilities = jsonSet(r.abilities, "speed", atoi(val))
		case "invisible":
			r.abilities = jsonSet(r.abilities, "invisible", val != "0")
		case "healthgain":
			r.abilities = jsonSet(r.abilities, "healthGain", atoi(val))
		case "healthticks":
			r.abilities = jsonSet(r.abilities, "healthTicks", atoi(val))
		case "managain":
			r.abilities = jsonSet(r.abilities, "manaGain", atoi(val))
		case "manaticks":
			r.abilities = jsonSet(r.abilities, "manaTicks", atoi(val))
		case "manashield":
			r.abilities = jsonSet(r.abilities, "manaShield", val != "0")
		case "skillsword":
			r.abilities = jsonSet(r.abilities, "skillSword", atoi(val))
		case "skillaxe":
			r.abilities = jsonSet(r.abilities, "skillAxe", atoi(val))
		case "skillclub":
			r.abilities = jsonSet(r.abilities, "skillClub", atoi(val))
		case "skilldist":
			r.abilities = jsonSet(r.abilities, "skillDist", atoi(val))
		case "skillfish":
			r.abilities = jsonSet(r.abilities, "skillFish", atoi(val))
		case "skillshield":
			r.abilities = jsonSet(r.abilities, "skillShield", atoi(val))
		case "skillfist":
			r.abilities = jsonSet(r.abilities, "skillFist", atoi(val))
		case "maxhealthpoints", "maxhitpoints":
			r.abilities = jsonSet(r.abilities, "maxHealthPoints", atoi(val))
		case "maxmanapoints":
			r.abilities = jsonSet(r.abilities, "maxManaPoints", atoi(val))
		case "magiclevelpoints", "magicpoints":
			r.abilities = jsonSet(r.abilities, "magicPoints", atoi(val))
		case "absorbpercentall":
			v := atoi(val)
			r.abilities = jsonSet(r.abilities, "absorbAll", v)
		case "absorbpercentphysical":
			r.abilities = jsonSet(r.abilities, "absorbPhysical", atoi(val))
		case "absorbpercentenergy":
			r.abilities = jsonSet(r.abilities, "absorbEnergy", atoi(val))
		case "absorbpercentfire":
			r.abilities = jsonSet(r.abilities, "absorbFire", atoi(val))
		case "absorbpercentpoison", "absorbpercentearth":
			r.abilities = jsonSet(r.abilities, "absorbEarth", atoi(val))
		case "absorbpercentice":
			r.abilities = jsonSet(r.abilities, "absorbIce", atoi(val))
		case "absorbpercentholy":
			r.abilities = jsonSet(r.abilities, "absorbHoly", atoi(val))
		case "absorbpercentdeath":
			r.abilities = jsonSet(r.abilities, "absorbDeath", atoi(val))
		case "absorbpercentlifedrain":
			r.abilities = jsonSet(r.abilities, "absorbLifeDrain", atoi(val))
		case "absorbpercentmanadrain":
			r.abilities = jsonSet(r.abilities, "absorbManaDrain", atoi(val))
		case "absorbpercentdrown":
			r.abilities = jsonSet(r.abilities, "absorbDrown", atoi(val))
		case "suppressenergy", "suppressshock":
			r.abilities = jsonSet(r.abilities, "suppressEnergy", true)
		case "suppressfire", "suppressburn":
			r.abilities = jsonSet(r.abilities, "suppressFire", true)
		case "suppresspoison", "suppressearth":
			r.abilities = jsonSet(r.abilities, "suppressPoison", true)
		case "suppressice", "suppressfreeze":
			r.abilities = jsonSet(r.abilities, "suppressIce", true)
		case "suppressholy", "suppressdazzle":
			r.abilities = jsonSet(r.abilities, "suppressHoly", true)
		case "suppressdeath", "suppresscurse":
			r.abilities = jsonSet(r.abilities, "suppressDeath", true)
		case "suppressdrown":
			r.abilities = jsonSet(r.abilities, "suppressDrown", true)
		case "suppressphysical":
			r.abilities = jsonSet(r.abilities, "suppressPhysical", true)
		case "suppressdrunk":
			r.abilities = jsonSet(r.abilities, "suppressDrunk", true)
		case "preventloss":
			r.abilities = jsonSet(r.abilities, "preventLoss", val != "0")
		case "preventdrop":
			r.abilities = jsonSet(r.abilities, "preventDrop", val != "0")
		}
	}
}

func jsonSet(existing, key string, val interface{}) string {
	if existing == "" {
		existing = "{}"
	}
	// Simple key-value append without json package for performance
	prefix := existing[:len(existing)-1] // strip closing }
	if len(prefix) > 1 {
		prefix += ","
	}
	switch v := val.(type) {
	case int:
		return fmt.Sprintf(`%s"%s":%d}`, prefix, key, v)
	case bool:
		return fmt.Sprintf(`%s"%s":%t}`, prefix, key, v)
	default:
		return fmt.Sprintf(`%s"%s":%v}`, prefix, key, v)
	}
}

func floorchangeBit(val string) uint8 {
	switch val {
	case "down":
		return 1 << 0
	case "north":
		return 1 << 1
	case "south":
		return 1 << 2
	case "east":
		return 1 << 3
	case "west":
		return 1 << 4
	case "northex":
		return 1 << 5
	case "southex":
		return 1 << 6
	case "eastex":
		return 1 << 7
	}
	return 0
}

func weaponTypeVal(s string) int {
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

func ammoTypeVal(s string) int {
	m := map[string]int{
		"bolt": 1, "arrow": 2, "poisonarrow": 3, "burstarrow": 4,
		"throwingstar": 5, "throwingknife": 6, "smallstone": 7,
		"largerock": 8, "snowball": 9, "powerbolt": 10, "spear": 11,
	}
	return m[s]
}

func shootTypeVal(s string) int {
	m := map[string]int{
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

func magicEffectVal(s string) int {
	m := map[string]int{
		"redspark": 1, "bluebubble": 2, "poff": 3, "yellowspark": 4,
		"explosionarea": 5, "explosiondamage": 6, "firearea": 7,
		"yellowrings": 8, "greenrings": 9, "hitarea": 10,
		"teleport": 11, "energydamage": 12, "energyarea": 13,
		"blackspark": 18, "blueshimmer": 27, "redshimmer": 28,
		"greenspark": 29, "mortarea": 30,
	}
	return m[s]
}

func corpseTypeVal(s string) int {
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

func fluidTypeVal(s string) int {
	m := map[string]int{
		"water": 1, "blood": 2, "beer": 3, "slime": 4,
		"lemonade": 5, "milk": 6, "mana": 7, "life": 8,
		"oil": 9, "urine": 10, "coconutmilk": 11, "wine": 12,
		"mud": 13, "fruitjuice": 14, "lava": 15, "rum": 16,
		"swamp": 17, "tea": 18, "mead": 19,
	}
	return m[s]
}

func slotTypeVal(s string) int {
	m := map[string]int{
		"head": 1, "body": 2, "legs": 4, "feet": 8,
		"backpack": 16, "two-handed": 32, "right-hand": 64,
		"left-hand": 128, "necklace": 256, "ring": 512, "ammo": 1024, "hand": 2048,
	}
	return m[s]
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
