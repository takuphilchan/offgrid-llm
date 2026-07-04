//go:build windows
// +build windows

package models

import (
	"golang.org/x/sys/windows"
	"unsafe"
)

// getDiskSpace returns available disk space in bytes for a given path (Windows)
func getDiskSpace(path string) (int64, error) {
	h := windows.MustLoadDLL("kernel32.dll")
	c := h.MustFindProc("GetDiskFreeSpaceExW")

	var freeBytes int64
	var totalBytes int64
	var availBytes int64

	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}

	ret, _, err := c.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(unsafe.Pointer(&freeBytes)),
		uintptr(unsafe.Pointer(&totalBytes)),
		uintptr(unsafe.Pointer(&availBytes)),
	)

	if ret == 0 {
		return 0, err
	}

	return availBytes, nil
}
