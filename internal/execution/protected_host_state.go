package execution

import (
	"os"
	"path/filepath"
	"strings"
)

// PrivateStateDisjointFromRoot rejects both granting a private directory and
// granting any of its descendants. Missing private leaves are resolved through
// their existing ancestors, so aliases cannot hide overlap before first use.
func PrivateStateDisjointFromRoot(root string, paths ...string) error {
	approved, err := canonicalPrivateLocation(root)
	if err != nil || !filepath.IsAbs(approved) {
		return ErrPath
	}
	for _, path := range paths {
		resolved, err := resolvePrivateLocation(path)
		if err != nil {
			return err
		}
		if containsPrivateLocation(approved, resolved) || containsPrivateLocation(resolved, approved) {
			return ErrAuthority
		}
	}
	return nil
}

func resolvePrivateLocation(path string) (string, error) {
	if !filepath.IsAbs(path) || strings.ContainsRune(path, 0) {
		return "", ErrPath
	}
	cursor := filepath.Clean(path)
	var suffix []string
	for {
		resolved, err := canonicalPrivateLocation(cursor)
		if err == nil {
			for i := len(suffix) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, suffix[i])
			}
			return resolved, nil
		}
		if !os.IsNotExist(err) {
			return "", ErrPath
		}
		if _, statErr := os.Lstat(cursor); statErr == nil || !os.IsNotExist(statErr) {
			return "", ErrPath
		}
		parent := filepath.Dir(cursor)
		if parent == cursor {
			return "", ErrPath
		}
		suffix = append(suffix, filepath.Base(cursor))
		cursor = parent
	}
}

func containsPrivateLocation(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// protectedHostDirectories is local supervisor policy, never model or renderer
// input. Always reserve the standard desktop browser state, including before a
// browser has started. Explicit isolated desktop profiles add their own root.
func protectedHostDirectories(extra []string) ([]string, error) {
	if len(extra) > 16 {
		return nil, ErrAuthority
	}
	home, err := os.UserHomeDir()
	if err != nil || !filepath.IsAbs(home) {
		return nil, ErrAuthority
	}
	paths := []string{filepath.Join(home, ".offgrid-llm", "computer")}
	if custom := os.Getenv("OFFGRID_DESKTOP_HOME"); custom != "" {
		if !filepath.IsAbs(custom) {
			return nil, ErrAuthority
		}
		paths = append(paths, filepath.Join(custom, "computer"))
	}
	for _, path := range extra {
		if !filepath.IsAbs(path) || len(path) > 32768 || strings.ContainsRune(path, 0) {
			return nil, ErrAuthority
		}
		paths = append(paths, path)
	}
	return paths, nil
}
