package main

import (
	"bufio"
	"fmt"
	"os"
)

func fillBuild(repoName string, b *Build) {
	// we only fill it in if not already set. typically during a build.
	if b.DiskUsage == 0 {
		buildDir := fmt.Sprintf("data/build/%s/%d", repoName, b.ID)
		b.DiskUsage = buildDiskUsage(buildDir)
	}

	if b.Finish == nil || b.Status == "success" {
		return
	}
	path := fmt.Sprintf("data/build/%s/%d/output/%s.output", repoName, b.ID, b.Status)
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
