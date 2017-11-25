package main

import (
	"database/sql"
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
		_removeDir(repoName, buildId)
	}
}

func _removeDir(repoName string, buildId int) {
	req := request{msg{MsgRemovedir, repoName, buildId, "", nil}, make(chan error, 0), nil}
	rootRequests <- req
	err := <-req.errorResponse
	sherpaCheck(err, "removing files")
}

func _removeBuilddir(tx *sql.Tx, repoName string, buildId int) {
	err := tx.QueryRow("update build set builddir_removed=true where id=$1 returning id", buildId).Scan(&buildId)
	sherpaCheck(err, "marking builddir as removed in database")

	_removeDir(repoName, buildId)
}
