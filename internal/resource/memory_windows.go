//go:build windows

package resource

import (
	"syscall"
	"unsafe"
)

type memoryStatusEx struct {
	Length                   uint32
	MemoryLoad               uint32
	TotalPhysical            uint64
	AvailablePhysical        uint64
	TotalPageFile            uint64
	AvailablePageFile        uint64
	TotalVirtual             uint64
	AvailableVirtual         uint64
	AvailableExtendedVirtual uint64
}

func detectNativeMemory(res *SystemResources) bool {
	status := memoryStatusEx{Length: uint32(unsafe.Sizeof(memoryStatusEx{}))}
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("GlobalMemoryStatusEx")
	result, _, _ := proc.Call(uintptr(unsafe.Pointer(&status)))
	if result == 0 || status.TotalPhysical == 0 {
		return false
	}
	res.TotalRAM = int64(status.TotalPhysical >> 20)
	res.AvailableRAM = int64(status.AvailablePhysical >> 20)
	res.AvailableCommitMB = int64(status.AvailablePageFile >> 20)
	res.CommitMeasured = true
	return true
}
