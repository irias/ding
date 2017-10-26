/*
Package httpasset lets you embed files in your go binary by simply appending a zip file to said binary.

Typically on launch of your program, you call httpasset.Fs() to get a
handle to the http.FileSystem that represents the appended zip file.
It is on this fs that you should call Open and friends, as opposed to
the normal os.Open.

An example:

	package main

	import (
		"log"
		"net/http"
		"bitbucket.org/mjl/httpasset"
	)

	func main() {
		// the error-check is optional, httpasset.Fs() always returns a non-nil http.FileSystem.
		// however, after failed initialization (eg no zip file was appended to the binary),
		// fs operations return an error.
		fs := httpasset.Fs()
		if err := httpasset.Error(); err != nil {
			log.Print(err)
			log.Print("falling back to local assets")
			// or alternatively fallback to to local file system:
			fs = http.Dir("assets")
		}

		http.Handle("/", http.FileServer(fs))
		addr := ":8000"
		log.Println("listening on", addr)
		log.Fatal(http.ListenAndServe(addr, nil))
	}


Build your program, let's say the result is "mybinary".
Now create a zip file, eg on unix:

	(cd assets && zip -r0 ../assets.zip .)

Append it to the binary:

	cat assets.zip >>mybinary

If you run mybinary, it will serve http on port 8000, serving the
files from the zip file that was appended to the binary.

Note that net/http's FileServer will redirect requests for /index.html
to /, and handle requests for / by returning the file /index.html
if it exists.  If /index.html doesn't exist, it will list the
contents of the directory. For net/http's Dir(), that works (the
fallback `fs` in the example code). For httpasset's `fs`, reading
directories isn't supported and returns an empty list of files
(listing files is often not needed, simpler, and it's usually better
not to leak such information).

net/http's FileServer also supports requests for random i/o (range
requests), and advertises this in response headers (unfortunately).
Files in zip files can be compressed. Compressed files don't support
random access. Httpasset returns an error when asked to serve range
requests. In the future, support for random i/o could be added for
files in the zip file that aren't compressed.

To make this work, an assumption about zip files is made:
That the central directory (with a list of files inside the zip file)
comes right before the "end of central directory" marker.  This is almost
always the case with zip files.  With this assumption, httpasset can locate
the start and end of the zip file that is appended to the binary, which
archive/zip needs in order to parse the zip file.

Some existing tools for reading zip files can still read the
binary-with-zipfile as a zip file.  For example 7z, and the unzip
command-line tool.  Windows XP's explorer zip opener does NOT seem to
understand it, and also Mac OS X's archive utility gets confused.

This has been tested with binaries on Linux (Ubuntu 12.04), Mac OS X
(10.9.2) and Windows 8.1.  These operating systems don't
seem to mind extra data at the end of the binary.
*/
package httpasset

