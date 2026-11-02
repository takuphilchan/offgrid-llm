package models

import (
	"os"
	"path"
)

// RetainedGGUFBundleBytes reports resumable storage, not verified installation.
// It uses the pinned digest-specific staging names and never follows symlinks.
func RetainedGGUFBundleBytes(dir, local string, b GGUFBundle) int64 {
	if b.Validate() != nil || !validArtifactPath(local) || path.Base(local) != local {
		return 0
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return 0
	}
	defer root.Close()
	var total int64
	for i, a := range b.Artifacts() {
		target := local
		if i == 1 {
			target = companionPath(local, b)
		}
		if checkedDirectory(root, path.Dir(target)) != nil {
			continue
		}
		for _, name := range []string{target, target + "." + a.SHA256[:16] + ".tmp"} {
			if identity, err := assetIdentity(root, name); err == nil {
				total += min(identity.Size, a.Size)
				break
			}
		}
	}
	return total
}
