package main

import (
	"bufio"
	"bytes"
	"database/sql"
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
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
			if serr, ok := r.(*sherpa.Error); !ok || serr.Code != "userError" {
				panic(r)
			}
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

	runPrefix := func(args ...string) []string {
		if len(config.Run) > 0 {
			args = append(config.Run, args...)
		}
		return args
	}

	_updateStatus("clone")
	var err error
	switch repo.VCS {
	case "git":
		// we clone without hard links because we chown later, don't want to mess up local git source repo's
		// we have to clone as the user running ding. otherwise, git clone won't work due to ssh refusing to run as a user without a username ("No user exists for uid ...")
		err = run(build.Id, env, "clone", buildDir, buildDir, runPrefix("git", "clone", "--recursive", "--no-hardlinks", "--branch", build.Branch, repo.Origin, "checkout/"+repo.CheckoutPath)...)
		sherpaUserCheck(err, "cloning git repository")
	case "mercurial":
		cmd := []string{"hg", "clone", "--branch", build.Branch}
		if build.CommitHash != "" {
			cmd = append(cmd, "--rev", build.CommitHash, "--updaterev", build.CommitHash)
		}
		cmd = append(cmd, repo.Origin, "checkout/"+repo.CheckoutPath)
		err = run(build.Id, env, "clone", buildDir, buildDir, runPrefix(cmd...)...)
		sherpaUserCheck(err, "cloning mercurial repository")
	case "command":
		err = run(build.Id, env, "clone", buildDir, buildDir, runPrefix("sh", "-c", repo.Origin)...)
		sherpaUserCheck(err, "cloning repository from command")
	default:
		serverError("unexpected VCS " + repo.VCS)
	}

	checkoutDir := fmt.Sprintf("%s/checkout/%s", buildDir, repo.CheckoutPath)

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
			cmd := execCommand(runPrefix(command...)...)
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
		err = run(build.Id, env, "clone", buildDir, checkoutDir, runPrefix("git", "checkout", build.CommitHash)...)
		sherpaUserCheck(err, "checkout revision")
	}

	req := request{
		msg{MsgChown, repo.Name, build.Id, repo.CheckoutPath, nil},
		make(chan error, 0),
		nil,
	}
	rootRequests <- req
	err = <-req.errorResponse
	sherpaCheck(err, "chown")

	_updateStatus("build")
	req = request{
		msg{MsgBuild, repo.Name, build.Id, repo.CheckoutPath, env},
		nil,
		make(chan buildResult, 0),
	}
	rootRequests <- req
	result := <-req.buildResponse
	if result.err != nil {
		sherpaUserCheck(result.err, "building")
	}

	wait := make(chan error, 1)
	go func() {
		defer result.status.Close()

		var r string
		err = gob.NewDecoder(result.status).Decode(&r)
		check(err, "decoding gob from result.status")
		var err error
		if r != "" {
			err = fmt.Errorf("%s", r)
		}
		wait <- err
	}()
	err = track(build.Id, "build", buildDir, result.stdout, result.stderr, wait)
	sherpaUserCheck(err, "running command")

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

// start a command and return readers for its output and the final result of the command.
// it mimics a command started through the root process under a unique uid.
func setupCmd(buildId int, env []string, step, buildDir, workDir string, args ...string) (stdout, stderr io.ReadCloser, wait <-chan error, rerr error) {
	type Error struct {
		err error
	}

	var devnull, stdoutr, stdoutw, stderrr, stderrw *os.File
	defer func() {
		close := func(f *os.File) {
			if f != nil {
				f.Close()
			}
		}
		// always close subprocess-part of the fd's
		close(devnull)
		close(stdoutw)
		close(stderrw)

		e := recover()
		if e == nil {
			return
		}

		if ee, ok := e.(Error); ok {
			// only close returning fd's on error
			close(stdoutr)
			close(stderrr)

			rerr = ee.err
			return
		}
		panic(e)
	}()

	xcheck := func(err error, msg string) {
		if err != nil {
			panic(Error{fmt.Errorf("%s: %s", msg, err)})
		}
	}

	var err error
	devnull, err = os.Open("/dev/null")
	xcheck(err, "open /dev/null")

	stdoutr, stdoutw, err = os.Pipe()
	xcheck(err, "pipe for stdout")

	stderrr, stderrw, err = os.Pipe()
	xcheck(err, "pipe for stderr")

	attr := &os.ProcAttr{
		Dir: workDir,
		Env: env,
		Files: []*os.File{
			devnull,
			stdoutw,
			stderrw,
		},
	}
	proc, err := os.StartProcess(args[0], args, attr)
	xcheck(err, "command start")

	c := make(chan error, 1)
	go func() {
		state, err := proc.Wait()
		if err == nil && !state.Success() {
			err = fmt.Errorf(state.String())
		}
		c <- err
	}()
	return stdoutr, stderrr, c, nil
}

