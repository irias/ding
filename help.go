package main

import (
	"flag"
	"fmt"
	"io"
	"os"
)

func help(args []string) {
	fs := flag.NewFlagSet("help", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: ding help")
		fs.PrintDefaults()
	}
	fs.Parse(args)
	args = fs.Args()
	if len(args) != 0 {
		fs.Usage()
		os.Exit(2)
	}

	f, err := httpFS.Open("/INSTALL.md")
	check(err, "opening install instructions")
	_, err = io.Copy(os.Stdout, f)
	check(err, "copy")
	check(f.Close(), "close")
}
