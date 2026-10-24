package platform

import (
	"os"
	"path/filepath"
	"runtime"
)

// IsSystemPathAlias recognizes only macOS's fixed root-owned directory aliases.
// It does not permit arbitrary links in user configuration or workspace paths.
func IsSystemPathAlias(path string) bool {
	if runtime.GOOS != "darwin" || (path != "/var" && path != "/tmp" && path != "/etc") {
		return false
	}
	target, err := os.Readlink(path)
	if err != nil {
		return false
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(path), target)
	}
	return filepath.Clean(target) == "/private"+path
}
