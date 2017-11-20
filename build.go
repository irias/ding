package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"bitbucket.org/mjl/sherpa"
)

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
	events <- eventBuild{repo.Name, build}
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

func prepareBuild(repoName, branch, commit string) (repo Repo, build Build, buildDir string, err error) {
	if branch == "" {
		err = fmt.Errorf("Branch cannot be empty.")
		return
	}
	defer func() {
		xerr := recover()
		if xerr == nil {
			return
		}
		if serr, ok := xerr.(*sherpa.Error); ok {
			err = fmt.Errorf("%s", serr.Error())
		}
	}()
	repo, build, buildDir = _prepareBuild(repoName, branch, commit)
	return repo, build, buildDir, nil
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
		transact(func(tx *sql.Tx) {
			_, err := tx.Exec("update build set finish=NOW() where id=$1 and finish is null", build.Id)
			sherpaCheck(err, "marking build as finished in database")
			events <- eventBuild{repo.Name, _build(tx, repo.Name, build.Id)}
		})

		_cleanupBuilds(repo.Name, build.Branch)

		r := recover()
		if r != nil {
			if serr, ok := r.(*sherpa.Error); ok && serr.Code == "userError" {
				transact(func(tx *sql.Tx) {
					err := tx.QueryRow(`update build set error_message=$1 where id=$2 returning id`, serr.Message, build.Id).Scan(&build.Id)
					sherpaCheck(err, "updating error message in database")
					events <- eventBuild{repo.Name, _build(tx, repo.Name, build.Id)}
				})
			}
		}

		var prevStatus string
		err := database.QueryRow("select status from build join repo on build.repo_id = repo.id and repo.name = $1 and build.branch = $2 order by build.id desc offset 1 limit 1", repo.Name, build.Branch).Scan(&prevStatus)
		if r != nil && (err != nil || prevStatus == "success") {

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
			_sendMailFailing(repo, build, errmsg)
		}
		if r == nil && err == nil && prevStatus != "success" {
			_sendMailFixed(repo, build)
		}

		if r != nil {
			panic(r)
		}
	}()

	_updateStatus := func(status string) {
		transact(func(tx *sql.Tx) {
			_, err := tx.Exec("update build set status=$1 where id=$2", status, build.Id)
			sherpaCheck(err, "updating build status in database")
			events <- eventBuild{repo.Name, _build(tx, repo.Name, build.Id)}
		})
	}

	env := []string{
		"BUILDDIR=" + buildDir,
		"CHECKOUTPATH=" + repo.CheckoutPath,
		fmt.Sprintf("HOME=%s/home", buildDir),
		fmt.Sprintf("BUILDID=%d", build.Id),
		"REPONAME=" + repo.Name,
		"BRANCH=" + build.Branch,
		"COMMIT=" + build.CommitHash,
	}
	for key, value := range config.Environment {
		env = append(env, key+"="+value)
	}

	execCommand := func(args ...string) *exec.Cmd {
		return exec.Command(args[0], args[1:]...)
	}

	_updateStatus("clone")
	var err error
	switch repo.VCS {
	case "git":
		// we clone without hard links because we chown later, don't want to mess up local git source repo's
		// we have to clone as the user running ding. otherwise, git clone won't work due to ssh refusing to run as a user without a username ("No user exists for uid ...")
		err = run(build.Id, env, "clone", buildDir, buildDir, "git", "clone", "--recursive", "--no-hardlinks", "--branch", build.Branch, repo.Origin, "checkout/"+repo.CheckoutPath)
		sherpaUserCheck(err, "cloning git repository")
	case "mercurial":
		cmd := []string{"hg", "clone", "--branch", build.Branch}
		if build.CommitHash != "" {
			cmd = append(cmd, "--rev", build.CommitHash, "--updaterev", build.CommitHash)
		}
		cmd = append(cmd, repo.Origin, "checkout/"+repo.CheckoutPath)
		err = run(build.Id, env, "clone", buildDir, buildDir, cmd...)
		sherpaUserCheck(err, "cloning mercurial repository")
	case "command":
		err = run(build.Id, env, "clone", buildDir, buildDir, "sh", "-c", repo.Origin)
		sherpaUserCheck(err, "cloning repository from command")
	default:
		serverError("unexpected VCS " + repo.VCS)
	}

	checkoutDir := fmt.Sprintf("%s/checkout/%s", buildDir, repo.CheckoutPath)
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
		if len(config.Run) > 0 {
			args = append(config.Run, args...)
		}
		if config.IsolateBuilds.Enabled {
			args = append(RUNAS, args...)
		}
		return args
	}

	if build.CommitHash == "" {
		if repo.VCS == "command" {
			clone := readFile(buildDir + "/output/clone.stdout")
			clone = strings.TrimSpace(clone)
			l := strings.Split(clone, "\n")
			s := l[len(l)-1]
			if !strings.HasPrefix(s, "commit:") {
				userError(`output of clone command should start with "commit:" followed by the commit id/hash`)
			}
			build.CommitHash = s[len("commit:"):]
		} else {
			var command []string
			switch repo.VCS {
			case "git":
				command = []string{"git", "rev-parse", "HEAD"}
			case "mercurial":
				command = []string{"hg", "id", "--id"}
			default:
				serverError("unexpected VCS " + repo.VCS)
			}
			cmd := execCommand(runas(command...)...)
			cmd.Dir = checkoutDir
			buf, err := cmd.Output()
			sherpaCheck(err, "finding commit hash")
			build.CommitHash = strings.TrimSpace(string(buf))
		}
		if build.CommitHash == "" {
			sherpaCheck(fmt.Errorf("cannot find commit hash"), "finding commit hash")
		}
		transact(func(tx *sql.Tx) {
			err = tx.QueryRow(`update build set commit_hash=$1 where id=$2 returning id`, build.CommitHash, build.Id).Scan(&build.Id)
			sherpaCheck(err, "updating commit hash in database")
			events <- eventBuild{repo.Name, _build(tx, repo.Name, build.Id)}
		})
	}

	if repo.VCS == "git" {
		_updateStatus("checkout")
		err = run(build.Id, env, "checkout", buildDir, checkoutDir, runas("git", "checkout", build.CommitHash)...)
		sherpaUserCheck(err, "checkout revision")
	}

	_updateStatus("build")
	err = run(build.Id, env, "build", buildDir, checkoutDir, runas(buildDir+"/scripts/build.sh")...)
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

		events <- eventBuild{repo.Name, _build(tx, repo.Name, build.Id)}
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
			events <- eventRemoveBuild{repoName, b.Id}
		}
	}
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

func run(buildId int, env []string, step, buildDir, workDir string, args ...string) (err error) {
	events <- eventOutput{buildId, step, "stdout", ""}

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
	if output, err = os.Create(buildDir + "/output/" + step + ".output"); err != nil {
		return fmt.Errorf("creating combined output file: %s", err)
	}
	output = LineWriter(output, "", "", -1)
	if stdout, err = os.Create(buildDir + "/output/" + step + ".stdout"); err != nil {
		return fmt.Errorf("creating stdout file: %s", err)
	}
	stdout = LineWriter(stdout, step, "stdout", buildId)
	cmd.Stdout = io.MultiWriter(stdout, output)

	if stderr, err = os.Create(buildDir + "/output/" + step + ".stderr"); err != nil {
		return fmt.Errorf("opening stderr file: %s", err)
	}
	stderr = LineWriter(stderr, step, "stderr", buildId)
	cmd.Stderr = io.MultiWriter(stderr, output)

	if nsecFile, err = os.Create(buildDir + "/output/" + step + ".nsec"); err != nil {
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
