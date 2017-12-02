package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
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

	// returns nil on error, with http response already sent.
	gatherFiles := func(query string, pathMaker func(repoCheckoutPath, name string) string) []archiveFile {
		rows, err := database.Query(query, repoName, buildID)
		if err != nil {
			fail(err)
			return nil
		}
		defer rows.Close()
		files := []archiveFile{}
		for rows.Next() {
			var repoCheckoutPath, name string
			var filesize int64
			err = rows.Scan(&repoCheckoutPath, &name, &filesize)
			if err != nil {
				fail(err)
				return nil
			}
			files = append(files, archiveFile{pathMaker(repoCheckoutPath, name), filesize})
		}
		if err = rows.Err(); err != nil {
			fail(err)
			return nil
		}
		if len(files) == 0 {
			http.NotFound(w, r)
			return nil
		}
		return files
	}

	switch t[1] {
	case "release":
		q := `
			select repo.checkout_path, result.filename, result.filesize
			from result
			join build on result.build_id = build.id
			join repo on build.repo_id = repo.id
			join release on build.id = release.build_id
			where repo.name=$1 and build.id=$2
		`
		files := gatherFiles(q, func(repoCheckoutPath, name string) string {
			return fmt.Sprintf("data/release/%s/%d/%s", repoName, buildID, path.Base(name))
		})
		name := t[4]
		isGzip := true
		_serveDownload(w, r, name, files, isGzip)

	case "result":
		q := `
			select repo.checkout_path, result.filename, result.filesize
			from result
			join build on result.build_id = build.id
			join repo on build.repo_id = repo.id
			where repo.name=$1 and build.id=$2
		`
		files := gatherFiles(q, func(repoCheckoutPath, name string) string {
			return fmt.Sprintf("data/build/%s/%d/checkout/%s/%s", repoName, buildID, repoCheckoutPath, name)
		})
		name := t[4]
		isGzip := false
		_serveDownload(w, r, name, files, isGzip)

	default:
		http.NotFound(w, r)
		return
	}

}

type archiveFile struct {
	Path string
	Size int64
}

// we have .gz on disk.  gzip is a deflate stream with a header and a footer.
// a zip file consists of headers/footer and deflate streams.
// so we can serve zip files quickly based on the .gz's on disk.
// we just have to strip the gzip header & footer and pass the raw deflate stream through.
type gzipStrippingDeflateWriter struct {
	w        io.Writer
	header   []byte // we need the 10 byte header before we can do anything.
	flag     byte   // from header, indicates optional fields we must skip. we clear the flags one we skipped parts.
	leftover []byte // we always hold the last 8 bytes back, it could be the gzip footer that we must skip.
}

func (x *gzipStrippingDeflateWriter) Write(buf []byte) (int, error) {
	n := len(buf)

	if len(x.header) < 10 {
		take := 10 - len(x.header)
		if take > len(buf) {
			take = len(buf)
		}
		x.header = append(x.header, buf[:take]...)
		buf = buf[take:]
		if len(x.header) == 10 {
			if x.header[0] != 0x1f || x.header[1] != 0x8b {
				return -1, fmt.Errorf("not a gzip header: %x", x.header[:2])
			}
			x.flag = x.header[3]
		}
	}
	const (
		FlagFHCRC = 1 << iota
		FlagFEXTRA
		FlagFNAME
		FlagFCOMMENT
	)

	// null-terminated string
	skipString := func(l []byte) ([]byte, bool) {
		for i := range l {
			if l[i] == 0 {
				return l[i+1:], true
			}
		}
		return nil, false
	}

	if (x.flag & FlagFEXTRA) != 0 {
		return -1, fmt.Errorf("extra gzip header data, not supported yet") // please fix me (:
	}
	if (x.flag & FlagFNAME) != 0 {
		var skipped bool
		buf, skipped = skipString(buf)
		if skipped {
			x.flag &^= FlagFNAME
		}
	}
	if (x.flag & FlagFCOMMENT) != 0 {
		var skipped bool
		buf, skipped = skipString(buf)
		if skipped {
			x.flag &^= FlagFCOMMENT
		}
	}

	if (x.flag & FlagFHCRC) != 0 {
		// 2 bytes

		if len(x.leftover)+len(buf) < 2 {
			x.leftover = append(x.leftover, buf...)
			return n, nil
		}
		drop := 2
		if len(x.leftover) > 0 {
			xdrop := drop
			if len(x.leftover) < drop {
				xdrop = len(x.leftover)
			}
			drop -= xdrop
			x.leftover = x.leftover[xdrop:]
		}
		buf = buf[:drop]
		x.flag &^= FlagFHCRC
	}

	if len(buf) < 8 {
		nn := 8 - len(buf)
		if nn > len(x.leftover) {
			nn = len(x.leftover)
		}
		if nn > 0 {
			_, err := x.w.Write(x.leftover[:nn])
			if err != nil {
				return -1, err
			}
			x.leftover = x.leftover[nn:]
		}
		x.leftover = append(x.leftover, buf...)
		return n, nil
	}
	// below here, we have at least 8 bytes in buf

	if len(x.leftover) > 0 {
		_, err := x.w.Write(x.leftover)
		if err != nil {
			return -1, err
		}
		x.leftover = nil
	}
	if len(buf) > 8 {
		_, err := x.w.Write(buf[:len(buf)-8])
		if err != nil {
			return -1, err
		}
	}
	x.leftover = append(x.leftover, buf[len(buf)-8:]...)
	return n, nil
}

