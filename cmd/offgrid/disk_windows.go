//go:build windows
// +build windows

package main

import (
	"syscall"
	"unsafe"
)

var (
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	getDiskFreeSpace = kernel32.NewProc("GetDiskFreeSpaceExW")
)

// getDiskSpace returns disk space information for Windows
func getDiskSpace(path string) (*diskSpaceInfo, error) {
	// Convert path to UTF16 pointer
	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}

	var freeBytesAvailable, totalBytes, totalFreeBytes uint64

	// Call GetDiskFreeSpaceEx
	r1, _, err := getDiskFreeSpace.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(unsafe.Pointer(&freeBytesAvailable)),
		uintptr(unsafe.Pointer(&totalBytes)),
		uintptr(unsafe.Pointer(&totalFreeBytes)),
	)

	if r1 == 0 {
		return nil, err
	}

	used := int64(totalBytes - freeBytesAvailable)
	usedPercent := 0.0
	if totalBytes > 0 {
		usedPercent = float64(used) / float64(totalBytes) * 100
	}

	return &diskSpaceInfo{
		Total:       int64(totalBytes),
		Available:   int64(freeBytesAvailable),
		UsedPercent: usedPercent,
	}, nil
}
