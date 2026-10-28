package execution

import (
	"os"
	"path/filepath"
	"strings"
)

// PrivateStateOutsideRoot resolves every existing path prefix before comparing
// it with the workspace. Missing leaf directories must not hide a link back
// into files that the model can read or replace.
func PrivateStateOutsideRoot(root string, paths ...string) error {
	approved, err := filepath.EvalSymlinks(root)
	if err != nil || !filepath.IsAbs(approved) {
		return ErrPath
	}
	for _, path := range paths {
		if !filepath.IsAbs(path) {
			return ErrPath
		}
		cursor := filepath.Clean(path)
		var suffix []string
		for {
			resolved, resolveErr := filepath.EvalSymlinks(cursor)
			if resolveErr == nil {
				for i := len(suffix) - 1; i >= 0; i-- {
					resolved = filepath.Join(resolved, suffix[i])
				}
				rel, relErr := filepath.Rel(approved, resolved)
				if relErr == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel) {
					return ErrAuthority
				}
				break
			}
			if !os.IsNotExist(resolveErr) {
				return ErrPath
			}
			parent := filepath.Dir(cursor)
			if parent == cursor {
				return ErrPath
			}
			suffix = append(suffix, filepath.Base(cursor))
			cursor = parent
		}
	}
	return nil
}

// EnsurePrivateDirectory creates only the requested leaf. Existing state must
// already be account-private; never silently change permissions on an arbitrary
// caller-selected directory. Parent creation is the trusted supervisor's job.
func EnsurePrivateDirectory(path string) error {
	if !filepath.IsAbs(path) {
		return ErrPath
	}
	if err := os.Mkdir(path, 0700); err != nil {
		if !os.IsExist(err) {
			return err
		}
	} else if err := ProtectPrivatePath(path); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return ErrAuthority
	}
	return CheckPrivatePath(path)
}
