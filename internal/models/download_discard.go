package models

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
)

// DiscardGGUFPartial removes only the exact temporary artifacts associated with
// this download. Published weights, projectors and receipts are never removed.
// The caller must hold the download admission lock until this returns.
func DiscardGGUFPartial(dir, local string, bundle *GGUFBundle) error {
	if !validArtifactPath(local) || path.Base(local) != local || filepath.Base(local) != local {
		return fmt.Errorf("invalid download filename")
	}
	names := []string{local + ".tmp"}
	if bundle != nil {
		if err := bundle.Validate(); err != nil {
			return err
		}
		for i, artifact := range bundle.Artifacts() {
			target := local
			if i == 1 {
				target = companionPath(local, *bundle)
			}
			names = append(names, target+"."+artifact.SHA256[:16]+".tmp")
		}
	}
	root, err := os.OpenRoot(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer root.Close()
	// Validate every entry before removing any. Root-relative operations prevent
	// traversal even if another local process changes a directory concurrently.
	for _, name := range names {
		if err := discardPartialDirectory(root, path.Dir(name)); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return err
		}
		info, err := root.Lstat(name)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("download partial is not a regular file")
		}
	}
	for _, name := range names {
		if err := root.Remove(name); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return nil
}

func discardPartialDirectory(root *os.Root, name string) error {
	for dir := name; dir != "."; dir = path.Dir(dir) {
		info, err := root.Lstat(dir)
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("download directory is replaced")
		}
	}
	return nil
}
