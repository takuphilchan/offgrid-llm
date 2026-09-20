package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorkspaceIdentityPersistsAndIsolates(t *testing.T) {
	directory := t.TempDir()
	owner, err := AcquireOwnership(directory)
	if err != nil {
		t.Fatal(err)
	}
	id, err := owner.WorkspaceID()
	if err != nil || len(id) != 32 {
		t.Fatalf("identity: %q %v", id, err)
	}
	owner.Close()
	owner, err = AcquireOwnership(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	again, err := owner.WorkspaceID()
	if err != nil || again != id {
		t.Fatal("identity changed on restart", err)
	}
	other, err := AcquireOwnership(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	otherID, err := other.WorkspaceID()
	if err != nil || otherID == id {
		t.Fatal("workspaces share identity", err)
	}
	if err := os.WriteFile(filepath.Join(directory, "workspace-id"), []byte("bad"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.WorkspaceID(); err == nil {
		t.Fatal("corrupt identity silently replaced")
	}
}
