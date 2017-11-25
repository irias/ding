package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"bitbucket.org/mjl/sherpa"
)

var (
	stepNames = []string{
		"clone",
		"build",
	}
)

// The Ding API lets you compile git branches, build binaries, run tests, and publish binaries.
type Ding struct {
	SSE SSE `sherpa:"Server-Sent Events"`
}

// Checks program status.
// If backend connectivity is broken, this sherpa call results in a 500 internal server error. Useful for monitoring tools.
func (Ding) Status() {
	type what int
	const (
		filesystem what = iota
		xdatabase
		timer
	)

	type done struct {
		what  what
		error bool
	}

	errors := make(chan done, 3)

	go func() {
		defer os.Remove("data/test")
		f, err := os.Create("data/test")
		if err == nil {
			err = f.Close()
		}
		if err != nil {
			log.Printf("status: file system unavailable: %s\n", err)
			errors <- done{filesystem, true}
			return
		}
		errors <- done{filesystem, false}
	}()

	go func() {
		var one int
		err := database.QueryRow("select 1").Scan(&one)
		if err != nil {
			log.Printf("status: database unavailable: %s\n", err)
			errors <- done{xdatabase, true}
			return
		}
		errors <- done{xdatabase, false}
	}()

	timeout := time.AfterFunc(time.Second*5, func() {
		log.Println("status: timeout for db or fs checks")
		errors <- done{timer, true}
	})

	statusError := func(msg string) {
		log.Println("status:", msg)
		panic(&sherpa.InternalServerError{"serverError", msg})
	}

	db := false
	fs := false
	for !db || !fs {
		done := <-errors
		if !done.error {
			switch done.what {
			case filesystem:
				fs = true
			case xdatabase:
				db = true
			default:
				serverError("status: internal error")
			}
			continue
		}

		timeout.Stop()
		switch done.what {
		case filesystem:
			statusError("filesystem unavailable")
		case xdatabase:
			statusError("database unavailable")
		case timer:
			if !db && !fs {
				statusError("timeout for both filesystem and database")
			}
			if !db {
				statusError("timeout for database")
			}
			if !fs {
				statusError("timeout for filesystem")
			}
		default:
			serverError("status: missing case")
		}
	}
	timeout.Stop()
}

func _repo(tx *sql.Tx, repoName string) (r Repo) {
	q := `select row_to_json(repo.*) from repo where name=$1`
	sherpaCheckRow(tx.QueryRow(q, repoName), &r, "fetching repo")
	return
}

func _build(tx *sql.Tx, repoName string, id int) (b Build) {
	q := `select row_to_json(bwr.*) from build_with_result bwr where id = $1`
	sherpaCheckRow(tx.QueryRow(q, id), &b, "fetching build")
	fillBuild(repoName, &b)
	return
}

// Build a specific commit in the background, returning immediately.
// `Commit` can be empty, in which case the origin is cloned and the checked out commit is looked up.
func (Ding) CreateBuild(repoName, branch, commit string) Build {
	if branch == "" {
		userError("Branch cannot be empty.")
	}

	repo, build, buildDir := _prepareBuild(repoName, branch, commit)
	go func() {
		defer func() {
			if err := recover(); err != nil {
				if serr, ok := err.(*sherpa.Error); ok {
					if serr.Code != "userError" {
						log.Println("background build failed:", serr.Message)
					}
				}
			}
		}()
		doBuild(repo, build, buildDir)
	}()
	return build
}

func toJSON(v interface{}) string {
	buf, err := json.Marshal(v)
	sherpaCheck(err, "encoding to json")
	return string(buf)
}

