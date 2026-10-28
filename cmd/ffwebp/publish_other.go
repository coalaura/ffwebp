//go:build !darwin && !linux && !windows

package main

import "errors"

func renameOutputExclusive(_, _ string) error {
	return errors.ErrUnsupported
}
