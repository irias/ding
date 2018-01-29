package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

func check(err error, msg string) {
	if err != nil {
		log.Printf("%s: %s\n", msg, err)
		panic(err)
	}
}

func toJSON(v interface{}) string {
	buf, err := json.Marshal(v)
	check(err, "marshal to json")
	return string(buf)
}

func concat(l ...[]string) (r []string) {
	for _, e := range l {
		r = append(r, e...)
	}
	return
}

func parseInt(s string) int {
	r, err := strconv.ParseInt(s, 10, 32)
	check(err, "parseInt")
	return int(r)
}

func write(p, contents string) {
	log.Println("write", p)
	os.MkdirAll(path.Dir(p), os.ModePerm)
	f, err := os.Create(p)
	check(err, "create")
	_, err = f.Write([]byte(contents))
	check(err, "write")
	check(f.Close(), "close")
}

func jshint(paths ...string) bool {
	jshintPath := filepath.Join("node_modules", ".bin", "jshint")
	return run(jshintPath, paths...)
}

func read(path string) string {
	buf, err := ioutil.ReadFile(path)
	check(err, "readfile")
	return string(buf)
}

func readall(paths ...string) string {
	s := ""
	for _, p := range paths {
		s += read(p)
	}
	return s
}

func copy(dest string, srcs ...string) {
	write(dest, readall(srcs...))
}

func makehash(p string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(read(p))))[:12]
}

func revrepl(contents, dest string) string {
	re1 := regexp.MustCompile(`\s(href|src)="([^"]+)\?v=[a-zA-Z0-9]+"`)
	contents = re1.ReplaceAllStringFunc(contents, func(s string) string {
		l := re1.FindStringSubmatch(s)
		filename := l[2]
		v := makehash(dest + "/" + filename)
		return fmt.Sprintf(` %s="%s?v=%s"`, l[1], filename, v)
	})
	re2 := regexp.MustCompile(`url\('([^']+)\?v=[a-zA-Z0-9]+'\)`)
	contents = re2.ReplaceAllStringFunc(contents, func(s string) string {
		l := re2.FindStringSubmatch(s)
		filename := l[1]
		v := makehash(dest + "/" + filename)
		return fmt.Sprintf(`url('%s?v=%s')`, filename, v)
	})
	return contents
}

func run(cmd string, args ...string) bool {
	c := exec.Command(cmd, args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	err := c.Run()
	check(err, fmt.Sprintf("run %s %v", cmd, args))
	return true
}

func readdir(dir string) (r []string) {
	l, err := ioutil.ReadDir(dir)
	check(err, "readdir")
	for _, fi := range l {
		r = append(r, fi.Name())
	}
	return
}

func dirlist(dir, suffix, prefix string) (r []string) {
	l, err := ioutil.ReadDir(dir)
	check(err, "readdir")
	for _, fi := range l {
		name := fi.Name()
		if strings.HasSuffix(name, suffix) && strings.HasPrefix(name, prefix) {
			r = append(r, filepath.ToSlash(dir+"/"+name))
		}
	}
	return
}

func dirtree(dir, suffix string) (r []string) {
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		check(err, "walk")
		name := info.Name()
		if !info.IsDir() && strings.HasSuffix(name, suffix) {
			r = append(r, filepath.ToSlash(p))
		}
		return nil
	})
	check(err, "walk")
	return
}

func ngtemplates(modname string, paths []string, stripPrefix, addPrefix string) string {
	r := fmt.Sprintf("angular.module(%s, []).run([\"$templateCache\", function($templateCache) {\n", toJSON(modname))
	for _, p := range paths {
		htmlPath := p
		if !strings.HasPrefix(htmlPath, stripPrefix) {
			panic(fmt.Sprintf("unexpected files for template: %s", htmlPath))
		}
		htmlPath = htmlPath[len(stripPrefix):]
		htmlPath = addPrefix + htmlPath
		r += fmt.Sprintf("$templateCache.put(%s,%s);\n", toJSON(htmlPath), toJSON(read(p)))
	}
	r += "}]);\n"
	return r
}

func dirty(d string, s []string) bool {
	if len(s) == 0 {
		panic("empty source list")
	}

	stat := func(p string) int64 {
		info, err := os.Stat(p)
		if os.IsNotExist(err) {
			return 0
		}
		check(err, "stat")
		return info.ModTime().Unix()
	}

	dmtime := stat(d)
	smtime := int64(0)
	for _, e := range s {
		t := stat(e)
		if t == 0 {
			return true
		}
		if t > smtime {
			smtime = t
		}
	}
	return smtime > dmtime
}

func dirtyCopy(d string, s ...string) {
	if dirty(d, s) {
		copy(d, s...)
	}
}

func sorted(l []string) []string {
	sort.Slice(l, func(i, j int) bool {
		return l[i] < l[j]
	})
	return l
}
