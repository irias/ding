package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"bitbucket.org/mjl/httpasset"
)

const (
	DB_VERSION = 8
)

var (
	httpFS  http.FileSystem
	version string = "dev"
	config  struct {
		ShowSherpaErrors      bool
		PrintSherpaErrorStack bool
		Database              string
		Environment           map[string]string
		Notify                struct {
			Name  string
			Email string
		}
		BaseURL             string
		GithubWebhookSecret string // for github webhook "push" events, to create a build; configure the same secret as in your github repository settings.
		IsolateBuilds       struct {
			Enabled    bool     // if false, we run all build commands as the user running ding.  if true, we run each build under its own uid.
			UidStart   int      // we'll use this + buildId as the unix uid to run the commands under
			UidEnd     int      // if we reach this uid, we wrap around to uidStart again
			DingUid    int      // the unix uid ding runs as, used to chown files back before deleting.
			DingGid    int      // the unix gid ding runs as, used to run build commands under.
			Runas      []string // if enabled is true, the build commands are prepended with: these parameters, followed by a uid to run as, followed by a gid to run as.
			ChownBuild []string // if enabled is true, this command is executed and must restore file permissions in the build directory so files can be removed by ding.  this command is run with these parameters: uid, gid, one or more paths.
			BuildsDir  string   // absolute path to the build/ directory, for checking by chownbuild
		}
		Mail struct {
			Enabled,
			SmtpTls bool
			SmtpPort int
			SmtpHost,
			SmtpUsername,
			SmtpPassword,
			From,
			FromName,
			ReplyTo,
			ReplyToName string
		}
	}
	database *sql.DB
)

func check(err error, msg string) {
	if err == nil {
		return
	}
	log.Fatalf("%s: %s\n", msg, err)
}

func init() {
	httpFS = httpasset.Fs()
	if err := httpasset.Error(); err != nil {
		log.Println("falling back to local assets:", err)
		httpFS = http.Dir("assets")
	}
}

func parseConfig(path string) {
	f, err := os.Open(path)
	check(err, "opening config file")
	err = json.NewDecoder(f).Decode(&config)
	check(err, "parsing config file")
	err = f.Close()
	check(err, "closing config file")
}

func main() {
	log.SetFlags(0)
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: ding help")
		fmt.Fprintln(os.Stderr, "       ding serve config.json")
		fmt.Fprintln(os.Stderr, "       ding chownbuild config.json uid gid path")
		fmt.Fprintln(os.Stderr, "       ding upgrade config.json [commit]")
		fmt.Fprintln(os.Stderr, "       ding version")
		flag.PrintDefaults()
	}
	flag.Parse()
	args := flag.Args()
	if len(args) == 0 {
		flag.Usage()
		os.Exit(2)
	}

	cmd := args[0]
	args = args[1:]
	switch cmd {
	case "help":
		help(args)
	case "serve":
		serve(args)
	case "chownbuild":
		chownbuild(args)
	case "upgrade":
		upgrade(args)
	case "version":
		fl := flag.NewFlagSet(cmd, flag.ExitOnError)
		fl.Usage = func() {
			fmt.Println("usage: ding version")
			fl.PrintDefaults()
		}
		fl.Parse(args)
		if len(fl.Args()) != 0 {
			fl.Usage()
			os.Exit(2)
		}
		fmt.Printf("%s\ndatabase schema version %d\n", version, DB_VERSION)
	default:
		flag.Usage()
	}
}
