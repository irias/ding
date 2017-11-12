package main

import (
	"crypto/hmac"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
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

var (
	dingWorkDir          string
	serveFlag            = flag.NewFlagSet("serve", flag.ExitOnError)
	listenAddress        = serveFlag.String("listen", ":6084", "address to listen on")
	listenWebhookAddress = serveFlag.String("listenwebhook", ":6085", "address to listen on for webhooks, like from github; set empty for no listening")
	webhookHandler       *http.ServeMux
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

func sherpaCheckRow(row *sql.Row, r interface{}, msg string) {
	var buf []byte
	err := row.Scan(&buf)
	if err == sql.ErrNoRows {
		panic(&sherpa.Error{Code: "userNotFound", Message: "Not found"})
	}
	sherpaCheck(err, msg+": reading json from database row into buffer")
	sherpaCheck(json.Unmarshal(buf, r), msg+": parsing json from database")
}

func checkRow(row *sql.Row, r interface{}, msg string) {
	var buf []byte
	err := row.Scan(&buf)
	if err == sql.ErrNoRows {
		log.Fatal("no row in result")
	}
	check(err, msg+": reading json from database row into buffer")
	check(json.Unmarshal(buf, r), msg+": parsing json from database")
}

type job struct {
	repoName string
	rc       chan struct{}
}

var (
	newJobs      chan job
	finishedJobs chan string // repoName
)

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

	collector, err := collector.NewCollector("ding", nil)
	check(err, "creating sherpa prometheus collector")

	handler, err := sherpa.NewHandler("/ding/", version, Ding{}, &doc, collector)
	check(err, "making sherpa handler")

	http.HandleFunc("/", serveAsset)
	http.HandleFunc("/LICENSES", serveAsset)
	http.Handle("/ding/", handler)
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/release/", serveRelease)
	http.HandleFunc("/result/", serveResult)
	http.HandleFunc("/events", serveEvents)

	go eventMux()

	dingWorkDir, err = os.Getwd()
	check(err, "getting current work dir")

	newJobs = make(chan job, 1)
	finishedJobs = make(chan string, 1)
	go func() {
		active := map[string]struct{}{}
		pending := map[string][]job{}

		kick := func(repoName string) {
			if _, ok := active[repoName]; ok {
				return
			}
			jobs := pending[repoName]
			if len(jobs) == 0 {
				return
			}
			job := jobs[0]
			pending[repoName] = jobs[1:]
			active[repoName] = struct{}{}
			job.rc <- struct{}{}
		}

		for {
			select {
			case job := <-newJobs:
				pending[job.repoName] = append(pending[job.repoName], job)
				kick(job.repoName)

			case repoName := <-finishedJobs:
				delete(active, repoName)
				kick(repoName)
			}
		}
	}()

	unfinishedMsg := "marked as failed/unfinished at ding startup."
	result, err := database.Exec(`update build set finish=now(), error_message=$1 where finish is null and status!='new'`, unfinishedMsg)
	check(err, "marking unfinished builds as failed")
	rows, err := result.RowsAffected()
	check(err, "reading affected rows for marking unfinished builds as failed")
	if rows > 0 {
		log.Printf("marked %d unfinished builds as failed\n", rows)
	}

	var buf []byte
	var newBuilds []struct {
		Repo  Repo
		Build Build
	}
	qnew := `
		select coalesce(json_agg(x.*), '[]') from (
			select row_to_json(repo.*) as repo, row_to_json(build.*) as build from repo join build on repo.id = build.repo_id where status='new'
		) x
	`
	check(database.QueryRow(qnew).Scan(&buf), "fetching new builds from database")
	check(json.Unmarshal(buf, &newBuilds), "parsing new builds from database")
	for _, repoBuild := range newBuilds {
		func(repo Repo, build Build) {
			job := job{
				repo.Name,
				make(chan struct{}),
			}
			newJobs <- job
			go func() {
				<-job.rc
				defer func() {
					finishedJobs <- job.repoName
				}()

				buildDir := fmt.Sprintf("%s/data/build/%s/%d", dingWorkDir, repo.Name, build.Id)
				_doBuild(repo, build, buildDir)
			}()
		}(repoBuild.Repo, repoBuild.Build)
	}

	if *listenWebhookAddress != "" {
		log.Printf("ding version %s, listening on %s and for webhooks on %s\n", version, *listenAddress, *listenWebhookAddress)
		webhookHandler = http.NewServeMux()
		webhookHandler.HandleFunc("/github/", githubHookHandler)
		webhookHandler.HandleFunc("/bitbucket/", bitbucketHookHandler)
		go func() {
			server := &http.Server{Addr: *listenWebhookAddress, Handler: webhookHandler}
			log.Fatal(server.ListenAndServe())
		}()
	} else {
		log.Printf("ding version %s, listening on %s\n", version, *listenAddress)
	}
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}

func githubHookHandler(w http.ResponseWriter, r *http.Request) {
	if config.GithubWebhookSecret == "" {
		http.NotFound(w, r)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}

	if !strings.HasPrefix(r.URL.Path, "/github/") {
		http.NotFound(w, r)
		return
	}
	repoName := r.URL.Path[len("/github/"):]

	var vcs string
	err := database.QueryRow("select vcs from repo where name=$1", repoName).Scan(&vcs)
	if err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		log.Printf("github webhook: reading vcs from database: %s\n", err)
		http.Error(w, "error", 500)
		return
	}
	if !(vcs == "git" || vcs == "command") {
		log.Printf("github webhook: push event for a non-git repository\n")
		http.Error(w, "misconfigured repositories", 500)
		return
	}

	sigstr := strings.TrimSpace(r.Header.Get("X-Hub-Signature"))
	t := strings.Split(sigstr, "=")
	if len(t) != 2 || t[0] != "sha1" || len(t[1]) != 2*sha1.Size {
		http.Error(w, "malformed/missing X-Hub-Signature header", 400)
		return
	}
	sig, err := hex.DecodeString(t[1])
	if err != nil {
		http.Error(w, "malformed hex in X-Hub-Signature", 400)
		return
	}
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "error reading request", 500)
		return
	}
	mac := hmac.New(sha1.New, []byte(config.GithubWebhookSecret))
	mac.Write(buf)
	exp := mac.Sum(nil)
	if !hmac.Equal(exp, sig) {
		log.Printf("github webhook: bad signature, refusing message\n")
		http.Error(w, "invalid signature", 400)
		return
	}
	var event struct {
		Repository struct {
			Name string `json:"name"`
		} `json:"repository"`
		Ref   string `json:"ref"`
		After string `json:"after"`
	}
	err = json.Unmarshal(buf, &event)
	if err != nil {
		log.Printf("github webhook: bad JSON body: %s\n", err)
		http.Error(w, "bad json", 400)
		return
	}
	if event.Repository.Name != repoName {
		log.Printf("github webhook: repository does not match, github sent %s for URL for %s\n", event.Repository.Name, repoName)
		http.Error(w, "repository mismatch", 400)
		return
	}
	branch := "master"
	if strings.HasPrefix(event.Ref, "refs/heads/") {
		branch = event.Ref[len("refs/heads/"):]
	}
	commit := event.After
	repo, build, buildDir, err := prepareBuild(repoName, branch, commit)
	if err != nil {
		log.Printf("github webhook: error starting build for push event for repo %s, branch %s, commit %s\n", repoName, branch, commit)
		http.Error(w, "could not create build", 500)
		return
	}
	go doBuild(repo, build, buildDir)
	w.WriteHeader(204)
}

