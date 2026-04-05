package main

import (
	"fmt"
	"os"

	"github.com/codefatherllc/wypas-otconv/internal/items"
	"github.com/codefatherllc/wypas-otconv/internal/world"
)

func main() {
	if len(os.Args) < 3 {
		printUsage()
		os.Exit(1)
	}

	resource := os.Args[1]
	action := os.Args[2]
	args := os.Args[3:]

	var err error
	switch resource {
	case "items":
		switch action {
		case "seed":
			err = items.Seed(args)
		default:
			fmt.Fprintf(os.Stderr, "unknown items action: %s\n", action)
			printUsage()
			os.Exit(1)
		}
	case "map":
		switch action {
		case "seed":
			err = world.Seed(args)
		default:
			fmt.Fprintf(os.Stderr, "unknown map action: %s\n", action)
			printUsage()
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown resource: %s\n", resource)
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `Usage:
  otconv items seed --otb FILE --xml FILE --dsn DSN
  otconv map   seed --otbm FILE [--spawns FILE] [--houses FILE] --dsn DSN

Tables (v2 schema):
  items seed  → item_types
  map seed    → map, items, spawns, spawn, houses, towns, waypoints`)
}
