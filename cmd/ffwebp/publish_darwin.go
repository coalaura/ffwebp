package main

import (
	"os"

	"golang.org/x/sys/unix"
)

func renameOutputExclusive(source, destination string) error {
	err := unix.RenamexNp(source, destination, unix.RENAME_EXCL)
	if err != nil {
		return &os.LinkError{Op: "rename", Old: source, New: destination, Err: err}
	}

	return nil
}
