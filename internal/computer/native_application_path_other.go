//go:build !windows

package computer

import (
	"path/filepath"
	"strings"
)

func validateInstalledApplicationPath(path string) error {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil || !strings.EqualFold(filepath.Clean(resolved), filepath.Clean(path)) {
		return ErrControlScope
	}
	return nil
}
