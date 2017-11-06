package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"mime"
	"net/http"
	"os"
	"runtime/debug"
	"strconv"
	"strings"

	"bitbucket.org/mjl/sherpa"
	"github.com/irias/sherpa-prometheus-collector"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func sherpaCheck(err error, msg string) {
	if err == nil {
		return
	}

	m := msg
	if m != "" {
		m += ": "
	}
	m += err.Error()
	if config.PrintSherpaErrorStack {
		log.Println("sherpa serverError:", m)
		debug.PrintStack()
	}
	if config.ShowSherpaErrors {
		m = msg + ": " + err.Error()
	} else {
		m = "An error occurred. Please try again later or contact us."
	}
	serverError(m)
}

func serverError(m string) {
	panic(&sherpa.Error{Code: "serverError", Message: m})
}

func sherpaUserCheck(err error, msg string) {
	if err == nil {
		return
	}

	m := msg
	if m != "" {
		m += ": "
	}
	m += err.Error()
	if config.PrintSherpaErrorStack {
		log.Println("sherpa userError:", m)
		debug.PrintStack()
	}
	if config.ShowSherpaErrors {
		m = msg + ": " + err.Error()
	} else {
		m = "An error occurred. Please try again later or contact us."
	}
	userError(m)
}

func userError(m string) {
	panic(&sherpa.Error{Code: "userError", Message: m})
}

func checkParseRow(row *sql.Row, r interface{}, msg string) {
	var buf []byte
	err := row.Scan(&buf)
	if err == sql.ErrNoRows {
		panic(&sherpa.Error{Code: "userNotFound", Message: "Not found"})
	}
	sherpaCheck(err, msg+": reading json from database row into buffer")
	sherpaCheck(json.Unmarshal(buf, r), msg+": parsing json from database")
}

func serve(args []string) {
	serveFlag.Init("serve", flag.ExitOnError)
	serveFlag.Usage = func() {
		fmt.Println("usage: ding [flags] serve config.json")
		serveFlag.PrintDefaults()
	}
	serveFlag.Parse(args)
	args = serveFlag.Args()
	if len(args) != 1 {
		serveFlag.Usage()
		os.Exit(2)
	}

	parseConfig(args[0])

	var err error
	database, err = sql.Open("postgres", config.Database)
	check(err, "opening database connection")
	var dbVersion int
	err = database.QueryRow("select max(version) from schema_upgrades").Scan(&dbVersion)
	check(err, "fetching database schema version")
	if dbVersion != DB_VERSION {
		log.Fatalf("bad database schema version, expected %d, saw %d", DB_VERSION, dbVersion)
	}

	// mostly here to ensure go http lib doesn't do content sniffing. if it does, file serving breaks because seeking http assets is only partially implemeneted.
	mime.AddExtensionType(".woff", "font/woff")
	mime.AddExtensionType(".woff2", "font/woff2")
	mime.AddExtensionType(".eot", "application/vnd.ms-fontobject")
	mime.AddExtensionType(".svg", "image/svg+xml")
	mime.AddExtensionType(".ttf", "font/ttf")
	mime.AddExtensionType(".otf", "font/otf")
	mime.AddExtensionType(".map", "application/json") // browser trying to look for css/js .map files

	var doc sherpa.Doc
	ff, err := httpFS.Open("/ding.json")
	check(err, "opening sherpa docs")
	err = json.NewDecoder(ff).Decode(&doc)
	check(err, "parsing sherpa dos")
	err = ff.Close()
	check(err, "closing sherpa docs after parsing")

	collector, err := collector.NewCollector("mraelaadkaart", nil)
	check(err, "creating sherpa prometheus collector")

	handler, err := sherpa.NewHandler("/ding/", version, Ding{}, &doc, collector)
	check(err, "making sherpa handler")

	http.HandleFunc("/", serveAsset)
	http.Handle("/ding/", handler)
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/release/", serveRelease)
	http.HandleFunc("/result/", serveResult)

	log.Printf("version %s, listening on %s\n", version, *listenAddress)
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}

func serveAsset(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, "/") {
		r.URL.Path += "index.html"
	}
	f, err := httpFS.Open(r.URL.Path)
	if err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		}
		log.Printf("serving asset %s: %s\n", r.URL.Path, err)
		http.Error(w, "500 - Server error", 500)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		log.Printf("serving asset %s: %s\n", r.URL.Path, err)
		http.Error(w, "500 - Server error", 500)
		return
	}

	if info.IsDir() {
		http.NotFound(w, r)
		return
	}

	_, haveCacheBuster := r.URL.Query()["v"]
	cache := "no-cache, max-age=0"
	if haveCacheBuster {
		cache = fmt.Sprintf("public, max-age=%d", 31*24*3600)
	}
	w.Header().Set("Cache-Control", cache)

	http.ServeContent(w, r, r.URL.Path, info.ModTime(), f)
}

func serveRelease(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "bad method", 405)
		return
	}
	t := strings.Split(r.URL.Path[1:], "/")
	if len(t) != 4 || t[1] == ".." || t[1] == "." || t[2] == ".." || t[2] == "." || t[3] == ".." || t[3] == "." {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, fmt.Sprintf("data/release/%s/%s/%s", t[1], t[2], t[3]))
}

func serveResult(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "bad method", 405)
		return
	}
	t := strings.Split(r.URL.Path[1:], "/")
	if len(t) != 4 || t[1] == ".." || t[1] == "." || t[2] == ".." || t[2] == "." || t[3] == ".." || t[3] == "." {
		http.NotFound(w, r)
		return
	}
	repoName := t[1]
	buildId, err := strconv.Atoi(t[2])
	if err != nil {
		http.NotFound(w, r)
		return
	}
	basename := t[3]

	fail := func(err error) {
		log.Printf("error fetching result: %s\n", err)
		http.Error(w, "internal error", 500)
	}

	q := `
		select result.filename
		from result
		join build on result.build_id = build.id
		join repo on build.repo_id = repo.id
		where repo.name=$1 and build.id=$2
	`
	rows, err := database.Query(q, repoName, buildId)
	if err != nil {
		fail(err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		err = rows.Scan(&name)
		if err != nil {
			fail(err)
			return
		}
		if strings.HasSuffix(name, "/"+basename) {
			path := fmt.Sprintf("data/build/%s/%d/checkout/%s/%s", repoName, buildId, repoName, name)
			http.ServeFile(w, r, path)
			return
		}
	}
	if err = rows.Err(); err != nil {
		fail(err)
		return
	}
	http.NotFound(w, r)
}
