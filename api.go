package main

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path"
	"strconv"
	"strings"
	"time"

	"bitbucket.org/mjl/sherpa"
)

var (
	stepNames = []string{
		"clone",
		"checkout",
		"build",
		"success",
	}
)

// The Ding API lets you compile git branches, build binaries, run tests, and publish binaries.
type Ding struct {
}

// Checks program status.
// If backend connectivity is broken, this sherpa call results in a 500 internal server error. Useful for monitoring tools.
func (Ding) Status() {
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

func _prepareBuild(repoName, branch, commit string) (repo Repo, build Build, buildDir string) {
	transact(func(tx *sql.Tx) {
		repo = _repo(tx, repoName)

		q := `insert into build (repo_id, branch, commit_hash, status, start) values ($1, $2, $3, $4, NOW()) returning id`
		sherpaCheckRow(tx.QueryRow(q, repo.Id, branch, commit, "new"), &build.Id, "inserting new build into database")

		buildDir = fmt.Sprintf("%s/data/build/%s/%d", dingWorkDir, repo.Name, build.Id)
		err := os.MkdirAll(buildDir, 0777)
		sherpaCheck(err, "creating build dir")

		err = os.MkdirAll(buildDir+"/scripts", 0777)
		sherpaCheck(err, "creating scripts dir")
		err = os.MkdirAll(buildDir+"/home", 0777)
		sherpaCheck(err, "creating home dir")

		buildSh := buildDir + "/scripts/build.sh"
		writeFile(buildSh, repo.BuildScript)
		err = os.Chmod(buildSh, os.FileMode(0755))
		sherpaCheck(err, "chmod")

		outputDir := buildDir + "/output"
		err = os.MkdirAll(outputDir, 0777)
		sherpaCheck(err, "creating output dir")

		build = _build(tx, repo.Name, build.Id)
	})
	return
}

// Build a specific commit in the background, returning immediately.
// `Branch` can be empty, in which case the actual branch is determined after checkout of `commit`. `Commit` can also be empty, in which case a clone is done and the checked out commit is looked up.
func (Ding) CreateBuild(repoName, branch, commit string) Build {
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

func calcUid(buildId int) int {
	return config.IsolateBuilds.UidStart + buildId%(config.IsolateBuilds.UidEnd-config.IsolateBuilds.UidStart)
}

func doBuild(repo Repo, build Build, buildDir string) {
	job := job{
		repo.Name,
		make(chan struct{}),
	}
	newJobs <- job
	<-job.rc
	defer func() {
		finishedJobs <- job.repoName
	}()
	_doBuild(repo, build, buildDir)
}

func _doBuild(repo Repo, build Build, buildDir string) {
	defer func() {
		_, err := database.Exec("update build set finish=NOW() where id=$1 and finish is null", build.Id)
		sherpaCheck(err, "marking build as finished in database")

		if build.Branch != "" {
			_cleanupBuilds(repo.Name, build.Branch)
		}

		r := recover()
		if r != nil {
			if serr, ok := r.(*sherpa.Error); ok && serr.Code == "userError" {
				err = database.QueryRow(`update build set error_message=$1 where id=$2 returning id`, serr.Message, build.Id).Scan(&build.Id)
				sherpaCheck(err, "updating error message in database")
			}
		}

		var prevStatus string
		err = database.QueryRow("select status from build join repo on build.repo_id = repo.id and repo.name = $1 and build.branch = $2 order by build.id desc offset 1 limit 1", repo.Name, build.Branch).Scan(&prevStatus)
		if r != nil && (err != nil || prevStatus == "success") {
			link := fmt.Sprintf("%s/#/repo/%s/build/%d/", config.BaseURL, repo.Name, build.Id)

			// for build.LastLine
			transact(func(tx *sql.Tx) {
				build = _build(tx, repo.Name, build.Id)
			})
			fillBuild(repo.Name, &build)

			var errmsg string
			if serr, ok := r.(*sherpa.Error); ok {
				errmsg = serr.Message
			} else {
				errmsg = fmt.Sprintf("%v", r)
			}
			subject := fmt.Sprintf("ding: failure: repo %s branch %s failing", repo.Name, build.Branch)
			textMsg := fmt.Sprintf(`Hi!

Your build for branch %s on repo %s is now failing:

	%s

Last output:

	%s
	%s

Please fix, thanks!

Cheers,
Ding
`, build.Branch, repo.Name, link, build.LastLine, errmsg)
			_sendmail(config.Notify.Name, config.Notify.Email, subject, textMsg)
		}
		if r == nil && err == nil && prevStatus != "success" {
			link := fmt.Sprintf("%s/#/repo/%s/build/%d/", config.BaseURL, repo.Name, build.Id)
			subject := fmt.Sprintf("ding: resolved: repo %s branch %s is building again", repo.Name, build.Branch)
			textMsg := fmt.Sprintf(`Hi!

You fixed the build for branch %s on repo %s:

	%s

You're the bomb, keep it up!

Cheers,
Ding
`, build.Branch, repo.Name, link)
			_sendmail(config.Notify.Name, config.Notify.Email, subject, textMsg)
		}

		if r != nil {
			panic(r)
		}
	}()

	_updateStatus := func(status string) {
		_, err := database.Exec("update build set status=$1 where id=$2", status, build.Id)
		sherpaCheck(err, "updating build status in database")
	}

	env := []string{
		fmt.Sprintf("HOME=%s/home", buildDir),
		fmt.Sprintf("BUILDID=%d", build.Id),
		"REPONAME=" + repo.Name,
		"BRANCH=" + build.Branch,
		"COMMITHASH=" + build.CommitHash,
	}
	for key, value := range config.Environment {
		env = append(env, key+"="+value)
	}

	execCommand := func(args ...string) *exec.Cmd {
		return exec.Command(args[0], args[1:]...)
	}

	_updateStatus("clone")
	var err error
	// we clone without hard links because we chown later, don't want to mess up local git source repo's
	// we have to clone as the user running ding. otherwise, git clone won't work due to ssh refusing to run as a user without a username ("No user exists for uid ...")
	if build.Branch == "" {
		err = run(env, "clone", buildDir, buildDir, "git", "clone", "--no-hardlinks", repo.Origin, "checkout/"+repo.Name)
	} else {
		err = run(env, "clone", buildDir, buildDir, "git", "clone", "--no-hardlinks", "--branch", build.Branch, repo.Origin, "checkout/"+repo.Name)
	}
	sherpaUserCheck(err, "cloning repository")
	checkoutDir := fmt.Sprintf("%s/checkout/%s", buildDir, repo.Name)
	if config.IsolateBuilds.Enabled {
		chownBuild := append(config.IsolateBuilds.ChownBuild, fmt.Sprintf("%d", calcUid(build.Id)), fmt.Sprintf("%d", config.IsolateBuilds.DingGid), buildDir+"/checkout", buildDir+"/home")
		cmd := execCommand(chownBuild...)
		cmd.Dir = buildDir
		output, err := cmd.CombinedOutput()
		if err != nil {
			sherpaCheck(err, "setting owner/group on checkout and home directory: "+strings.TrimSpace(string(output)))
		}
	}

	// from now on, we run commands under its own uid, if config.IsolateBuilds is on.
	RUNAS := append(config.IsolateBuilds.Runas, fmt.Sprintf("%d", calcUid(build.Id)), fmt.Sprintf("%d", config.IsolateBuilds.DingGid))
	runas := func(args ...string) []string {
		if config.IsolateBuilds.Enabled {
			return append(RUNAS, args...)
		}
		return args
	}

	if build.CommitHash == "" {
		cmd := execCommand(runas("git", "rev-parse", "HEAD")...)
		cmd.Dir = checkoutDir
		buf, err := cmd.Output()
		sherpaCheck(err, "finding commit hash")
		build.CommitHash = strings.TrimSpace(string(buf))
		if build.CommitHash == "" {
			sherpaCheck(fmt.Errorf("cannot find commit hash"), "finding commit hash")
		}
		err = database.QueryRow(`update build set commit_hash=$1 where id=$2 returning id`, build.CommitHash, build.Id).Scan(&build.Id)
		sherpaCheck(err, "updating commit hash in database")
	}

	_updateStatus("checkout")
	err = run(env, "checkout", buildDir, checkoutDir, runas("git", "checkout", build.CommitHash)...)
	sherpaUserCheck(err, "checkout revision")

	if build.Branch == "" {
		cmd := execCommand(runas("sh", "-c", `git branch | sed 's/^..//' | grep -v "(HEAD detached at" | head -n1`)...)
		cmd.Dir = checkoutDir
		buf, err := cmd.Output()
		sherpaCheck(err, "determining branch for commit")
		build.Branch = strings.TrimSpace(string(buf))
		if build.Branch == "" {
			sherpaCheck(fmt.Errorf("cannot determine branch for checkout"), "finding branch")
		}
		err = database.QueryRow(`update build set branch=$1 where id=$2 returning id`, build.Branch, build.Id).Scan(&build.Id)
		sherpaCheck(err, "updating branch in database")
	}

	_updateStatus("build")
	err = run(env, "build", buildDir, checkoutDir, runas("../../scripts/build.sh")...)
	sherpaUserCheck(err, "building")

	transact(func(tx *sql.Tx) {
		outputDir := buildDir + "/output"
		results := parseResults(checkoutDir, outputDir+"/build.stdout")

		qins := `insert into result (build_id, command, version, os, arch, toolchain, filename, filesize) values ($1, $2, $3, $4, $5, $6, $7, $8) returning id`
		for _, result := range results {
			var id int
			err = tx.QueryRow(qins, build.Id, result.Command, result.Version, result.Os, result.Arch, result.Toolchain, result.Filename, result.Filesize).Scan(&id)
			sherpaCheck(err, "inserting result into database")
		}

		_, err = tx.Exec("update build set status='success', finish=NOW() where id=$1", build.Id)
		sherpaCheck(err, "marking build as success in database")
	})
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

func parseResults(checkoutDir, path string) (results []Result) {
	f, err := os.Open(path)
	sherpaUserCheck(err, "opening build output")
	defer func() {
		sherpaUserCheck(f.Close(), "closing build output")
	}()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		// lines should be of the form:
		//  "release:" command version os arch toolchain path
		line := scanner.Text()
		t := strings.Split(line, " ")
		if t[0] != "release:" {
			continue
		}
		if len(t) != 7 {
			sherpaUserCheck(err, "invalid output line, should have 7 words: "+line)
		}
		result := Result{t[1], t[2], t[3], t[4], t[5], t[6], 0}
		if !strings.HasPrefix(result.Filename, "/") {
			result.Filename = checkoutDir + "/" + result.Filename
		}
		info, err := os.Stat(result.Filename)
		sherpaUserCheck(err, "testing whether released file exists")
		result.Filename = result.Filename[len(checkoutDir+"/"):]
		result.Filesize = info.Size()
		results = append(results, result)
	}
	err = scanner.Err()
	sherpaUserCheck(err, "reading build output")
	return
}

func run(env []string, stage, buildDir, workDir string, args ...string) (err error) {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = workDir
	cmd.Env = env
	var output, stdout, stderr, nsecFile io.WriteCloser
	t0 := time.Now()
	defer func() {
		check := func(err2 error) {
			if err == nil {
				err = err2
			}
		}
		if output != nil {
			check(output.Close())
		}
		if stdout != nil {
			check(stdout.Close())
		}
		if stderr != nil {
			check(stderr.Close())
		}

		if nsecFile != nil {
			_, err2 := fmt.Fprintf(nsecFile, "%d", time.Now().Sub(t0))
			check(err2)
		}
	}()
	if output, err = os.Create(buildDir + "/output/" + stage + ".output"); err != nil {
		return fmt.Errorf("creating combined output file: %s", err)
	}
	output = LineWriter(output)
	if stdout, err = os.Create(buildDir + "/output/" + stage + ".stdout"); err != nil {
		return fmt.Errorf("creating stdout file: %s", err)
	}
	stdout = LineWriter(stdout)
	cmd.Stdout = io.MultiWriter(stdout, output)

	if stderr, err = os.Create(buildDir + "/output/" + stage + ".stderr"); err != nil {
		return fmt.Errorf("opening stderr file: %s", err)
	}
	stderr = LineWriter(stderr)
	cmd.Stderr = io.MultiWriter(stderr, output)

	if nsecFile, err = os.Create(buildDir + "/output/" + stage + ".nsec"); err != nil {
		return fmt.Errorf("opening nsec file: %s", err)
	}

	if err = cmd.Run(); err != nil {
		return fmt.Errorf("workdir %s, command %s: %s", workDir, strings.Join(args, " "), err)
	}
	if err = output.Close(); err != nil {
		return err
	}
	output = nil
	if err = stdout.Close(); err != nil {
		return err
	}
	stdout = nil
	if err = stderr.Close(); err != nil {
		return err
	}
	stderr = nil
	return nil
}

func toJSON(v interface{}) string {
	buf, err := json.Marshal(v)
	sherpaCheck(err, "encoding to json")
	return string(buf)
}

// Release a build.
func (Ding) CreateRelease(repoName string, buildId int) (build Build) {
	transact(func(tx *sql.Tx) {
		build = _build(tx, repoName, buildId)
		if build.Finish == nil {
			panic(&sherpa.Error{Code: "userError", Message: "Build has not finished yet"})
		}
		if build.Status != "success" {
			panic(&sherpa.Error{Code: "userError", Message: "Build was not successful"})
		}

		br := _buildResult(repoName, build)
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
		checkoutDir := fmt.Sprintf("data/build/%s/%d/checkout/%s", repoName, build.Id, repoName)
		for _, filename := range filenames {
			fileCopy(checkoutDir+"/"+filename, fmt.Sprintf("data/release/%s/%d/%s", repoName, build.Id, path.Base(filename)))
		}
	})
	return
}

// RepoBuilds returns all repositories and their latest build per branch (always for master & develop, for other branches only if the latest build was less than 4 weeks ago).
func (Ding) RepoBuilds() (rb []RepoBuilds) {
	q := `
		with repo_branch_builds as (
			select *
			from build_with_result
			where id in (
				select max(id) as id
				from build
				where branch in ('master', 'develop') or start > now() - interval '4 weeks'
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

// Builds returns builds for a repo
func (Ding) Builds(repoName string) (builds []Build) {
	q := `select coalesce(json_agg(bwr.* order by start desc), '[]') from build_with_result bwr join repo on bwr.repo_id = repo.id where repo.name=$1`
	sherpaCheckRow(database.QueryRow(q, repoName), &builds, "fetching builds")
	for i, b := range builds {
		fillBuild(repoName, &b)
		builds[i] = b
	}
	return
}

func writeFile(path, content string) {
	f, err := os.Create(path)
	sherpaCheck(err, "creating file")
	_, err = f.Write([]byte(content))
	err2 := f.Close()
	if err == nil {
		err = err2
	}
	sherpaCheck(err, "writing file")
}

// Create new repository.
func (Ding) CreateRepo(repo Repo) (r Repo) {
	transact(func(tx *sql.Tx) {
		q := `insert into repo (name, origin, build_script) values ($1, $2, '') returning id`
		var id int64
		sherpaCheckRow(tx.QueryRow(q, repo.Name, repo.Origin), &id, "inserting repository in database")
		r = _repo(tx, repo.Name)
	})
	return
}

// Save repository.
func (Ding) SaveRepo(repo Repo) (r Repo) {
	transact(func(tx *sql.Tx) {
		q := `update repo set name=$1, origin=$2, build_script=$3 where id=$4 returning row_to_json(repo.*)`
		sherpaCheckRow(tx.QueryRow(q, repo.Name, repo.Origin, repo.BuildScript, repo.Id), &r, "updating repo in database")
		r = _repo(tx, repo.Name)
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

	_removeDir(dingWorkDir + "/data/build/" + repoName)

	err := os.RemoveAll(fmt.Sprintf("data/release/%s", repoName))
	sherpaCheck(err, "removing release directory")
}

func readFile(path string) string {
	f, err := os.Open(path)
	sherpaCheck(err, "opening script")
	buf, err := ioutil.ReadAll(f)
	err2 := f.Close()
	if err == nil {
		err = err2
	}
	sherpaCheck(err, "reading script")
	return string(buf)
}

func readFileLax(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	buf, err := ioutil.ReadAll(f)
	f.Close()
	if err != nil {
		return ""
	}
	return string(buf)
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

	outputDir := buildDir + "output/"
	for _, stepName := range stepNames {
		var step Step
		if stepName == "success" {
			step.Name = "success"
		} else {
			step = Step{
				Name:   stepName,
				Stdout: readFileLax(outputDir + stepName + ".stdout"),
				Stderr: readFileLax(outputDir + stepName + ".stderr"),
				Output: readFileLax(outputDir + stepName + ".output"),
				Nsec:   parseInt(readFileLax(outputDir + stepName + ".nsec")),
			}
		}
		br.Steps = append(br.Steps, step)
		if stepName == br.Build.Status {
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
}

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
	return
}

func _removeBuilddir(tx *sql.Tx, repoName string, buildId int) {
	err := tx.QueryRow("update build set builddir_removed=true where id=$1 returning id", buildId).Scan(&buildId)
	sherpaCheck(err, "marking builddir as removed in database")

	buildDir := fmt.Sprintf("%s/data/build/%s/%d", dingWorkDir, repoName, buildId)
	_removeDir(buildDir)
}

func _cleanupBuilds(repoName, branch string) {
	var builds []Build
	q := `
		select coalesce(json_agg(x.* order by x.id desc), '[]')
		from (
			select build.*
			from build join repo on build.repo_id = repo.id
			where repo.name=$1 and build.branch=$2
		) x
	`
	sherpaCheckRow(database.QueryRow(q, repoName, branch), &builds, "fetching builds from database")
	now := time.Now()
	for index, b := range builds {
		if index == 0 || b.Released != nil {
			continue
		}
		if index >= 10 || (b.Finish != nil && now.Sub(*b.Finish) > 14*24*3600*time.Second) {
			transact(func(tx *sql.Tx) {
				_removeBuild(tx, repoName, b.Id)
			})
		}
	}
}
