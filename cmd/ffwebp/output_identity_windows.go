package main

import (
	"os"

	"golang.org/x/sys/windows"
)

func identifyFile(path string, _ os.FileInfo) (fileKey, bool) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return fileKey{}, false
	}

	// Metadata-only access also works for directories and read-protected files.
	share := uint32(windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE | windows.FILE_SHARE_DELETE)

	handle, err := windows.CreateFile(name, 0, share, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return fileKey{}, false
	}

	defer windows.CloseHandle(handle)

	var info windows.ByHandleFileInformation

	err = windows.GetFileInformationByHandle(handle, &info)
	if err != nil {
		return fileKey{}, false
	}

	return fileKey{device: uint64(info.VolumeSerialNumber), inode: uint64(info.FileIndexHigh)<<32 | uint64(info.FileIndexLow)}, true
}
