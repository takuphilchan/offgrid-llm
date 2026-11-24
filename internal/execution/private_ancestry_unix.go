//go:build linux || darwin

package execution

import (
	"os"
	"syscall"
)

func checkPrivateAncestor(path string, info os.FileInfo) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != 0 && stat.Uid != uint32(os.Geteuid()) {
		return ErrAuthority
	}
	return checkPrivateAncestorPermissions(path, info)
}

func checkPrivateAncestorPermissions(path string, info os.FileInfo) error {
	// On a trusted-owner sticky directory only that owner, the child owner or
	// privileged root may remove/rename a child. Child owners are checked while
	// walking upward; the private-leaf constructor validates newly created state.
	if info.Mode().Perm()&0022 != 0 && info.Mode()&os.ModeSticky == 0 {
		return ErrAuthority
	}
	return checkPrivateExtendedAuthority(path)
}
