package main

import (
	"time"
)

type Repo struct {
	Id           int    `json:"id"`
	Name         string `json:"name"`
	Origin       string `json:"origin"`
	CheckoutPath string `json:"checkout_path"`
	BuildScript  string `json:"build_script"`
}

type RepoBuilds struct {
	Repo   Repo    `json:"repo"`
	Builds []Build `json:"builds"`
}

type Result struct {
	Command   string `json:"command"`
	Version   string `json:"version"`
	Os        string `json:"os"`
	Arch      string `json:"arch"`
	Toolchain string `json:"toolchain"`
	Filename  string `json:"filename"`
	Filesize  int64  `json:"filesize"`
}

type Build struct {
	Id              int        `json:"id"`
	RepoId          int        `json:"repo_id"`
	Branch          string     `json:"branch"`
	CommitHash      string     `json:"commit_hash"`
	Status          string     `json:"status"`
	Start           time.Time  `json:"start"`
	Finish          *time.Time `json:"finish"`
	ErrorMessage    string     `json:"error_message"`
	Results         []Result   `json:"results"`
	Released        *time.Time `json:"released"`
	BuilddirRemoved bool       `json:"builddir_removed"`

	LastLine  string `json:"last_line"`  // last line from last steps output
	DiskUsage int64  `json:"disk_usage"` // disk usage for build
}

type Step struct {
	Name   string `json:"name"`
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
	Output string `json:"output"`
	Nsec   int64  `json:"nsec"`
}

type BuildResult struct {
	Build       Build  `json:"build"`
	BuildScript string `json:"build_script"`
	Steps       []Step `json:"steps"`
}
