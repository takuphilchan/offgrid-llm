package execution

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func privateAncestryTestDirectory(t *testing.T) string {
	t.Helper()
	parent, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil || CheckPrivateAncestry(parent) != nil {
		parent, err = os.UserHomeDir()
		if err == nil {
			parent, err = filepath.EvalSymlinks(parent)
		}
	}
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckPrivateAncestry(parent); err != nil {
		t.Fatalf("no safe isolated fixture parent: %v", err)
	}
	directory, err := os.MkdirTemp(parent, ".offgrid-ancestry-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if filepath.Dir(directory) != parent || !strings.HasPrefix(filepath.Base(directory), ".offgrid-ancestry-test-") {
			t.Fatal("fixture cleanup escaped owner")
		}
		if err := os.RemoveAll(directory); err != nil {
			t.Error(err)
		}
	})
	if err := ProtectPrivatePath(directory); err != nil {
		t.Fatal(err)
	}
	return directory
}

func TestPrivateAncestryRejectsInvalidPathsAndLinks(t *testing.T) {
	for _, path := range []string{"", "relative", string([]byte{'/', 0})} {
		if CheckPrivateAncestry(path) == nil {
			t.Fatal("invalid ancestry accepted")
		}
	}
	directory := privateAncestryTestDirectory(t)
	if err := CheckPrivateAncestry(filepath.Join(directory, "missing", "leaf")); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(directory, "file")
	if err := os.WriteFile(file, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if CheckPrivateAncestry(file) == nil {
		t.Fatal("file ancestry accepted")
	}
	link := filepath.Join(directory, "alias")
	if err := os.Symlink(directory, link); err != nil {
		t.Skipf("symlink fixture unavailable: %v", err)
	}
	if CheckPrivateAncestry(filepath.Join(link, "future")) == nil {
		t.Fatal("linked ancestry accepted")
	}
}