func (x *gzipStrippingDeflateWriter) Close() error {
	if len(x.leftover) != 8 {
		return fmt.Errorf("not 8 bytes left over at close")
	}
	return nil
}

func newGzipStrippingDeflateWriter(w io.Writer) (io.WriteCloser, error) {
	return &gzipStrippingDeflateWriter{w: w}, nil
}

// `files` do not have the .gz suffix they have in the file system.
func _serveDownload(w http.ResponseWriter, r *http.Request, name string, files []archiveFile, isGzip bool) {
	if strings.HasSuffix(name, ".zip") {
		base := strings.TrimSuffix(name, ".zip")
		w.Header().Set("Content-Type", "application/zip")
		zw := zip.NewWriter(w)
		if isGzip {
			zw.RegisterCompressor(zip.Deflate, newGzipStrippingDeflateWriter)
		}

		addFile := func(file archiveFile) bool {
			lpath := file.Path
			if isGzip {
				lpath += ".gz"
			}
			f, err := os.Open(lpath)
			if err != nil {
				log.Printf("download: open %s to add to zip: %s\n", lpath, err)
				return false
			}
			defer f.Close()

			filename := path.Base(file.Path)
			fw, err := zw.Create(base + "/" + filename)
			if err != nil {
				log.Printf("download: adding file to zip: %s\n", err)
				return false
			}
			_, err = io.Copy(fw, f)
			if err != nil {
				// probably just a closed connection
				log.Printf("download: copying data: %s\n", err)
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
		err := zw.Close()
		if err != nil {
			log.Printf("download: finishing write: %s\n", err)
		}
	} else if strings.HasSuffix(name, ".tgz") {
		base := strings.TrimSuffix(name, ".tgz")
		gzw := gzip.NewWriter(w)
		tw := tar.NewWriter(gzw)

		addFile := func(file archiveFile) bool {
			lpath := file.Path
			if isGzip {
				lpath += ".gz"
			}
			f, err := os.Open(lpath)
			if err != nil {
				log.Printf("download: open %s to add to tgz: %s\n", lpath, err)
				return false
			}
			defer f.Close()
			fi, err := f.Stat()
			if err != nil {
				log.Printf("download: stat %s to add to tgz: %s\n", lpath, err)
				return false
			}
			var gzr io.Reader = f
			if isGzip {
				gzr, err = gzip.NewReader(f)
				if err != nil {
					log.Printf("download: reading gzip %s: %s\n", lpath, err)
					return false
				}
			}

			hdr := &tar.Header{
				Name:     base + "/" + path.Base(file.Path),
				Mode:     int64(fi.Mode().Perm()),
				Size:     file.Size,
				ModTime:  fi.ModTime(),
				Typeflag: tar.TypeReg,
			}
			err = tw.WriteHeader(hdr)
			if err != nil {
				log.Printf("download: adding file to tgz: %s\n", err)
				return false
			}
			_, err = io.Copy(tw, gzr)
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
