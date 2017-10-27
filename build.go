package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

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
	Id           int        `json:"id"`
	RepoId       int        `json:"repo_id"`
	Branch       string     `json:"branch"`
	CommitHash   string     `json:"commit_hash"`
	Status       string     `json:"status"`
	Start        time.Time  `json:"start"`
	Finish       *time.Time `json:"finish"`
	ErrorMessage string     `json:"error_message"`
	Results      []Result   `json:"results"`

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
	Build       Build      `json:"build"`
	BuildConfig RepoConfig `json:"build_config"`
	Steps       []Step     `json:"steps"`
}

func fillBuild(repoName string, b *Build) {
	// add disk usage
	b.DiskUsage = 0
	buildDir := fmt.Sprintf("build/%s/%d", repoName, b.Id)
	filepath.Walk(buildDir, func(path string, info os.FileInfo, err error) error {
		if err == nil {
			const overhead = 2 * 1024
			b.DiskUsage += overhead + info.Size()
		}
		return nil
	})

	if b.Finish == nil || b.Status == "success" {
		return
	}
	path := fmt.Sprintf("build/%s/%d/output/%s.output", repoName, b.Id, b.Status)
	f, err := os.Open(path)
	if err != nil {
		b.LastLine = fmt.Sprintf("(open for last line: %s)", err)
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		s := scanner.Text()
		if s != "" {
			b.LastLine = s
		}
	}
	if err = scanner.Err(); err != nil {
		b.LastLine = fmt.Sprintf("(reading for last line: %s)", err)
	}
}

func _build(tx *sql.Tx, repoName string, id int) (b Build) {
	q := `select row_to_json(bwr.*) from build_with_result bwr where id = $1`
	checkParseRow(tx.QueryRow(q, id), &b, "fetching build")
	fillBuild(repoName, &b)
	return
}
