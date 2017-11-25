// Ding is a self-hosted build server for developers.
//
// See the INSTALL.md file for installation instructions, or run "ding help".
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
		BaseURL                string
		GithubWebhookSecret    string   // for github webhook "push" events, to create a build; configure the same secret as in your github repository settings.
		BitbucketWebhookSecret string   // we use this in the URL the user must configure at bitbucket; they don't have any other authentication mechanism.
		Run                    []string // prefixed to commands we run. e.g. call "nice" or "timeout"
		IsolateBuilds          struct {
			Enabled  bool // if false, we run all build commands as the user running ding.  if true, we run each build under its own uid.
			UidStart int  // we'll use this + buildId as the unix uid to run the commands under
			UidEnd   int  // if we reach this uid, we wrap around to uidStart again
			DingUid  int  // the unix uid ding runs as, used to chown files back before deleting.
			DingGid  int  // the unix gid ding runs as, used to run build commands under.
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
	if err != nil {
		log.Fatalf("%s: %s\n", msg, err)
	}
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
	case "serve-http":
		// undocumented, for unpriviliged http process
		servehttp(args)
	case "upgrade":
		upgrade(args)
	case "version":
		_version(args)
	default:
		flag.Usage()
		os.Exit(2)
	}
}
