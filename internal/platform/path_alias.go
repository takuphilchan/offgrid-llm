package platform

import (
	"os"
	"runtime"
)

// IsSystemPathAlias recognizes only macOS's fixed root-owned directory aliases.
// It does not permit arbitrary links in user configuration or workspace paths.
func IsSystemPathAlias(path string) bool {
	if runtime.GOOS != "darwin" || (path != "/var" && path != "/tmp" && path != "/etc") {
		return false
	}
	target, err := os.Readlink(path)
	return err == nil && target == "/private"+path
}
