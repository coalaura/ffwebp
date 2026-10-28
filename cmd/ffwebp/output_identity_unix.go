//go:build linux || darwin

package main

import (
	"os"
	"syscall"
)

func identifyFile(_ string, info os.FileInfo) (fileKey, bool) {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return fileKey{device: uint64(stat.Dev), inode: uint64(stat.Ino)}, true
	}

	return fileKey{}, false
}
