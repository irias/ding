package main

import (
	"database/sql"
)

func _removeBuild(tx *sql.Tx, repoName string, buildID int) {
	var filenames []string
	qres := `select coalesce(json_agg(filename), '[]') from result where build_id=$1`
	sherpaCheckRow(tx.QueryRow(qres, buildID), &filenames, "fetching released files")

	_, err := tx.Exec(`delete from result where build_id=$1`, buildID)
	sherpaCheck(err, "removing results from database")

	builddirRemoved := false
	q := `delete from build where id=$1 returning builddir_removed`
	sherpaCheckRow(tx.QueryRow(q, buildID), &builddirRemoved, "removing build from database")

	if !builddirRemoved {
		_removeDir(repoName, buildID)
	}
}

func _removeDir(repoName string, buildID int) {
	req := request{msg{msgRemovedir, repoName, buildID, "", nil}, make(chan error, 0), nil}
	rootRequests <- req
	err := <-req.errorResponse
	sherpaCheck(err, "removing files")
}

func _removeBuilddir(tx *sql.Tx, repoName string, buildID int) {
	err := tx.QueryRow("update build set builddir_removed=true where id=$1 returning id", buildID).Scan(&buildID)
	sherpaCheck(err, "marking builddir as removed in database")

	_removeDir(repoName, buildID)
}
