package main

import (
	"encoding/gob"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/unix"
)

var (
	dingWorkDir          string
	serveFlag            = flag.NewFlagSet("serve", flag.ExitOnError)
	listenAddress        = serveFlag.String("listen", ":6084", "address to listen on")
	listenWebhookAddress = serveFlag.String("listenwebhook", ":6085", "address to listen on for webhooks, like from github; set empty for no listening")

	rootRequests chan request // for http-serve
)

func serve(args []string) {
	log.SetFlags(0)
	log.SetPrefix("serve: ")
	serveFlag.Init("serve", flag.ExitOnError)
	serveFlag.Usage = func() {
		fmt.Println("usage: ding [flags] serve config.json")
		serveFlag.PrintDefaults()
	}
	serveFlag.Parse(args)
	args = serveFlag.Args()
	if len(args) != 1 {
		serveFlag.Usage()
		os.Exit(2)
	}

	parseConfig(args[0])

	var err error
	dingWorkDir, err = os.Getwd()
	check(err, "getting current work dir")

	if config.IsolateBuilds.Enabled && os.Getuid() != 0 {
		log.Fatalln(`must run as root when isolateBuilds is enabled`)
	} else if !config.IsolateBuilds.Enabled && os.Getuid() == 0 {
		log.Fatalln(`mjust not run as root when isolateBuilds is disabled`)
	}

	proto := 0
	// we exchange gob messages with unprivileged httpserver over socketsA
	socketsA, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM, proto)
	check(err, "creating socketpair")

	// and we send file descriptors from to unprivileged httpserver after kicking off a build under a unique uid
	socketsB, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM, proto)
	check(err, "creating socketpair")

	rootAFD := os.NewFile(uintptr(socketsA[0]), "rootA")
	httpAFD := os.NewFile(uintptr(socketsA[1]), "httpA")
	rootBFD := os.NewFile(uintptr(socketsB[0]), "rootB")
	httpBFD := os.NewFile(uintptr(socketsB[1]), "httpB")

	fileconn, err := net.FileConn(rootBFD)
	check(err, "fileconn")
	unixconn, ok := fileconn.(*net.UnixConn)
	if !ok {
		log.Fatalln("not unixconn")
	}
	check(rootBFD.Close(), "closing root unix fd")
	rootBFD = nil

	argv := append([]string{os.Args[0], "serve-http"}, os.Args[2:len(os.Args)-1]...)
	attr := &os.ProcAttr{
		Files: []*os.File{
			os.Stdin,
			os.Stdout,
			os.Stderr,
			httpAFD,
			httpBFD,
		},
	}
	if config.IsolateBuilds.Enabled {
		attr.Sys = &syscall.SysProcAttr{
			Credential: &syscall.Credential{
				Uid:         uint32(config.IsolateBuilds.DingUid),
				Gid:         uint32(config.IsolateBuilds.DingGid),
				Groups:      []uint32{},
				NoSetGroups: false,
			},
		}
	}
	_, err = os.StartProcess(argv[0], argv, attr)
	check(err, "starting http process")

	check(httpAFD.Close(), "closing http fd a")
	check(httpBFD.Close(), "closing http fd b")
	httpAFD = nil
	httpBFD = nil

	dec := gob.NewDecoder(rootAFD)
	enc := gob.NewEncoder(rootAFD)
	err = enc.Encode(&config)
	check(err, "writing config to httpserver")
	for {
		var msg msg
		err := dec.Decode(&msg)
		check(err, "decoding msg")

		switch msg.Kind {
		case MsgChown:
			msgChown(msg, enc)
		case MsgRemovedir:
			msgRemovedir(msg, enc)
		case MsgBuild:
			msgBuild(msg, enc, unixconn)
		default:
			log.Fatalf("unknown msg kind %d\n", msg.Kind)
		}
	}
}

