package main

import (
	"bytes"
	"io"
)

type lineWriter struct {
	w   io.WriteCloser
	buf []byte
}

func LineWriter(w io.WriteCloser) io.WriteCloser {
	return &lineWriter{w, nil}
}

func (lw *lineWriter) Write(buf []byte) (int, error) {
	n := len(buf)
	i := bytes.LastIndexByte(buf, '\n')
	if i < 0 {
		lw.buf = append(lw.buf, buf...)
		return n, nil
	}

	if len(lw.buf) > 0 {
		_, err := lw.w.Write(lw.buf)
		if err != nil {
			return -1, err
		}
		lw.buf = nil
	}
	_, err := lw.w.Write(buf[:i+1])
	if err != nil {
		return -1, err
	}
	buf = buf[i+1:]
	if len(buf) > 0 {
		lw.buf = buf
	}
	return n, nil
}

func (lw *lineWriter) Close() (err error) {
	if len(lw.buf) > 0 {
		_, err = lw.w.Write(lw.buf)
	}
	err2 := lw.w.Close()
	if err == nil {
		err = err2
	}
	return err
}