// Release a build.
func (Ding) CreateRelease(repoName string, buildId int) (build Build) {
	transact(func(tx *sql.Tx) {
		repo := _repo(tx, repoName)

		build = _build(tx, repo.Name, buildId)
		if build.Finish == nil {
			panic(&sherpa.Error{Code: "userError", Message: "Build has not finished yet"})
		}
		if build.Status != "success" {
			panic(&sherpa.Error{Code: "userError", Message: "Build was not successful"})
		}

		br := _buildResult(repo.Name, build)
		steps := toJSON(br.Steps)

		qrel := `insert into release (build_id, time, build_script, steps) values ($1, now(), $2, $3::json) returning build_id`
		err := tx.QueryRow(qrel, build.Id, br.BuildScript, steps).Scan(&build.Id)
		sherpaCheck(err, "inserting release into database")

		qup := `update build set released=now() where id=$1 returning id`
		err = tx.QueryRow(qup, build.Id).Scan(&build.Id)
		sherpaCheck(err, "marking build as released in database")

		var filenames []string
		q := `select coalesce(json_agg(result.filename), '[]') from result where build_id=$1`
		sherpaCheckRow(tx.QueryRow(q, build.Id), &filenames, "fetching build results from database")
		checkoutDir := fmt.Sprintf("data/build/%s/%d/checkout/%s", repo.Name, build.Id, repo.CheckoutPath)
		for _, filename := range filenames {
			fileCopy(checkoutDir+"/"+filename, fmt.Sprintf("data/release/%s/%d/%s", repo.Name, build.Id, path.Base(filename)))
		}

		events <- eventBuild{repo.Name, _build(tx, repo.Name, buildId)}
	})
	return
}

func fileCopy(src, dst string) {
	err := os.MkdirAll(path.Dir(dst), 0777)
	sherpaCheck(err, "making directory for copying result file")
	sf, err := os.Open(src)
	sherpaCheck(err, "open result file")
	defer sf.Close()
	df, err := os.Create(dst)
	sherpaCheck(err, "creating destination result file")
	defer func() {
		err2 := df.Close()
		if err == nil {
			err = err2
		}
		if err != nil {
			os.Remove(dst)
			sherpaCheck(err, "installing result file")
		}
	}()
	_, err = io.Copy(df, sf)
	sherpaCheck(err, "copying result file to destination")
}

// RepoBuilds returns all repositories and their latest build per branch (always for master, default & develop, for other branches only if the latest build was less than 4 weeks ago).
func (Ding) RepoBuilds() (rb []RepoBuilds) {
	q := `
		with repo_branch_builds as (
			select *
			from build_with_result
			where id in (
				select max(id) as id
				from build
				where branch in ('master', 'default', 'develop') or start > now() - interval '4 weeks'
				group by repo_id, branch
			)
		)
		select coalesce(json_agg(repobuilds.*), '[]')
		from (
			select row_to_json(repo.*) as repo, array_remove(array_agg(rbb.*), null) as builds
			from repo
			left join repo_branch_builds rbb on repo.id = rbb.repo_id
			group by repo.id
		) repobuilds
	`
	sherpaCheckRow(database.QueryRow(q), &rb, "fetching repobuilds")
	for _, e := range rb {
		for i, b := range e.Builds {
			fillBuild(e.Repo.Name, &b)
			e.Builds[i] = b
		}
	}
	return
}

func (Ding) Repo(repoName string) (repo Repo) {
	transact(func(tx *sql.Tx) {
		repo = _repo(tx, repoName)
	})
	return
}

// Builds returns builds for a repo.
func (Ding) Builds(repoName string) (builds []Build) {
	q := `select coalesce(json_agg(bwr.* order by start desc), '[]') from build_with_result bwr join repo on bwr.repo_id = repo.id where repo.name=$1`
	sherpaCheckRow(database.QueryRow(q, repoName), &builds, "fetching builds")
	for i, b := range builds {
		fillBuild(repoName, &b)
		builds[i] = b
	}
	return
}

func _checkRepo(repo Repo) {
	if repo.CheckoutPath == "" {
		userError("Checkout path cannot be empty.")
	}
	if strings.HasPrefix(repo.CheckoutPath, "/") || strings.HasSuffix(repo.CheckoutPath, "/") {
		userError("Checkout path cannot start or end with a slash.")
	}
}

// Create new repository.
func (Ding) CreateRepo(repo Repo) (r Repo) {
	_checkRepo(repo)

	transact(func(tx *sql.Tx) {
		q := `insert into repo (name, vcs, origin, checkout_path, build_script) values ($1, $2, $3, $4, '') returning id`
		var id int64
		sherpaCheckRow(tx.QueryRow(q, repo.Name, repo.VCS, repo.Origin, repo.CheckoutPath), &id, "inserting repository in database")
		r = _repo(tx, repo.Name)

		events <- eventRepo{r}
	})
	return
}

