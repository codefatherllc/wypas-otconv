package items

import (
	"database/sql"
	"flag"
	"fmt"

	_ "github.com/go-sql-driver/mysql"

	"github.com/codefatherllc/wypas-lib/gamedata"
	"github.com/codefatherllc/wypas-lib/otb"
)

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

	items, err := otb.LoadItems(*otbPath, *xmlPath)
	if err != nil {
		return fmt.Errorf("load items: %w", err)
	}

	db, err := sql.Open("mysql", *dsn)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return fmt.Errorf("ping db: %w", err)
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM item_types"); err != nil {
		return fmt.Errorf("truncate: %w", err)
	}

	if err := gamedata.BulkInsertItems(tx, items); err != nil {
		return fmt.Errorf("insert: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	fmt.Printf("seeded %d items into item_types\n", len(items))
	return nil
}
