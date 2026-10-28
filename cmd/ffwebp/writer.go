package main

import (
	"crypto/rand"
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

func writeOutput(path string, skipExisting bool, logger io.Writer, encode func(io.Writer) error) (written int64, resultErr error) {
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

	info, err := os.Stat(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return 0, err
	}

	mode := os.FileMode(0644)

	if info != nil {
		mode = info.Mode().Perm()
	}

	err = os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		return 0, err
	}

	file, err := createOutputTemp(filepath.Dir(path), mode)
	if err != nil {
		return 0, err
	}

	temporary := file.Name()

	defer func() {
		err := os.Remove(temporary)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			resultErr = errors.Join(resultErr, err)
		}
	}()

	defer file.Close()

	logx.Fprintf(logger, "writing output file %q\n", filepath.ToSlash(path))

	writer := &countWriter{w: file}

	encodeErr := encode(writer)
	closeErr := file.Close()

	err = errors.Join(encodeErr, closeErr)
	if err != nil {
		return writer.n, err
	}

	if info != nil {
		err = os.Chmod(temporary, info.Mode().Perm())
		if err != nil {
			return writer.n, err
		}
	}

	if skipExisting {
		err = publishOutput(temporary, path)
		if errors.Is(err, os.ErrExist) {
			logx.Fprintf(logger, "skipping %q (already exists)\n", filepath.ToSlash(path))

			return 0, nil
		}
	} else {
		err = os.Rename(temporary, path)
	}

	return writer.n, err
}

func createOutputTemp(directory string, mode os.FileMode) (*os.File, error) {
	for {
		path := filepath.Join(directory, ".ffwebp-"+rand.Text())

		// OpenFile applies the process umask without changing it, unlike Chmod.
		file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, mode)
		if errors.Is(err, os.ErrExist) {
			continue
		}

		return file, err
	}
}

func publishOutput(temporary, path string) error {
	err := renameOutputExclusive(temporary, path)
	if err == nil || errors.Is(err, os.ErrExist) {
		return err
	}

	// A hard link is also atomic, but is only an optimization, not a requirement.
	linkErr := os.Link(temporary, path)
	if linkErr == nil || errors.Is(linkErr, os.ErrExist) {
		return linkErr
	}

	return copyOutputExclusive(temporary, path)
}

func copyOutputExclusive(temporary, path string) error {
	source, err := os.Open(temporary)
	if err != nil {
		return err
	}

	defer source.Close()

	info, err := source.Stat()
	if err != nil {
		return err
	}

	// O_EXCL preserves no-overwrite semantics on filesystems without atomic
	// publication. Readers can see a partial file until this copy completes.
	destination, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		return err
	}

	created, statErr := destination.Stat()
	_, copyErr := io.Copy(destination, source)
	closeErr := destination.Close()

	err = errors.Join(statErr, copyErr, closeErr)
	if err != nil {
		current, checkErr := os.Lstat(path)
		if checkErr == nil && created != nil && os.SameFile(created, current) {
			err = errors.Join(err, os.Remove(path))
		}
	}

	return err
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
