package execution

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrivateGrantAndJournalCannotBeInsideWorkspace(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{root, filepath.Join(root, "grant.json"), filepath.Join(root, "new", "journal")} {
		if PrivateStateOutsideRoot(root, path) == nil {
			t.Fatal("private state exposed", path)
		}
	}
	if err := PrivateStateOutsideRoot(root, filepath.Join(t.TempDir(), "new", "journal")); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	link := filepath.Join(outside, "into-workspace")
	if err := os.Symlink(root, link); err == nil {
		if PrivateStateOutsideRoot(root, filepath.Join(link, "journal")) == nil {
			t.Fatal("linked private state exposed")
		}
	}
}

func TestPrivateDirectoryDoesNotRepairUntrustedExistingState(t *testing.T) {
	parent := t.TempDir()
	path := filepath.Join(parent, "private")
	if err := EnsurePrivateDirectory(path); err != nil {
		t.Fatal(err)
	}
	if err := CheckPrivatePath(path); err != nil {
		t.Fatal(err)
	}
	if err := EnsurePrivateDirectory(path); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(parent, "file")
	if err := os.WriteFile(file, []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	if EnsurePrivateDirectory(file) == nil {
		t.Fatal("file treated as journal directory")
	}
	if data, _ := os.ReadFile(file); string(data) != "preserve" {
		t.Fatal("unrelated file changed")
	}
	link := filepath.Join(parent, "link")
	if err := os.Symlink(path, link); err == nil && EnsurePrivateDirectory(link) == nil {
		t.Fatal("linked journal accepted")
	}
	if EnsurePrivateDirectory("relative") == nil {
		t.Fatal("implicit cwd used for private state")
	}
}
