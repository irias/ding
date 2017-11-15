package main

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strings"
)

func _removeBuild(tx *sql.Tx, repoName string, buildId int) {
	var filenames []string
	qres := `select coalesce(json_agg(filename), '[]') from result where build_id=$1`
	sherpaCheckRow(tx.QueryRow(qres, buildId), &filenames, "fetching released files")

	_, err := tx.Exec(`delete from result where build_id=$1`, buildId)
	sherpaCheck(err, "removing results from database")

	builddirRemoved := false
	q := `delete from build where id=$1 returning builddir_removed`
	sherpaCheckRow(tx.QueryRow(q, buildId), &builddirRemoved, "removing build from database")

	if !builddirRemoved {
		buildDir := fmt.Sprintf("%s/data/build/%s/%d", dingWorkDir, repoName, buildId)
		_removeDir(buildDir)
	}
}

func _removeDir(path string) {
	if config.IsolateBuilds.Enabled {
		user, err := user.Current()
		sherpaCheck(err, "getting current uid/gid")
		chownbuild := append(config.IsolateBuilds.ChownBuild, string(user.Uid), string(user.Gid), path)
		cmd := exec.Command(chownbuild[0], chownbuild[1:]...)
		buf, err := cmd.CombinedOutput()
		if err != nil {
			serverError(fmt.Sprintf("changing user/group ownership of %s: %s: %s", path, err, strings.TrimSpace(string(buf))))
		}
	}

	err := os.RemoveAll(path)
	sherpaCheck(err, "removing files")
}

func _removeBuilddir(tx *sql.Tx, repoName string, buildId int) {
	err := tx.QueryRow("update build set builddir_removed=true where id=$1 returning id", buildId).Scan(&buildId)
	sherpaCheck(err, "marking builddir as removed in database")

	buildDir := fmt.Sprintf("%s/data/build/%s/%d", dingWorkDir, repoName, buildId)
	_removeDir(buildDir)
}
