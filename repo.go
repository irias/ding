package main

import (
	"database/sql"
)

type Repo struct {
	Id     int    `json:"id"`
	Name   string `json:"name"`
	Origin string `json:"origin"`
}

type RepoConfig struct {
	BuildScript   string `json:"build_script"`
	TestScript    string `json:"test_script"`
	ReleaseScript string `json:"release_script"`
}

type RepoBuilds struct {
	Repo   Repo    `json:"repo"`
	Builds []Build `json:"builds"`
}

func _repo(tx *sql.Tx, repoName string) Repo {
	q := `select row_to_json(repo.*) from repo where name=$1`
	var r Repo
	checkParseRow(tx.QueryRow(q, repoName), &r, "fetching repo")
	return r
}
