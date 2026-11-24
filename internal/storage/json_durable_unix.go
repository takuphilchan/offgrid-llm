//go:build linux || darwin

package storage

import "path/filepath"

func replaceDurableSnapshot(source, target string) (bool, error) {
	if err := replaceSnapshot(source, target); err != nil {
		return false, err
	}
	return true, syncBackupParent(filepath.Dir(target))
}
