package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

const destination = "assets"

func main() {
	log.SetFlags(0)
	flag.Usage = func() {
		log.Println("usage: build {clean | install}")
		flag.PrintDefaults()
	}
	flag.Parse()
	args := flag.Args()
	if len(args) != 1 {
		flag.Usage()
		os.Exit(2)
	}
	switch args[0] {
	default:
		flag.Usage()
		os.Exit(2)
	case "clean":
		os.RemoveAll(destination)
	case "install":
		build(destination)
	}
}

func build(dest string) {
	target := func(s string) string {
		return dest + "/web/" + s
	}
	internalTarget := func(s string) string {
		return dest + "/" + s
	}

	var d string
	var s []string

	// angularjs templates
	d = target("static/js/app-templates.js")
	s = dirtree("www-src/html", ".html")
	if dirty(d, s) {
		write(d, ngtemplates("templates", s, "www-src/html/", "static/html/"))
	}

	// images
	s = dirlist("www-src/img", "", "")
	for _, e := range s {
		d = target(fmt.Sprintf("static/img/%s", path.Base(e)))
		dirtyCopy(d, e)
	}

	// app js
	d = target("static/js/app.js")
	s = concat(
		[]string{
			"www-src/js/app.js",
			"www-src/js/app.config.js",
		},
		dirlist("www-src/js/ctlr", ".js", ""),
		dirlist("www-src/js/directives", ".js", ""),
		dirlist("www-src/js/filters", ".js", ""),
		dirlist("www-src/js/services", ".js", ""),
	)
	if dirty(d, s) && jshint(s...) {
		copy(d, s...)
	}

	// vendor js
	d = target("static/js/app-vendor.js")
	s = []string{
		"www-src/vendors/js/jquery-3.1.0.min.js",
		"www-src/vendors/js/angular-1.5.7.min.js",
		"www-src/vendors/js/angular-route-1.5.7.min.js",
		"www-src/vendors/js/ui-bootstrap-tpls-1.3.3.min.js",
		"www-src/vendors/js/lodash-4.13.1.min.js",
	}
	dirtyCopy(d, s...)

	// vendor css
	d = target("static/css/app-vendor.css")
	s = []string{
		"www-src/vendors/bootstrap-3.3.6/css/bootstrap.min.css",
		"www-src/vendors/font-awesome-4.6.3/css/font-awesome.css",
	}
	dirtyCopy(d, s...)

	// app css
	d = target("static/css/app.css")
	s = concat(
		[]string{"www-src/scss/app.scss"},
		dirlist("www-src/scss", ".scss", "_"),
	)
	if dirty(d, s) {
		os.MkdirAll(path.Dir(d), os.ModePerm)
		run(filepath.Join("node_modules", ".bin", "node-sass"), "--style", "compact", s[0], d)
	}

	// fonts
	s = concat(
		dirlist("www-src/vendors/font-awesome-4.6.3/fonts", "", ""),
		dirlist("www-src/vendors/bootstrap-3.3.6/fonts", "", ""),
	)
	for _, e := range s {
		d = target("static/fonts/" + path.Base(e))
		dirtyCopy(d, e)
	}

	// index.html
	d = target("index.html")
	s = []string{
		"www-src/index.html",
		target("static/css/app-vendor.css"),
		target("static/css/app.css"),
		target("static/js/app-vendor.js"),
		target("static/js/app-templates.js"),
		target("static/js/app.js"),
		target("static/img/logo.png"),
	}
	if dirty(d, s) {
		write(d, revrepl(read("www-src/index.html"), dest+"/web"))
	}

	files := []string{
		"favicon.ico",
		"robots.txt",
	}
	for _, name := range files {
		dirtyCopy(target(name), "www-src/"+name)
	}

	dirtyCopy(internalTarget("INSTALL.md"), "INSTALL.md")

	// licenses
	type license struct {
		name  string
		files []string
	}
	d = target("LICENSES")
	lics := []license{
		{"Ding",
			[]string{"LICENSE.md"}},
		{"Go runtime and standard library",
			[]string{"www-src/licenses/go"}},
		{"Bootstrap 3.3.6",
			[]string{"www-src/licenses/bootstrap-3.3.6"}},
		{"Fontawesome 4.6.3\n\nFont Awesome by Dave Gandy - http://fontawesome.io\nFont licensed under SIL OFL 1.1-license\nCode, such as CSS, under MIT-license",
			[]string{}},
		{"jQuery 3.1.0",
			[]string{"www-src/licenses/jquery-3.1.0"}},
		{"lodash 4.13.1",
			[]string{"www-src/licenses/lodash-4.13.1"}},
		{"AngularJS including the route module 1.5.7",
			[]string{"www-src/licenses/angularjs-1.5.7"}},
		{"UI Bootstrap 1.3.3",
			[]string{"www-src/licenses/ui-bootstrap-1.3.3"}},
		{"Sherpa Go server library",
			[]string{"vendor/bitbucket.org/mjl/sherpa/LICENSE"}},
		{"httpasset Go library",
			[]string{"vendor/bitbucket.org/mjl/httpasset/LICENSE"}},
		{"", []string{"vendor/github.com/beorn7/perks/LICENSE"}},
		{"", []string{"vendor/github.com/golang/protobuf/LICENSE"}},
		{"", []string{"vendor/github.com/irias/sherpa-prometheus-collector/LICENSE.md"}},
		{"", []string{"vendor/github.com/lib/pq/LICENSE.md"}},
		{"", []string{"vendor/github.com/matttproud/golang_protobuf_extensions/LICENSE"}},
		{"Prometheus Go client", []string{
			"vendor/github.com/prometheus/client_golang/LICENSE",
			"vendor/github.com/prometheus/client_golang/NOTICE",
			"vendor/github.com/prometheus/client_model/NOTICE",
			"vendor/github.com/prometheus/common/NOTICE",
			"vendor/github.com/prometheus/procfs/NOTICE"}},
	}
	s = nil
	for _, lic := range lics {
		s = concat(s, lic.files)
	}
	if dirty(d, s) {
		r := "# Licenses for included software:\n\n"
		for _, lic := range lics {
			if lic.name == "" {
				lic.name = lic.files[0]
				if strings.HasPrefix(lic.name, "vendor/") {
					lic.name = lic.name[len("vendor/"):]
				}
			}
			r += "## " + lic.name + "\n\n"
			for _, file := range lic.files {
				r += read(file) + "\n"
			}
			r += "\n\n"
		}
		write(d, r)
	}

	// sql
	type sql struct {
		Version  int    `json:"version"`
		Filename string `json:"filename"`
		SQL      string `json:"sql"`
	}
	var sqls []sql
	var sqlFilenames []string
	for _, f := range sorted(readdir("sql")) {
		match, err := regexp.MatchString("[0-9]{3}-.*\\.sql", f)
		check(err, "regexp")
		if !match {
			continue
		}
		versionStr := strings.Split(f, "-")[0]
		var version int
		if versionStr == "000" {
			version = 0
		} else {
			version = parseInt(versionStr)
		}
		sqls = append(sqls, sql{Version: version, Filename: f})
		sqlFilenames = append(sqlFilenames, "sql/"+f)
	}
	d = internalTarget("sql.json")
	if dirty(d, append(sqlFilenames, "sql")) {
		for i := range sqls {
			sqls[i].SQL = read("sql/" + sqls[i].Filename)
		}
		write(d, string(toJSON(sqls)))
	}
}
