package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
)

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
