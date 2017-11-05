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
	DB_VERSION = 4
)

var (
	serveFlag     = flag.NewFlagSet("serve", flag.ExitOnError)
	httpFS        http.FileSystem
	version       string = "dev"
	listenAddress        = flag.String("listen", ":6084", "address to listen on")
	config        struct {
		ShowSherpaErrors      bool
		PrintSherpaErrorStack bool
		Database              string
		Environment           map[string]string
		Notify                struct {
			Name  string
			Email string
		}
		BaseURL      string
		SudoBuild    bool     // if false, we run all build commands as the user running ding.  if true, we run each build under its own uid.
		SudoUidStart int      // we'll use this + buildId as the unix uid to run the commands under
		SudoUidEnd   int      // if we reach this uid, we wrap around to sudoUidStart again
		SudoUid      int      // the unix uid ding runs as, used to chown files back before deleting.
		SudoGid      int      // the unix gid ding runs as, used to run build commands under.
		Runas        []string // if SudoBuild is true, the build commands are prepended with: these parameters, followed by a uid to run as, followed by a gid to run as.
		ChownBuild   []string // if SudoBuild is true, this command is executed and must restore file permissions in the build directory so files can be removed by ding.  this command is run with these parameters: uid, gid, one or more paths.
		BuildsDir    string   // absolute path to the build/ directory, for checking by chownbuild
		Mail         struct {
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
		fmt.Println("usage: ding [flags] serve config.json")
		fmt.Println("usage: ding [flags] chownbuild config.json uid gid path")
		flag.PrintDefaults()
	}
	flag.Parse()
	args := flag.Args()
	if len(args) == 0 {
		flag.Usage()
		os.Exit(2)
	}

	switch args[0] {
	case "serve":
		serve(args[1:])
	case "chownbuild":
		chownbuild(args[1:])
	default:
		flag.Usage()
	}
}
