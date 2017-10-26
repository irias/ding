package main

import (
	"database/sql"
	"time"
)

type Result struct {
	Command   string `json:"command"`
	Version   string `json:"version"`
	Os        string `json:"os"`
	Arch      string `json:"arch"`
	Toolchain string `json:"toolchain"`
	Filename  string `json:"filename"`
}

type Build struct {
	Id         int        `json:"id"`
	RepoId     int        `json:"repo_id"`
	Branch     string     `json:"branch"`
	CommitHash string     `json:"commit_hash"`
	Status     string     `json:"status"`
	Start      time.Time  `json:"start"`
	Finish     *time.Time `json:"finish"`
	Results    []Result   `json:"results"`
}

type Step struct {
	Name   string `json:"name"`
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
	Output string `json:"output"`
	Nsec   int64  `json:"nsec"`
}

type BuildResult struct {
	Build      Build      `json:"build"`
	RepoConfig RepoConfig `json:"repo_config"`
	Steps      []Step     `json:"steps"`
}

func _build(tx *sql.Tx, id int) (b Build) {
	q := `select row_to_json(bwr.*) from build_with_result bwr where id = $1`
	checkParseRow(tx.QueryRow(q, id), &b, "fetching build")
	return
}
