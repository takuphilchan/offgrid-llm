package storage

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestExistingOwnershipDoesNotCreateAndPreservesLock(t *testing.T) {
	parent := t.TempDir()
	missing := filepath.Join(parent, "missing")
	if _, err := AcquireExistingOwnership(missing); !os.IsNotExist(err) {
		t.Fatalf("missing existing lock: %v", err)
	}
	entries, err := os.ReadDir(parent)
	if err != nil || len(entries) != 0 {
		t.Fatalf("read-only ownership created state: %v %v", entries, err)
	}
	if _, err = AcquireExistingOwnership(parent); !os.IsNotExist(err) {
		t.Fatalf("missing lock not distinguished: %v", err)
	}
	owner, err := AcquireOwnership(parent)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	if _, err = AcquireExistingOwnership(parent); !errors.Is(err, ErrWorkspaceInUse) {
		t.Fatalf("concurrent owner ignored: %v", err)
	}
	if err = owner.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(parent, ".offgrid-owner.lock")
	before, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	inspector, err := AcquireExistingOwnership(parent)
	if err != nil {
		t.Fatal(err)
	}
	defer inspector.Close()
	if _, err = AcquireOwnership(parent); !errors.Is(err, ErrWorkspaceInUse) {
		t.Fatalf("inspector did not hold ordinary lock: %v", err)
	}
	if err = inspector.Close(); err != nil {
		t.Fatal(err)
	}
	if err = inspector.Close(); err != nil {
		t.Fatalf("close not idempotent: %v", err)
	}
	after, err := os.Lstat(path)
	if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() || before.Size() != after.Size() {
		t.Fatalf("lock changed: %v", err)
	}
}
