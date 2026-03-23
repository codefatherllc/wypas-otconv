package items

import (
	"flag"
	"fmt"
)

func Unpack(args []string) error {
	fs := flag.NewFlagSet("items unpack", flag.ExitOnError)
	fs.String("oti", "", "path to .oti file")
	fs.String("otb", "", "output items.otb path")
	fs.String("xml", "", "output items.xml path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	fmt.Println("not yet implemented")
	return nil
}
