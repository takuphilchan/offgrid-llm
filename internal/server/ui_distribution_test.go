package server

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPackagedUIDiscoveryAndOverride(t *testing.T) {
	root := t.TempDir()
	packaged := filepath.Join(root, "web", "ui")
	configured := filepath.Join(root, "configured")
	for _, dir := range []string{packaged, configured} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	executable := filepath.Join(root, "offgrid.exe")
	t.Setenv("OFFGRID_UI_DIR", "")
	if got := resolveUIRootForExecutable(executable); got != packaged {
		t.Fatalf("got %q, want %q", got, packaged)
	}
	t.Setenv("OFFGRID_UI_DIR", configured)
	if got := resolveUIRootForExecutable(executable); got != configured {
		t.Fatalf("explicit UI override lost: %q", got)
	}
	if err := os.Remove(filepath.Join(configured, "index.html")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(configured, "index.html"), 0700); err != nil {
		t.Fatal(err)
	}
	if got := resolveUIRootForExecutable(executable); got != packaged {
		t.Fatalf("directory accepted as UI index: %q", got)
	}
}