// Save repository.
func (Ding) SaveRepo(repo Repo) (r Repo) {
	_checkRepo(repo)

	transact(func(tx *sql.Tx) {
		q := `update repo set name=$1, vcs=$2, origin=$3, checkout_path=$4, build_script=$5 where id=$6 returning row_to_json(repo.*)`
		sherpaCheckRow(tx.QueryRow(q, repo.Name, repo.VCS, repo.Origin, repo.CheckoutPath, repo.BuildScript, repo.Id), &r, "updating repo in database")
		r = _repo(tx, repo.Name)

		events <- eventRepo{r}
	})
	return
}

// Remove repository and all its builds.
func (Ding) RemoveRepo(repoName string) {
	transact(func(tx *sql.Tx) {
		_, err := tx.Exec(`delete from result where build_id in (select id from build where repo_id in (select id from repo where name=$1))`, repoName)
		sherpaCheck(err, "removing results from database")

		_, err = tx.Exec(`delete from build where repo_id in (select id from repo where name=$1)`, repoName)
		sherpaCheck(err, "removing builds from database")

		var id int
		sherpaCheckRow(tx.QueryRow(`delete from repo where name=$1 returning id`, repoName), &id, "removing repo from database")
	})
	events <- eventRemoveRepo{repoName}

	_removeDir(repoName, -1)

	err := os.RemoveAll(fmt.Sprintf("data/release/%s", repoName))
	sherpaCheck(err, "removing release directory")
}

func parseInt(s string) int64 {
	if s == "" {
		return 0
	}
	v, err := strconv.ParseInt(s, 10, 64)
	sherpaCheck(err, "parsing integer")
	return v
}

func _buildResult(repoName string, build Build) (br BuildResult) {
	buildDir := fmt.Sprintf("data/build/%s/%d/", repoName, build.Id)
	br.BuildScript = readFile(buildDir + "scripts/build.sh")
	br.Steps = []Step{}

	if build.Status == "new" {
		return
	}

	outputDir := buildDir + "output/"
	for _, stepName := range stepNames {
		br.Steps = append(br.Steps, Step{
			Name:   stepName,
			Stdout: readFileLax(outputDir + stepName + ".stdout"),
			Stderr: readFileLax(outputDir + stepName + ".stderr"),
			Output: readFileLax(outputDir + stepName + ".output"),
			Nsec:   parseInt(readFileLax(outputDir + stepName + ".nsec")),
		})
		if stepName == build.Status {
			break
		}
	}
	return
}

// Get build result.
func (Ding) BuildResult(repoName string, buildId int) (br BuildResult) {
	var build Build
	transact(func(tx *sql.Tx) {
		build = _build(tx, repoName, buildId)
	})
	br = _buildResult(repoName, build)
	br.Build = build
	return
}

// Fetch build config and results for a release.
func (Ding) Release(repoName string, buildId int) (br BuildResult) {
	transact(func(tx *sql.Tx) {
		build := _build(tx, repoName, buildId)

		q := `select row_to_json(release.*) from release where build_id=$1`
		sherpaCheckRow(tx.QueryRow(q, buildId), &br, "fetching release from database")
		br.Build = build
	})
	return
}

// Remove build completely. Both from database and all local files.
func (Ding) RemoveBuild(buildId int) {
	var repoName string
	transact(func(tx *sql.Tx) {
		qrepo := `select to_json(repo.name) from build join repo on build.repo_id = repo.id where build.id = $1`
		sherpaCheckRow(tx.QueryRow(qrepo, buildId), &repoName, "fetching repo name from database")

		build := _build(tx, repoName, buildId)
		if build.Released != nil {
			panic(&sherpa.Error{Code: "userError", Message: "Build has been released, cannot be removed"})
		}

		_removeBuild(tx, repoName, buildId)
	})
	events <- eventRemoveBuild{repoName, buildId}
}

// Clean up (remove) the build dir.  This does not remove the build itself from the database.
func (Ding) CleanupBuilddir(repoName string, buildId int) (build Build) {
	transact(func(tx *sql.Tx) {
		build = _build(tx, repoName, buildId)
		if build.BuilddirRemoved {
			panic(&sherpa.Error{Code: "userError", Message: "Builddir already removed"})
		}

		_removeBuilddir(tx, repoName, buildId)
		build = _build(tx, repoName, buildId)
		fillBuild(repoName, &build)
	})
	events <- eventBuild{repoName, build}
	return
}
