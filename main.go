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

	switch resource {
	case "items":
		switch action {
		case "pack":
			if err := items.Pack(args); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
		case "unpack":
			if err := items.Unpack(args); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
		default:
			fmt.Fprintf(os.Stderr, "unknown items action: %s\n", action)
			printUsage()
			os.Exit(1)
		}
	case "map":
		switch action {
		case "pack":
			if err := world.Pack(args); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
		case "unpack":
			if err := world.Unpack(args); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
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
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `Usage:
  otconv items pack   --otb FILE --xml FILE -o FILE
  otconv items unpack --oti FILE --otb FILE --xml FILE
  otconv map   pack   --otbm FILE [--spawns FILE] [--houses FILE] -o FILE
  otconv map   unpack --otw FILE --otbm FILE [--spawns FILE] [--houses FILE]`)
}
