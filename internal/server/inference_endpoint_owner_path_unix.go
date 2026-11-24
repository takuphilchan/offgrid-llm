//go:build !windows

package server

import (
	"errors"
	"os"
	"syscall"
)

func verifyEndpointOwnerSecurity(_ string, info os.FileInfo, ancestor bool) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Geteuid()) && stat.Uid != 0 || info.Mode().IsRegular() && stat.Nlink != 1 {
		return errors.New("endpoint owner path has untrusted ownership or hardlinks")
	}
	if info.Mode().Perm()&0022 != 0 {
		// Root-owned sticky ancestors such as /tmp cannot let another account
		// replace the service-owned private child directory.
		if !ancestor || !info.IsDir() || stat.Uid != 0 || info.Mode()&os.ModeSticky == 0 {
			return errors.New("endpoint owner path is writable by another account")
		}
	}
	return nil
}
