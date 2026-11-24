package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/models"
)

func TestKitCheckUsesCanonicalManifestValidation(t *testing.T) {
	root := t.TempDir()
	resolutions, err := models.CuratedPackageResolutions()
	if err != nil {
		t.Fatal(err)
	}
	manifest := resolutions[0].Manifest
	dir := filepath.Join(root, "recognition")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	write := func() {
		t.Helper()
		data, err := json.Marshal(manifest)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write()
	if err := check(root); err != nil {
		t.Fatal(err)
	}
	manifest.Runtime.Adapter = "unreviewed-executable"
	write()
	if err := check(root); err == nil {
		t.Fatal("invalid runtime manifest accepted")
	}
}
