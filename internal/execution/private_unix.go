//go:build linux || darwin

package execution

import (
	"os"
	"syscall"
)

func privateOwnedPath(path string) (os.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() && !info.IsDir() {
		return nil, ErrAuthority
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Geteuid()) {
		return nil, ErrAuthority
	}
	return info, nil
}

func ProtectPrivatePath(path string) error {
	info, err := privateOwnedPath(path)
	if err != nil {
		return err
	}
	mode := os.FileMode(0600)
	if info.IsDir() {
		mode = 0700
	}
	return os.Chmod(path, mode)
}
func CheckPrivatePath(path string) error {
	info, err := privateOwnedPath(path)
	if err != nil {
		return err
	}
	if info.Mode().Perm()&0077 != 0 {
		return ErrAuthority
	}
	return nil
}
