package main

import (
	"time"
)

// Repo is a repository as stored in the database.
type Repo struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`          // short name for repo, typically last element of repo URL/path
	VCS          string `json:"vcs"`           // `git`, `mercurial` or `command`
	Origin       string `json:"origin"`        // git/mercurial "URL" (as understood by the respective commands), often SSH or HTTPS. if `vcs` is `command`, this is executed using sh.
	CheckoutPath string `json:"checkout_path"` // path to place the checkout in.
	BuildScript  string `json:"build_script"`  // shell scripts that compiles the software, runs tests, and creates releasable files.
}

// RepoBuilds is a repository and its most recent build per branch.
type RepoBuilds struct {
	Repo   Repo    `json:"repo"`
	Builds []Build `json:"builds"`
}

// Result is a file created during a build, as the result of a build. Files like this can be released.
type Result struct {
	Command   string `json:"command"`   // short name of command, without version, as you would want to run it from a command-line
	Version   string `json:"version"`   // typically semvar, x.y.z
	Os        string `json:"os"`        // eg `any`, `linux`, `darwin, `openbsd`, `windows`
	Arch      string `json:"arch"`      // eg `any`, `amd64`, `arm64`
	Toolchain string `json:"toolchain"` // string describing the tools used during build, eg SDK version
	Filename  string `json:"filename"`  // path relative to the checkout directory where build.sh is run
	Filesize  int64  `json:"filesize"`  // size of filename
}

// Build is an attempt at building a repository.
type Build struct {
	ID              int        `json:"id"`
	RepoID          int        `json:"repo_id"`
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

// Step is one phase of a build and stores the output generated in that step.
type Step struct {
	Name   string `json:"name"` // same values as build.status
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
	Output string `json:"output"` // combined output of stdout and stderr
	Nsec   int64  `json:"nsec"`   // time it took this step to finish, initially 0
}

// BuildResult is the stored result of a build, including the build script and step outputs.
type BuildResult struct {
	Build       Build  `json:"build"`
	BuildScript string `json:"build_script"`
	Steps       []Step `json:"steps"`
}
