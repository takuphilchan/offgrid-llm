//go:build windows
// +build windows

package models

import (
	"fmt"
	"golang.org/x/sys/windows"
	"unsafe"
)

// formatBytes formats bytes as human-readable string
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

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
