package main

import (
	"os"

	"golang.org/x/sys/windows"
)

func renameOutputExclusive(source, destination string) error {
	sourcePointer, err := windows.UTF16PtrFromString(source)
	if err != nil {
		return err
	}

	destinationPointer, err := windows.UTF16PtrFromString(destination)
	if err != nil {
		return err
	}

	// Omitting MOVEFILE_REPLACE_EXISTING makes publication fail if occupied.
	err = windows.MoveFileEx(sourcePointer, destinationPointer, 0)
	if err != nil {
		return &os.LinkError{Op: "rename", Old: source, New: destination, Err: err}
	}

	return nil
}
