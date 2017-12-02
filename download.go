package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
)

func serveDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "bad method", 405)
		return
	}

	// /download/{release,result}/<reponame>/<buildid>/<name>.{zip.tgz}
	t := strings.Split(r.URL.Path[1:], "/")
	if len(t) != 5 || hasBadElems(t) {
		http.NotFound(w, r)
		return
	}

	repoName := t[2]
	buildID, err := strconv.Atoi(t[3])
	if err != nil {
		http.NotFound(w, r)
		return
	}

	fail := func(err error) {
		log.Printf("download: %s\n", err)
		http.Error(w, "internal error", 500)
	}

	switch t[1] {
	case "release":
		path := fmt.Sprintf("data/release/%s/%d", repoName, buildID)
		fileinfos, err := ioutil.ReadDir(path)
		if err != nil {
			if os.IsNotExist(err) {
				http.NotFound(w, r)
				return
			}
			fail(err)
			return
		}
		files := make([]string, len(fileinfos))
		for i, fi := range fileinfos {
			files[i] = path + "/" + fi.Name()
		}

		name := t[4]
		_serveDownload(w, r, name, files)

	case "result":
		q := `
			select repo.checkout_path, result.filename
			from result
			join build on result.build_id = build.id
			join repo on build.repo_id = repo.id
			where repo.name=$1 and build.id=$2
		`
		rows, err := database.Query(q, repoName, buildID)
		if err != nil {
			fail(err)
			return
		}
		defer rows.Close()
		files := []string{}
		for rows.Next() {
			var repoCheckoutPath, name string
			err = rows.Scan(&repoCheckoutPath, &name)
			if err != nil {
				fail(err)
				return
			}
			file := fmt.Sprintf("data/build/%s/%d/checkout/%s/%s", repoName, buildID, repoCheckoutPath, name)
			files = append(files, file)
		}
		if err = rows.Err(); err != nil {
			fail(err)
			return
		}
		name := t[4]
		_serveDownload(w, r, name, files)

	default:
		http.NotFound(w, r)
		return
	}

}

func _serveDownload(w http.ResponseWriter, r *http.Request, name string, files []string) {
	if strings.HasSuffix(name, ".zip") {
		base := strings.TrimSuffix(name, ".zip")
		w.Header().Set("Content-Type", "application/zip")
		zw := zip.NewWriter(w)

		addFile := func(xpath string) bool {
			f, err := os.Open(xpath)
			if err != nil {
				log.Printf("download: open %s to add to zip: %s\n", xpath, err)
				return false
			}
			defer f.Close()

			filename := path.Base(xpath)
			fw, err := zw.Create(base + "/" + filename)
			if err != nil {
				log.Printf("download: adding file to zip: %s\n", err)
				return false
			}
			_, err = io.Copy(fw, f)
			if err != nil {
				// probably just a closed connection
				return false
			}
			return true
		}
		for _, path := range files {
			if !addFile(path) {
				break
			}
		}
		// errors would probably be closed connections
		zw.Close()
	} else if strings.HasSuffix(name, ".tgz") {
		base := strings.TrimSuffix(name, ".tgz")
		gzw := gzip.NewWriter(w)
		tw := tar.NewWriter(gzw)

		addFile := func(xpath string) bool {
			f, err := os.Open(xpath)
			if err != nil {
				log.Printf("download: open %s to add to tgz: %s\n", xpath, err)
				return false
			}
			defer f.Close()
			fi, err := f.Stat()
			if err != nil {
				log.Printf("download: stat %s to add to tgz: %s\n", xpath, err)
				return false
			}

			hdr := &tar.Header{
				Name:     base + "/" + path.Base(xpath),
				Mode:     int64(fi.Mode().Perm()),
				Size:     fi.Size(),
				ModTime:  fi.ModTime(),
				Typeflag: tar.TypeReg,
			}
			err = tw.WriteHeader(hdr)
			if err != nil {
				log.Printf("download: adding file to tgz: %s\n", err)
				return false
			}
			_, err = io.Copy(tw, f)
			if err != nil {
				// probably just a closed connection
				return false
			}
			return true
		}
		for _, path := range files {
			if !addFile(path) {
				break
			}
		}
		// errors would probably be closed connections
		tw.Close()
		gzw.Close()
	} else {
		http.NotFound(w, r)
		return
	}
}
