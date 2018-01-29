package httpasset

import (
	"os"
	"time"
)

type fileinfo struct {
}

var zerofileinfo fileinfo

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
