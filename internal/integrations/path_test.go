package integrations

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrivateStateAllowsNativeTemporaryRootButRejectsUserLink(t *testing.T) {
	root := t.TempDir()
	if err := privatePath(filepath.Join(root, "new-state"), true); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "user-link")
	if err := os.Symlink(filepath.Join(root, "new-state"), link); err != nil {
		t.Skipf("symlink fixture unavailable: %v", err)
	}
	if err := privatePath(filepath.Join(link, "connection.json"), false); err == nil {
		t.Fatal("accepted a user-created symlink")
	}
}
