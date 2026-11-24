package execution

import (
	"os"
	"path/filepath"
	"strings"
)

// CheckPrivateAncestry checks existing directories before any private leaf is
// created. A private child alone does not stop another principal renaming an
// ancestor. Shared read/traverse access is permitted; mutation authority is not.
// It never repairs permissions and does not defend against the account itself
// or a privileged OS administrator changing the tree after validation.
func CheckPrivateAncestry(directory string) error {
	return checkPrivateAncestry(directory, checkPrivateAncestor)
}

func checkPrivateAncestry(directory string, check func(string, os.FileInfo) error) error {
	if !filepath.IsAbs(directory) || len(directory) > 32768 || strings.ContainsRune(directory, 0) {
		return ErrPath
	}
	for cursor := filepath.Clean(directory); ; cursor = filepath.Dir(cursor) {
		info, err := os.Lstat(cursor)
		if err == nil {
			if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				return ErrAuthority
			}
			if err := check(cursor, info); err != nil {
				return err
			}
		} else if !os.IsNotExist(err) {
			return err
		}
		if filepath.Dir(cursor) == cursor {
			if err != nil {
				return ErrPath
			}
			return nil
		}
	}
}
