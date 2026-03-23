package world

import (
	"flag"
	"fmt"
)

func Unpack(args []string) error {
	fs := flag.NewFlagSet("map unpack", flag.ExitOnError)
	fs.String("otw", "", "path to .otw file")
	fs.String("otbm", "", "output Map.otbm path")
	fs.String("spawns", "", "output spawns.xml path")
	fs.String("houses", "", "output houses.xml path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	fmt.Println("not yet implemented")
	return nil
}
