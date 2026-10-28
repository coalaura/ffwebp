package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/coalaura/ffwebp/internal/logx"
)

type countWriter struct {
	w io.Writer
	n int64
}

func (cw *countWriter) Write(p []byte) (int, error) {
	m, err := cw.w.Write(p)

	cw.n += int64(m)

	if err == nil && m != len(p) {
		err = io.ErrShortWrite
	}

	return m, err
}

func writeOutput(path string, skipExisting bool, logger io.Writer, encode func(io.Writer) error) (int64, error) {
	if path == "-" {
		logx.Fprintf(logger, "writing output to <stdout>\n")

		writer := &countWriter{w: os.Stdout}
		err := encode(writer)

		return writer.n, err
	}

	skipped, err := skipOutput(path, skipExisting, logger)
	if err != nil || skipped {
		return 0, err
	}

	mode := os.FileMode(0644)

	info, err := os.Stat(path)
	if err == nil {
		mode = info.Mode().Perm()
	} else if !errors.Is(err, os.ErrNotExist) {
		return 0, err
	}

	err = os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		return 0, err
	}

	file, err := os.CreateTemp(filepath.Dir(path), ".ffwebp-*")
	if err != nil {
		return 0, err
	}

	temporary := file.Name()

	defer os.Remove(temporary)
	defer file.Close()

	logx.Fprintf(logger, "writing output file %q\n", filepath.ToSlash(path))

	writer := &countWriter{w: file}

	encodeErr := encode(writer)
	closeErr := file.Close()

	err = errors.Join(encodeErr, closeErr)
	if err != nil {
		return writer.n, err
	}

	err = os.Chmod(temporary, mode)
	if err != nil {
		return writer.n, err
	}

	if skipExisting {
		// A link publishes the completed file without overwriting a concurrent output.
		err = os.Link(temporary, path)
		if errors.Is(err, os.ErrExist) {
			logx.Fprintf(logger, "skipping %q (already exists)\n", filepath.ToSlash(path))

			return 0, nil
		}
	} else {
		err = os.Rename(temporary, path)
	}

	return writer.n, err
}

func skipOutput(path string, skipExisting bool, logger io.Writer) (bool, error) {
	if !skipExisting || path == "-" {
		return false, nil
	}

	_, err := os.Lstat(path)
	if err == nil {
		logx.Fprintf(logger, "skipping %q (already exists)\n", filepath.ToSlash(path))

		return true, nil
	}

	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}

	return false, err
}
