package server

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/storage"
)

// The service's existing workspace lease protects the default file. External
// owner files require an explicitly delegated, dedicated parent directory, whose
// existing OS Ownership lock also serializes other OffGrid workspaces. Trusted
// same-account/privileged filesystem mutation remains outside this boundary.
func acquireEndpointOwnerWriter(path, workspace, delegated string) (func() error, error) {
	if err := verifyEndpointOwnerWriter(path, workspace, delegated); err != nil {
		return nil, err
	}
	parent := filepath.Dir(path)
	before, err := os.Lstat(parent)
	if err != nil || !before.IsDir() {
		return nil, errors.New("endpoint owner parent is unavailable")
	}
	if filepath.Clean(parent) == filepath.Clean(workspace) {
		return func() error { return nil }, nil // Production Core already owns it.
	}
	owner, err := storage.AcquireOwnership(parent)
	if err != nil {
		return nil, err
	}
	after, err := os.Lstat(parent)
	if err != nil || !os.SameFile(before, after) || verifyEndpointOwnerPath(parent, true) != nil {
		owner.Close()
		return nil, errors.New("endpoint owner directory changed during writer admission")
	}
	return owner.Close, nil
}

func verifyEndpointOwnerWriter(path, workspace, delegated string) error {
	if !filepath.IsAbs(path) || !filepath.IsAbs(workspace) || strings.HasPrefix(filepath.ToSlash(path), "//") {
		return errors.New("endpoint owner file requires a local absolute path")
	}
	parent := filepath.Dir(path)
	defaultPath := filepath.Join(workspace, "inference-endpoints.json")
	if filepath.Clean(path) != defaultPath && filepath.Clean(path) != delegated {
		return errors.New("installation has not delegated this external owner file")
	}
	if err := verifyEndpointOwnerPath(parent, true); err != nil {
		return err
	}
	if err := verifyEndpointOwnerPath(path, false); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func verifyEndpointOwnerPath(path string, directory bool) error {
	for depth, current := 0, filepath.Clean(path); ; depth++ {
		if depth > 128 {
			return errors.New("endpoint owner ancestry exceeds the validation bound")
		}
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || (depth > 0 || directory) && !info.IsDir() || depth == 0 && !directory && !info.Mode().IsRegular() {
			return errors.New("endpoint owner path contains a link or unexpected file type")
		}
		if err := verifyEndpointOwnerSecurity(current, info, depth > 0); err != nil {
			return err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return nil
		}
		current = parent
	}
}
