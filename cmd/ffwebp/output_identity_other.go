//go:build !linux && !darwin && !windows

package main

import "os"

func identifyFile(_ string, _ os.FileInfo) (fileKey, bool) {
	return fileKey{}, false
}
