package main

import (
	"time"
)

type Repo struct {
	Id           int    `json:"id"`
	Name         string `json:"name"`          // short name for repo, typically last element of repo URL/path
	VCS          string `json:"vcs"`           // `git`, `mercurial` or `command`
	Origin       string `json:"origin"`        // git/mercurial "URL" (as understood by the respective commands), often SSH or HTTPS. if `vcs` is `command`, this is executed using sh.
	CheckoutPath string `json:"checkout_path"` // path to place the checkout in.
	BuildScript  string `json:"build_script"`  // shell scripts that compiles the software, runs tests, and creates releasable files.
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
