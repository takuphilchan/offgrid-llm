package server

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareDataLayoutCopiesLegacyStateWithoutOverwriting(t *testing.T) {
	root := t.TempDir()
	modelsDir := filepath.Join(root, "legacy", "models")
	dataDir := filepath.Join(root, "current")
	if err := os.MkdirAll(filepath.Join(modelsDir, "rag"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "legacy", "sessions"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modelsDir, "rag", "rag.db"), []byte("legacy-rag"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "legacy", "sessions", "chat.json"), []byte("legacy-session"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "sessions"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "sessions", "chat.json"), []byte("current-session"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := prepareDataLayout(modelsDir, dataDir); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(filepath.Join(dataDir, "sessions", "chat.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "current-session" {
		t.Fatalf("existing state was overwritten: %q", raw)
	}
	raw, err = os.ReadFile(filepath.Join(dataDir, "rag", "rag.db"))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "legacy-rag" {
		t.Fatalf("legacy RAG state was not copied: %q", raw)
	}
	if _, err := os.Stat(filepath.Join(dataDir, ".layout-version")); err != nil {
		t.Fatalf("layout marker missing: %v", err)
	}
}

func TestPrepareDataLayoutRejectsEmptyDataDirectory(t *testing.T) {
	if err := prepareDataLayout(t.TempDir(), ""); err == nil {
		t.Fatal("expected an error for an empty data directory")
	}
}
