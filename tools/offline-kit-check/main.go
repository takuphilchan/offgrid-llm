// offline-kit-check reuses the service's strict data-only package schema during
// media preparation. It reads manifests only: no imports, execution or downloads.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/takuphilchan/offgrid-llm/internal/models"
)

func check(root string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	if len(entries) == 0 || len(entries) > 512 {
		return fmt.Errorf("invalid speech package count")
	}
	identities := map[string]bool{}
	for _, entry := range entries {
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("speech package must be a regular directory")
		}
		name := filepath.Join(root, entry.Name(), "manifest.json")
		info, err := os.Lstat(name)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.Size() > models.MaxPackageManifestBytes {
			return fmt.Errorf("invalid model manifest file")
		}
		file, err := os.Open(name)
		if err != nil {
			return err
		}
		manifest, err := models.DecodePackageManifest(file)
		file.Close()
		if err != nil {
			return fmt.Errorf("package %s: %w", entry.Name(), err)
		}
		key := manifest.ID + "@" + manifest.Revision
		if identities[key] {
			return fmt.Errorf("duplicate package identity: %s", key)
		}
		identities[key] = true
	}
	return nil
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "Usage: offline-kit-check SPEECH_DIRECTORY")
		os.Exit(2)
	}
	if err := check(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
