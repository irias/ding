package main

import (
	"flag"
	"fmt"
	"os"

	"bitbucket.org/mjl/sherpa"
)

func kick(args []string) {
	fs := flag.NewFlagSet("kick", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: ding kick baseURL repoName branch commit")
		fs.PrintDefaults()
	}
	fs.Parse(args)
	args = fs.Args()
	if len(args) != 4 {
		fs.Usage()
		os.Exit(2)
	}

	baseURL := args[0]
	repoName := args[1]
	branch := args[2]
	commit := args[3]

	client, err := sherpa.NewClient(baseURL, []string{"build"})
	check(err, "initializing sherpa client")

	var build struct {
		ID int64
	}
	err = client.Call(&build, "createBuild", repoName, branch, commit)
	check(err, "building")
	_, err = fmt.Println("buildId", build.ID)
	check(err, "write")
}
