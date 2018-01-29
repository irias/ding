package httpasset

import (
	"archive/zip"
	"io"
	"net/http"
	"os"
)

type uncompressedFile struct {
	*io.SectionReader
	zipFile *zip.File
}

var _ http.File = &uncompressedFile{}

func (f *uncompressedFile) Close() error {
	return nil
}

func (f *uncompressedFile) Stat() (os.FileInfo, error) {
	return f.zipFile.FileInfo(), nil
}

func (f *uncompressedFile) Readdir(count int) ([]os.FileInfo, error) {
	return nil, NotDirErr
}
