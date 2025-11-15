//go:build linux || darwin || freebsd || openbsd || netbsd
// +build linux darwin freebsd openbsd netbsd

package main

import (
	"syscall"
)

// getDiskSpace returns disk space information for Unix-like systems
func getDiskSpace(path string) (*diskSpaceInfo, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return nil, err
	}

	available := int64(stat.Bavail * uint64(stat.Bsize))
	total := int64(stat.Blocks * uint64(stat.Bsize))
	used := total - available
	usedPercent := 0.0
	if total > 0 {
		usedPercent = float64(used) / float64(total) * 100
	}

	return &diskSpaceInfo{
		Total:       total,
		Available:   available,
		UsedPercent: usedPercent,
	}, nil
}
