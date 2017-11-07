package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
)

type Script struct {
	Version  int
	Filename string
	SQL      string
}

func parseSQLScripts() (scripts []Script) {
	f, err := httpFS.Open("/sql.json")
	check(err, "opening sql scripts")
	check(json.NewDecoder(f).Decode(&scripts), "parsing sql scripts")
	check(f.Close(), "closing sql scripts")

	lastScript := scripts[len(scripts)-1]
	if lastScript.Version != DB_VERSION {
		log.Fatalf("DB_VERSION %d does not match last upgrade script with version %d\n", DB_VERSION, lastScript.Version)
	}
	return scripts
}

func runScripts(tx *sql.Tx, dbVersion int, scripts []Script, committing bool) {
	for _, script := range scripts {
		if script.Version <= dbVersion {
			continue
		}
		_, err := tx.Exec(script.SQL)
		check(err, fmt.Sprintf("executing upgrade script %d: %s: %s", script.Version, script.Filename, err))

		var version int
		err = tx.QueryRow("select max(version) from schema_upgrades").Scan(&version)
		check(err, "fetching database schema version after upgrade")
		if version != script.Version {
			log.Fatalf("invalid upgrade script %s: database not at version %d after running, but at %d\n", script.Filename, script.Version, version)
		}

		switch script.Version {
		case 5:
			var repoNames []string
			checkRow(tx.QueryRow(`select coalesce(json_agg(name), '[]') from repo`), &repoNames, "reading repos from database")
			for _, repoName := range repoNames {
				dir := "data/config/" + repoName
				buildSh := readFile(dir + "/build.sh")
				testSh := readFileLax(dir + "/test.sh")
				releaseSh := readFileLax(dir + "/release.sh")
				buildSh += "\n\necho step:test\n" + testSh
				buildSh += "\n\necho step:release\n" + releaseSh
				var id int
				err = tx.QueryRow(`update repo set build_script=$1 where name=$2 returning id`, buildSh, repoName).Scan(&id)
				if err != nil {
					log.Printf("setting repo.build_script for repo %s: %s\n", repoName, err)
				}

				remove := func(path string) {
					err := os.Remove(path)
					if err != nil {
						log.Printf("cleaning up shell script %s: %s", path, err)
					}
				}
				if !committing {
					log.Fatal("aborting upgrade, cannot rollback changes to disk")
				}
				remove(dir + "/build.sh")
				remove(dir + "/test.sh")
				remove(dir + "/release.sh")
			}
		}
	}
	return
}

func upgrade(args []string) {
	fs := flag.NewFlagSet("upgrade", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: ding upgrade config.json [commit]")
		fs.PrintDefaults()
	}
	fs.Parse(args)
	args = fs.Args()
	switch len(args) {
	case 1:
	case 2:
		if args[1] != "commit" {
			flag.Usage()
			os.Exit(2)
		}
	default:
		fs.Usage()
		os.Exit(2)
	}

	parseConfig(args[0])
	scripts := parseSQLScripts()
	lastScript := scripts[len(scripts)-1]

	var err error
	database, err = sql.Open("postgres", config.Database)
	check(err, "connecting to database")

	tx, err := database.Begin()
	check(err, "beginning transaction")

	var have bool
	err = tx.QueryRow("select exists (select 1 from pg_tables where schemaname='public' and tablename='schema_upgrades')").Scan(&have)
	check(err, "checking whether table schema_upgrades exists")

	var dbVersion int
	if have {
		err = tx.QueryRow("select max(version) from schema_upgrades").Scan(&dbVersion)
		check(err, "finding database schema version")

		lastScript := scripts[len(scripts)-1]
		if dbVersion == lastScript.Version {
			fmt.Println("database already at latest version", dbVersion)
			os.Exit(0)
		}
		_, err = fmt.Printf("upgrading database from version %d to %d...\n", dbVersion, lastScript.Version)
		check(err, "write")
	} else {
		_, err = fmt.Printf("initializing database to latest version %d...\n", lastScript.Version)
		check(err, "write")
		dbVersion = -1
	}

	committing := len(args) == 2
	runScripts(tx, dbVersion, scripts, committing)

	if committing {
		check(tx.Commit(), "committing")
		_, err = fmt.Printf("upgrade to version %d committed\n", lastScript.Version)
		check(err, "write")
	} else {
		check(tx.Rollback(), "rolling back")
		_, err = fmt.Println("upgrade rolled back, run again with an additional parameter 'commit'")
		check(err, "write")
	}
}
