//go:build !windows
// +build !windows

package models

import "golang.org/x/sys/unix"

// getDiskSpace returns available disk space in bytes for a given path
func getDiskSpace(path string) (int64, error) {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return 0, err
	}
	// Available blocks * size per block = available space in bytes
	return int64(stat.Bavail) * int64(stat.Bsize), nil
}