func bitbucketHookHandler(w http.ResponseWriter, r *http.Request) {
	if config.BitbucketWebhookSecret == "" {
		http.NotFound(w, r)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}

	if !strings.HasPrefix(r.URL.Path, "/bitbucket/") {
		http.NotFound(w, r)
		return
	}
	t := strings.Split(r.URL.Path[len("/bitbucket/"):], "/")
	if len(t) != 2 {
		http.NotFound(w, r)
		return
	}
	repoName := t[0]
	key := t[1]
	if key != config.BitbucketWebhookSecret {
		log.Printf("bitbucket webhook: invalid secret in request for repoName %s\n", repoName)
		http.NotFound(w, r)
		return
	}

	/*
		https://confluence.atlassian.com/bitbucket/event-payloads-740262817.html#EventPayloads-Push
		example of the relevant parts:
		{
			"push": {
				"changes": [
					{
						"new": {
							"heads": [
								{
									"hash": "2951856392c4ba466082948455bac7303404675f",
									"type": "commit"
								}
							],
							"name": "default",
							"type": "named_branch"  # or "branch" or "tag" for git
						}
					}
				]
			},
			"repository": {
				"name": "bitbuckethgwebhooktest",
				"scm": "hg"  # or "git"
			}
		}
	*/
	var event struct {
		Push *struct {
			Changes []struct {
				New *struct {
					Heads []struct {
						Hash string `json:"hash"`
						Type string `json:"type"`
					} `json:"heads"`
					Name string `json:"name"`
					Type string `json:"type"` // hg: named_branch, tag, bookmark; git: branch, tag
				} `json:"new"` // null for branch deletes
			} `json:"changes"`
		} `json:"push"`
		Repository struct {
			Name string `json:"name"`
			SCM  string `json:"scm"`
		} `json:"repository"`
	}
	err := json.NewDecoder(r.Body).Decode(&event)
	if err != nil {
		log.Printf("bitbucket webhook: parsing JSON body: %s\n", err)
		http.Error(w, "bad json", 400)
		return
	}
	if event.Repository.Name != repoName {
		log.Printf("bitbucket webhook: unexpected repoName %s at endpoint for repoName %s\n", event.Repository.Name, repoName)
		http.Error(w, "bad request", 400)
		return
	}

	var vcs string
	err = database.QueryRow("select vcs from repo where name=$1", repoName).Scan(&vcs)
	if err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		log.Printf("bitbucket webhook: reading vcs from database: %s\n", err)
		http.Error(w, "error", 500)
		return
	}
	if event.Repository.SCM == "hg" && !(vcs == "mercurial" || vcs == "command") {
		log.Printf("bitbucket webhook: misconfigured repository type, bitbucket thinks mercurial, ding thinks %s\n", vcs)
		http.Error(w, "misconfigured webhook", 500)
		return
	}
	if event.Repository.SCM == "git" && !(vcs == "git" || vcs == "command") {
		log.Printf("bitbucket webhook: misconfigured repository type, bitbucket thinks git, ding thinks %s\n", vcs)
		http.Error(w, "misconfigured webhook", 500)
		return
	}

	if event.Push == nil {
		http.Error(w, "missing push event", 400)
		return
	}
	for _, change := range event.Push.Changes {
		if change.New == nil {
			continue
		}
		var branch string
		switch change.New.Type {
		case "branch":
		case "named_branch":
			branch = change.New.Name
		case "tag":
			// todo: fix for silly assumption that people only tag in master/default branch (eg after merge)
			branch = "master"
			if vcs == "hg" {
				branch = "default"
			}
		default:
			// we ignore bookmarks
			continue
		}
		for _, head := range change.New.Heads {
			if head.Type == "commit" {
				commit := head.Hash
				repo, build, buildDir, err := prepareBuild(repoName, branch, commit)
				if err != nil {
					log.Printf("bitbucket webhook: error starting build for push event for repo %s, branch %s, commit %s\n", repoName, branch, commit)
					http.Error(w, "could not create build", 500)
					return
				}
				go doBuild(repo, build, buildDir)
			}
		}
	}
	w.WriteHeader(204)
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
		select repo.checkout_path, result.filename
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
		var repoCheckoutPath, name string
		err = rows.Scan(&repoCheckoutPath, &name)
		if err != nil {
			fail(err)
			return
		}
		if strings.HasSuffix(name, "/"+basename) {
			path := fmt.Sprintf("data/build/%s/%d/checkout/%s/%s", repoName, buildId, repoCheckoutPath, name)
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