func run(buildId int, env []string, step, buildDir, workDir string, args ...string) error {
	cmdstdout, cmdstderr, wait, err := setupCmd(buildId, env, step, buildDir, workDir, args...)
	if err != nil {
		return fmt.Errorf("setting up command: %s", err)
	}
	return track(buildId, step, buildDir, cmdstdout, cmdstderr, wait)
}

func track(buildId int, step, buildDir string, cmdstdout, cmdstderr io.ReadCloser, wait <-chan error) (rerr error) {
	type Error struct {
		err error
	}

	defer func() {
		e := recover()
		if e == nil {
			return
		}
		if ee, ok := e.(Error); ok {
			rerr = ee.err
			return
		}
		panic(e)
	}()

	xcheck := func(err error, msg string) {
		if err != nil {
			panic(Error{fmt.Errorf("%s: %s", msg, err)})
		}
	}

	defer func() {
		cmdstdout.Close()
		cmdstderr.Close()
	}()

	// write .nsec file when we're done here
	t0 := time.Now()
	defer func() {
		time.Now().Sub(t0)
		nsec, err := os.Create(buildDir + "/output/" + step + ".nsec")
		xcheck(err, "creating nsec file")
		defer nsec.Close()
		_, err = fmt.Fprintf(nsec, "%d", time.Now().Sub(t0))
		xcheck(err, "writing nsec file")
	}()

	appendFlags := os.O_APPEND | os.O_CREATE | os.O_WRONLY
	output, err := os.OpenFile(buildDir+"/output/"+step+".output", appendFlags, 0644)
	xcheck(err, "creating output file")
	defer output.Close()
	stdout, err := os.OpenFile(buildDir+"/output/"+step+".stdout", appendFlags, 0644)
	xcheck(err, "creating stdout file")
	defer stdout.Close()
	stderr, err := os.OpenFile(buildDir+"/output/"+step+".stderr", appendFlags, 0644)
	xcheck(err, "creating stderr file")
	defer stderr.Close()

	// let it be known that we started this phase
	events <- eventOutput{buildId, step, "stdout", ""}

	// first we read all the data from stdout & stderr
	type Lines struct {
		text   string
		stdout bool
		err    error
	}
	lines := make(chan Lines, 0)
	linereader := func(r io.ReadCloser, stdout bool) {
		buf := make([]byte, 1024)
		have := 0
		for {
			//log.Println("calling read")
			n, err := r.Read(buf[have:])
			//log.Println("read returned")
			if n > 0 {
				have += n
				end := bytes.LastIndexByte(buf[:have], '\n')
				if end < 0 && have == len(buf) {
					// cannot gather any more data, flush it
					end = len(buf)
				} else if end < 0 {
					continue
				} else {
					// include the newline
					end += 1
				}
				lines <- Lines{string(buf[:end]), stdout, nil}
				copy(buf[:], buf[end:have])
				have -= end
			}
			if err == io.EOF {
				lines <- Lines{"", stdout, nil}
				break
			}
			if err != nil {
				lines <- Lines{stdout: stdout, err: err}
				return
			}
		}
	}
	//log.Println("new command, reading input")
	go linereader(cmdstdout, true)
	go linereader(cmdstderr, false)
	eofs := 0
	for {
		l := <-lines
		//log.Println("have line", l)
		if l.text == "" || l.err != nil {
			if l.err != nil {
				log.Println("reading output from command:", l.err)
			}
			eofs += 1
			if eofs >= 2 {
				//log.Println("done with command output")
				break
			}
			continue
		}
		_, err = output.Write([]byte(l.text))
		xcheck(err, "writing to output")
		var where string
		if l.stdout {
			where = "stdout"
			_, err = stdout.Write([]byte(l.text))
			xcheck(err, "writing to stdout")
		} else {
			where = "stderr"
			_, err = stderr.Write([]byte(l.text))
			xcheck(err, "writing to stderr")
		}
		events <- eventOutput{buildId, step, where, l.text}
	}

	// second, we wait for the command result
	xcheck(<-wait, "command failed")
	return
}
