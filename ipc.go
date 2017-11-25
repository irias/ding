package main

import (
	"os"
)

// message from unprivileged webserver to root process
type msg struct {
	Kind msgKind

	RepoName     string
	BuildId      int
	CheckoutPath string   // for the workdir of the build command
	Env          []string // environment when building
}

type msgKind int

const (
	MsgChown     = msgKind(iota) // chown the homedir & checkoutdir of a build
	MsgRemovedir                 // remove a builddir, or (if buildId < 0), an entire repo
	MsgBuild                     // start a build by running build.sh
)

// request from one of the http handlers to httpserve's request mux
type request struct {
	msg           msg
	errorResponse chan error
	buildResponse chan buildResult
}

// result of starting a build
type buildResult struct {
	err    error // if non-nil, quick failure.  otherwise, the files below will send updates
	stdout *os.File
	stderr *os.File
	status *os.File // we read a gob-encoded string from status as the exit string
}
