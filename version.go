package main

import (
	"flag"
	"fmt"
	"os"
)

func _version(args []string) {
	fl := flag.NewFlagSet("version", flag.ExitOnError)
	fl.Usage = func() {
		fmt.Println("usage: ding version")
		fl.PrintDefaults()
	}
	fl.Parse(args)
	if len(fl.Args()) != 0 {
		fl.Usage()
		os.Exit(2)
	}
	fmt.Printf("%s\ndatabase schema version %d\n", version, databaseVersion)
}
