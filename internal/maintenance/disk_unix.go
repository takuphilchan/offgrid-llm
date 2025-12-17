//go:build !windows
// +build !windows

package maintenance

import (
	"syscall"
)

func getDiskSpaceInfo(path string) (DiskSpaceInfo, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return DiskSpaceInfo{}, err
	}

	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bavail * uint64(stat.Bsize)
	used := total - free

	return DiskSpaceInfo{
		TotalGB:     float64(total) / (1024 * 1024 * 1024),
		FreeGB:      float64(free) / (1024 * 1024 * 1024),
		UsedGB:      float64(used) / (1024 * 1024 * 1024),
		UsedPercent: float64(used) / float64(total) * 100,
	}, nil
}
