package models

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanModelsReadOnlyDoesNotCreateMissingDirectory(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(root, "missing-models")
	if err := NewRegistry(missing).ScanModelsReadOnly(); !os.IsNotExist(err) {
		t.Fatalf("missing directory error=%v, want not-exist", err)
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("read-only scan created directory: %v", err)
	}
	if err := os.Mkdir(missing, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(missing, "fixture.gguf"), []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry(missing)
	if err := registry.ScanModelsReadOnly(); err != nil {
		t.Fatal(err)
	}
	if got := len(registry.ListModels()); got != 1 {
		t.Fatalf("model count=%d, want 1", got)
	}
}