import (
	"archive/zip"
	"encoding/binary"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type opener interface {
	Open() (http.File, error)
}

type fileOpener struct {
	file *zip.File
}

type file struct {
	offset int64 // offset in rc
	atEOF  bool  // if true, we ignore offset and rc, and just return eof, in order to implement part of Seek
	rc     io.ReadCloser
	file   *zip.File
}

func (f fileOpener) Open() (http.File, error) {
	ff, err := f.file.Open()
	if err != nil {
		return nil, err
	}
	return &file{0, false, ff, f.file}, nil
}

func (f *file) Seek(offset int64, whence int) (int64, error) {
	// we implement part of Seek so we can use net/http.ServeContent, which seeks to the end to determine file size
	switch whence {
	case io.SeekStart:
		if offset == 0 && f.offset == 0 {
			f.atEOF = false
			return 0, nil
		}
	case io.SeekEnd:
		if offset == 0 {
			f.atEOF = true
			return int64(f.file.UncompressedSize64), nil
		}
	}
	return -1, errors.New("seek only partially supported")
}

func (f *file) Close() error {
	return f.rc.Close()
}

func (f *file) Read(buf []byte) (int, error) {
	if f.atEOF {
		return 0, nil
	}
	n, err := f.rc.Read(buf)
	if n >= 0 {
		f.offset += int64(n)
	}
	return n, err
}

func (f *file) Readdir(count int) ([]os.FileInfo, error) {
	return nil, errors.New("not a directory")
}

func (f *file) Stat() (os.FileInfo, error) {
	return f.file.FileInfo(), nil
}

type dir struct {
}

func (d dir) Open() (http.File, error) {
	return d, nil
}

func (d dir) Close() error {
	return nil
}

func (d dir) Read(p []byte) (n int, err error) {
	return -1, errors.New("cannot read on directory")
}

func (d dir) Seek(offset int64, whence int) (int64, error) {
	return -1, errors.New("cannot seek on directory")
}

func (d dir) Readdir(count int) ([]os.FileInfo, error) {
	return []os.FileInfo{}, nil
}

type fileinfo struct {
}

func (fi fileinfo) Name() string {
	return ""
}

func (fi fileinfo) Size() int64 {
	return 0
}

func (fi fileinfo) Mode() os.FileMode {
	return os.ModeDir | 0777
}

func (fi fileinfo) ModTime() time.Time {
	return time.Time{}
}

func (fi fileinfo) IsDir() bool {
	return true
}

func (fi fileinfo) Sys() interface{} {
	return nil
}

var zerofileinfo fileinfo

func (d dir) Stat() (os.FileInfo, error) {
	return zerofileinfo, nil
}

type httpassetFS struct {
	rc    *zip.ReadCloser
	files map[string]opener
}

var fs http.FileSystem

// Fs returns the http.FileSystem for the assets contained in the binary.
// It always returns a non-nil FileSystem.  In case of an initialization
// error a "failing fs" is returned that returns errors for all operations.
func Fs() http.FileSystem {
	if fs == nil {
		var err error
		fs, err = open()
		if err != nil {
			fs = failfs(err)
		}
	}
	return fs
}

// find end-of-directory struct, near the end of the file.
// it specifies the size & offset of the central directory.
// we assume the central directory is located just before the end-of-central-directory.
// so that allows us to calculate the original size of the zip file.
// which in turn allows us to use godoc's zipfs to serve the zip file withend.
func open() (http.FileSystem, error) {
	f, err := os.Open(os.Args[0])
	if err != nil {
		return nil, err
	}
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}

	n := int64(65 * 1024)
	size := fi.Size()
	if size < n {
		n = size
	}
	buf := make([]byte, n)
	_, err = io.ReadAtLeast(io.NewSectionReader(f, size-n, n), buf, len(buf))
	if err != nil {
		return nil, err
	}
	o := int64(findSignatureInBlock(buf))
	if o < 0 {
		return nil, errors.New("could not locate zip file, no end-of-central-directory signature found")
	}
	cdirsize := int64(binary.LittleEndian.Uint32(buf[o+12:]))
	cdiroff := int64(binary.LittleEndian.Uint32(buf[o+16:]))
	zipsize := cdiroff + cdirsize + (int64(len(buf)) - o)

	rr := io.NewSectionReader(f, size-zipsize, zipsize)
	r, err := zip.NewReader(rr, zipsize)
	if err != nil {
		return nil, err
	}

	rc := &zip.ReadCloser{Reader: *r}

	// build map of files. we create our own dirs, we don't want to be dependent on zip files containing proper hierarchies.
	files := map[string]opener{}
	files[""] = dir{}
	for _, f := range rc.File {
		if strings.HasSuffix(f.Name, "/") {
			continue
		}
		files[f.Name] = fileOpener{f}
		elems := strings.Split(f.Name, "/")
		for e := 1; e <= len(elems)-1; e++ {
			name := strings.Join(elems[:e], "/")
			files[name] = dir{}
		}
	}
	return &httpassetFS{rc, files}, nil
}

// Error returns a non-nil error if no asset could be found in the binary.
// For example when no zip file was appended to the binary.
func Error() error {
	switch fs := Fs().(type) {
	case *failFS:
		return fs.err
	}
	return nil
}

// Close the FileSystem, closing open files to the (zip file within) the binary.
func Close() {
	switch fs := Fs().(type) {
	case *httpassetFS:
		fs.rc.Close()
	}
	fs = nil
}

func (fs *httpassetFS) Open(name string) (http.File, error) {
	if !strings.HasPrefix(name, "/") {
		return nil, os.ErrNotExist
	}
	name = name[1:]
	file, ok := fs.files[name]
	if ok {
		return file.Open()
	}
	return nil, os.ErrNotExist
}
