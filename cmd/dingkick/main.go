package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"bitbucket.org/mjl/sherpa"
)

var (
	version = "dev"
)

func usage() {
	fmt.Fprintln(os.Stderr, "usage: dingkick baseURL repoName branch commit")
	flag.PrintDefaults()
}

func check(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s\n", msg, err)
	}
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("dingkick: ")
	flag.Usage = usage
	flag.Parse()
	args := flag.Args()
	if len(args) != 4 {
		usage()
		os.Exit(2)
	}
	baseURL := args[0]
	repoName := args[1]
	branch := args[2]
	commit := args[3]

	client, err := sherpa.NewClient(baseURL, []string{"build"})
	check(err, "initializing sherpa client")

	var build struct {
		Id int64
	}
	err = client.Call(&build, "createBuild", repoName, branch, commit)
	check(err, "building")
	_, err = fmt.Println("buildId", build.Id)
	check(err, "write")
}