func calcUid(buildId int) int {
	return config.IsolateBuilds.UidStart + buildId%(config.IsolateBuilds.UidEnd-config.IsolateBuilds.UidStart)
}

func errstr(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func msgChown(msg msg, enc *gob.Encoder) {
	if !config.IsolateBuilds.Enabled {
		err := enc.Encode("")
		check(err, "encoding chown response")
		return
	}

	if msg.RepoName == "" {
		log.Fatal("received MsgChown with empty RepoName")
	}
	buildDir := fmt.Sprintf("%s/data/build/%s/%d", dingWorkDir, msg.RepoName, msg.BuildId)

	uid := calcUid(msg.BuildId)

	chown := func(path string) error {
		return filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// don't change symlinks, we would be modifying whatever they point to!
			if (info.Mode() & os.ModeSymlink) != 0 {
				return nil
			}
			return os.Chown(path, uid, config.IsolateBuilds.DingGid)
		})
	}

	err := chown(buildDir + "/home")
	if err == nil {
		err = chown(buildDir + "/checkout")
	}
	err = enc.Encode(errstr(err))
	check(err, "encoding msg")
}

func msgRemovedir(msg msg, enc *gob.Encoder) {
	if msg.RepoName == "" {
		log.Fatal("received MsgRemovedir with empty RepoName")
	}
	path := fmt.Sprintf("%s/data/build/%s", dingWorkDir, msg.RepoName)
	if msg.BuildId > 0 {
		path += fmt.Sprintf("/%d", msg.BuildId)
	}

	err := os.RemoveAll(path)
	err = enc.Encode(errstr(err))
	check(err, "writing removedir response")
}

func msgBuild(msg msg, enc *gob.Encoder, unixconn *net.UnixConn) {
	outr, outw, err := os.Pipe()
	check(err, "create stdout pipe")
	defer outr.Close()
	defer outw.Close()

	errr, errw, err := os.Pipe()
	check(err, "create stderr pipe")
	defer errr.Close()
	defer errw.Close()

	buildDir := fmt.Sprintf("%s/data/build/%s/%d", dingWorkDir, msg.RepoName, msg.BuildId)
	checkoutDir := fmt.Sprintf("%s/checkout/%s", buildDir, msg.CheckoutPath)

	uid := calcUid(msg.BuildId)

	devnull, err := os.Open("/dev/null")
	check(err, "opening /dev/null")
	defer devnull.Close()

	argv := []string{buildDir + "/scripts/build.sh"}
	attr := &os.ProcAttr{
		Dir: checkoutDir,
		Env: msg.Env,
		Files: []*os.File{
			devnull,
			outw,
			errw,
		},
	}
	if config.IsolateBuilds.Enabled {
		attr.Sys = &syscall.SysProcAttr{
			Credential: &syscall.Credential{
				Uid:         uint32(uid),
				Gid:         uint32(config.IsolateBuilds.DingGid),
				Groups:      []uint32{},
				NoSetGroups: false,
			},
		}
	}
	proc, err := os.StartProcess(argv[0], argv, attr)
	if err != nil {
		log.Println("start failed:", err)
		enc.Encode(err.Error())
		return
	}
	err = enc.Encode(errstr(err))
	check(err, "writing build start")

	statusr, statusw, err := os.Pipe()
	check(err, "create status pipe")

	buf := []byte{1}
	oob := unix.UnixRights(int(outr.Fd()), int(errr.Fd()), int(statusr.Fd()))
	_, _, err = unixconn.WriteMsgUnix(buf, oob, nil)
	defer statusr.Close()
	if err != nil {
		statusw.Close()
		check(err, "sending fds from root to http")
	}

	go func() {
		state, err := proc.Wait()
		if err == nil && !state.Success() {
			err = fmt.Errorf(state.String())
		}
		err = gob.NewEncoder(statusw).Encode(errstr(err))
		check(err, "writing status to http-serve")
	}()
}
