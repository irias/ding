package main

import (
	"crypto/hmac"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
)

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
