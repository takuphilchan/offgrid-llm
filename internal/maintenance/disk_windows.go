//go:build windows
// +build windows

package maintenance

import (
	"syscall"
	"unsafe"
)

func getDiskSpaceInfo(path string) (DiskSpaceInfo, error) {
	kernel32 := syscall.MustLoadDLL("kernel32.dll")
	getDiskFreeSpace := kernel32.MustFindProc("GetDiskFreeSpaceExW")

	var freeBytesAvailable, totalBytes, totalFreeBytes int64

	pathPtr, _ := syscall.UTF16PtrFromString(path)

	ret, _, err := getDiskFreeSpace.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(unsafe.Pointer(&freeBytesAvailable)),
		uintptr(unsafe.Pointer(&totalBytes)),
		uintptr(unsafe.Pointer(&totalFreeBytes)),
	)

	if ret == 0 {
		return DiskSpaceInfo{}, err
	}

	used := totalBytes - totalFreeBytes

	return DiskSpaceInfo{
		TotalGB:     float64(totalBytes) / (1024 * 1024 * 1024),
		FreeGB:      float64(totalFreeBytes) / (1024 * 1024 * 1024),
		UsedGB:      float64(used) / (1024 * 1024 * 1024),
		UsedPercent: float64(used) / float64(totalBytes) * 100,
	}, nil
}
