package httpasset

import (
	"errors"
	"net/http"
)

type failFS struct {
	err error
}

func failfs(err error) http.FileSystem {
	return &failFS{errors.New("httpasset: " + err.Error())}
}

func (fs *failFS) Open(name string) (http.File, error) {
	return nil, fs.err
}
