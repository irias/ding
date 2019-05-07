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

	var httpFS http.FileSystem

	func init() {
		// the error-check is optional, httpasset.Fs() always returns a non-nil http.FileSystem.
		// however, after failed initialization (eg no zip file was appended to the binary),
		// fs operations return an error.
		httpFS = httpasset.Fs()
		if err := httpasset.Error(); err != nil {
			log.Fatal(err)
			// or alternatively fallback to to local file system:
			log.Print("falling back to local assets")
			httpFS = http.Dir("assets")
		}
	}

	func main() {
		http.Handle("/", http.FileServer(httpFS))
		addr := ":8000"
		log.Println("listening on", addr)
		log.Fatal(http.ListenAndServe(addr, nil))
	}


Build your program, let's say the result is "mybinary".
Now create a zip file, eg on unix:

	(cd assets && zip -rq0 ../assets.zip .)

Append it to the binary:

	cat assets.zip >>mybinary

If you run mybinary, it will serve http on port 8000, serving the
files from the zip file that was appended to the binary.

Note that net/http's FileServer will redirect requests for /index.html
to /, and handle requests for / by returning the file /index.html
if it exists.  If /index.html doesn't exist, it will list the
contents of the directory. For net/http's Dir(), that works (the
fallback `fs` in the example code). For httpasset's `fs`, reading
directories isn't supported and returns an empty list of files.
Listing files is often not needed, simpler, and it's usually better
not to leak such information.

net/http's FileServer also supports requests for random i/o (range
requests), and advertises this in response headers.  Files in zip
files can be compressed. Compressed files don't support random
access. Httpasset returns an error when asked to serve range requests
for compressed files. It's recommeded to add files to the zip file
uncompressed. The -0 flag takes care of this in the example given
earlier.

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
)

var (
	NotDirErr    = errors.New("not a directory")
	LocateZipErr = errors.New("could not locate zip file, no end-of-central-directory signature found")
)

type opener interface {
	Open() (http.File, error)
}

type fileOpener struct {
	io.ReaderAt
	zipFile *zip.File
}

func (f fileOpener) Open() (http.File, error) {
	if f.zipFile.Method == zip.Store {
		offset, err := f.zipFile.DataOffset()
		if err != nil {
			return nil, err
		}
		return &uncompressedFile{io.NewSectionReader(f.ReaderAt, offset, int64(f.zipFile.UncompressedSize64)), f.zipFile}, nil
	}
	ff, err := f.zipFile.Open()
	if err != nil {
		return nil, err
	}
	return &compressedFile{ff, f.zipFile}, nil
}

type httpassetFS struct {
	binary io.Closer
	files  map[string]opener
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
	bin, err := os.Open(os.Args[0])
	if err != nil {
		return nil, err
	}
	fi, err := bin.Stat()
	if err != nil {
		bin.Close()
		return nil, err
	}

	n := int64(65 * 1024)
	size := fi.Size()
	if size < n {
		n = size
	}
	buf := make([]byte, n)
	_, err = io.ReadAtLeast(io.NewSectionReader(bin, size-n, n), buf, len(buf))
	if err != nil {
		bin.Close()
		return nil, err
	}
	o := int64(findSignatureInBlock(buf))
	if o < 0 {
		bin.Close()
		return nil, LocateZipErr
	}
	cdirsize := int64(binary.LittleEndian.Uint32(buf[o+12:]))
	cdiroff := int64(binary.LittleEndian.Uint32(buf[o+16:]))
	zipsize := cdiroff + cdirsize + (int64(len(buf)) - o)

	rr := io.NewSectionReader(bin, size-zipsize, zipsize)
	r, err := zip.NewReader(rr, zipsize)
	if err != nil {
		bin.Close()
		return nil, err
	}

	// build map of files. we create our own dirs, we don't want to be dependent on zip files containing proper hierarchies.
	files := map[string]opener{}
	files[""] = dir{}
	for _, f := range r.File {
		if strings.HasSuffix(f.Name, "/") {
			continue
		}
		files[f.Name] = fileOpener{rr, f}
		elems := strings.Split(f.Name, "/")
		for e := 1; e <= len(elems)-1; e++ {
			name := strings.Join(elems[:e], "/")
			files[name] = dir{}
		}
	}
	return &httpassetFS{bin, files}, nil
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
		fs.binary.Close()
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
