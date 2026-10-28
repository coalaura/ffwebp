package main

import (
	"os"

	"golang.org/x/sys/unix"
)

func renameOutputExclusive(source, destination string) error {
	err := unix.Renameat2(unix.AT_FDCWD, source, unix.AT_FDCWD, destination, unix.RENAME_NOREPLACE)
	if err != nil {
		return &os.LinkError{Op: "rename", Old: source, New: destination, Err: err}
	}

	return nil
}
