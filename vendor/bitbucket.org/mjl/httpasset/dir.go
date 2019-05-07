package httpasset

import (
	"errors"
	"net/http"
	"os"
)

type dir struct {
}

func (d dir) Open() (http.File, error) {
	return d, nil
}

func (d dir) Close() error {
	return nil
}

var readOnDirErr = errors.New("cannot read on directory")

func (d dir) Read(p []byte) (n int, err error) {
	return -1, readOnDirErr
}

var seekOnDirErr = errors.New("cannot seek on directory")

func (d dir) Seek(offset int64, whence int) (int64, error) {
	return -1, seekOnDirErr
}

func (d dir) Readdir(count int) ([]os.FileInfo, error) {
	return []os.FileInfo{}, nil
}

func (d dir) Stat() (os.FileInfo, error) {
	return zerofileinfo, nil
}
