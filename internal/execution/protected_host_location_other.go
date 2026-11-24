//go:build !windows && !darwin

package execution

import "path/filepath"

func canonicalPrivateLocation(path string) (string, error) {
	return filepath.EvalSymlinks(path)
}
