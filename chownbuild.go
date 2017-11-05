package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func chownbuild(args []string) {
	fs := flag.NewFlagSet("chownbuild", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Println("usage: ding [flags] chownbuild /path/to/config.json uid gid path ...")
		fs.PrintDefaults()
	}
	fs.Parse(args)
	args = fs.Args()
	if len(args) < 4 {
		fs.Usage()
		os.Exit(2)
	}

	parseConfig(args[0])
	uid, err := strconv.Atoi(args[1])
	check(err, "parsing uid")
	gid, err := strconv.Atoi(args[2])
	check(err, "parsing gid")
	if !config.IsolateBuilds.Enabled {
		log.Fatalln("isolate buids not enabled")
	}
	if uid != config.IsolateBuilds.DingUid && !(uid >= config.IsolateBuilds.UidStart && uid < config.IsolateBuilds.UidEnd) {
		log.Fatalf("uid %d not allowed, not config.IsolateBuilds.DingUid %d and not between config.IsolateBuilds.UidStart %d and config.IsolateBuilds.UidEnd %d\n", uid, config.IsolateBuilds.DingUid, config.IsolateBuilds.UidStart, config.IsolateBuilds.UidEnd)
	}
	if gid != config.IsolateBuilds.DingGid {
		log.Fatalf("gid %d not allowed, not config.IsolateBuilds.DingGid %d\n", gid, config.IsolateBuilds.DingGid)
	}
	for _, path := range args[3:] {
		if !strings.HasPrefix(path, config.IsolateBuilds.BuildsDir) {
			log.Fatalf("path %s not within config.BuildsDir %s\n", path, config.IsolateBuilds.BuildsDir)
		}
		if strings.Contains(path, "..") {
			log.Fatalf("path %s contains suspect ..\n", path)
		}
	}
	params := append([]string{"chown", "-R", fmt.Sprintf("%d:%d", uid, gid)}, args[3:]...)
	cmd := exec.Command(params[0], params[1:]...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("chown failed: %s: %s\n", err, strings.TrimSpace(string(output)))
	}
}
