package main

import (
	"time"
)

// Repository with origin and build script.
type Repo struct {
	Id           int    `json:"id"`
	Name         string `json:"name"`          // short name for repo, typically last element of repo URL/path
	VCS          string `json:"vcs"`           // `git`, `mercurial` or `command`
	Origin       string `json:"origin"`        // git/mercurial "URL" (as understood by the respective commands), often SSH or HTTPS. if `vcs` is `command`, this is executed using sh.
	CheckoutPath string `json:"checkout_path"` // path to place the checkout in.
	BuildScript  string `json:"build_script"`  // shell scripts that compiles the software, runs tests, and creates releasable files.
}

// Repo and its most recent build per branch.
type RepoBuilds struct {
	Repo   Repo    `json:"repo"`
	Builds []Build `json:"builds"`
}

// File created during a build, as the result of a build. Files like this can be released.
type Result struct {
	Command   string `json:"command"`   // short name of command, without version, as you would want to run it from a command-line
	Version   string `json:"version"`   // typically semvar, x.y.z
	Os        string `json:"os"`        // eg `any`, `linux`, `darwin, `openbsd`, `windows`
	Arch      string `json:"arch"`      // eg `any`, `amd64`, `arm64`
	Toolchain string `json:"toolchain"` // string describing the tools used during build, eg SDK version
	Filename  string `json:"filename"`  // path relative to the checkout directory where build.sh is run
	Filesize  int64  `json:"filesize"`  // size of filename
}

// An attempt at building a repository.
type Build struct {
	Id              int        `json:"id"`
	RepoId          int        `json:"repo_id"`
	Branch          string     `json:"branch"`
	CommitHash      string     `json:"commit_hash"` // can be empty until `checkout` step, when building latest version of a branch
	Status          string     `json:"status"`      // `new`, `clone`, `checkout`, `build`, `success`
	Start           time.Time  `json:"start"`
	Finish          *time.Time `json:"finish"`
	ErrorMessage    string     `json:"error_message"`
	Results         []Result   `json:"results"`
	Released        *time.Time `json:"released"`
	BuilddirRemoved bool       `json:"builddir_removed"`

	LastLine  string `json:"last_line"`  // last line from last steps output
	DiskUsage int64  `json:"disk_usage"` // disk usage for build
}

// Step and the output generated while executing it.
type Step struct {
	Name   string `json:"name"` // same values as build.status
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
	Output string `json:"output"` // combined output of stdout and stderr
	Nsec   int64  `json:"nsec"`   // time it took this step to finish, initially 0
}

type BuildResult struct {
	Build       Build  `json:"build"`
	BuildScript string `json:"build_script"`
	Steps       []Step `json:"steps"`
}
