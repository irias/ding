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
	if uid != config.SudoUid && !(uid >= config.SudoUidStart && uid < config.SudoUidEnd) {
		log.Fatalf("uid %d not allowed, not config.SudoUid %d and not between config.SudoUidStart %d and config.SudoUidEnd %d\n", uid, config.SudoUid, config.SudoUidStart, config.SudoUidEnd)
	}
	if gid != config.SudoGid {
		log.Fatalf("gid %d not allowed, not config.SudoGid %d\n", gid, config.SudoGid)
	}
	for _, path := range args[3:] {
		if !strings.HasPrefix(path, config.BuildsDir) {
			log.Fatalf("path %s not within config.BuildsDir %s\n", path, config.BuildsDir)
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
