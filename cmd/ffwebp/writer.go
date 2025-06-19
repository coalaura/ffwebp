package main

import "io"

type countWriter struct {
	w io.Writer
	n int64
}

func (cw *countWriter) Write(p []byte) (int, error) {
	m, err := cw.w.Write(p)

	cw.n += int64(m)

	return m, err
}
