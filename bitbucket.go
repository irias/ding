package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

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
