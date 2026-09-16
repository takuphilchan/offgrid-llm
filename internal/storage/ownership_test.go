package storage

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestOwnershipExclusiveAndReusable(t *testing.T) {
	dir := t.TempDir()
	owner, err := AcquireOwnership(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { owner.Close() })
	if second, err := AcquireOwnership(filepath.Join(dir, ".")); !errors.Is(err, ErrWorkspaceInUse) {
		if second != nil {
			second.Close()
		}
		t.Fatalf("second owner: %v", err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := AcquireOwnership(dir)
	if err != nil {
		t.Fatal(err)
	}
	second.Close()
}

func TestOwnershipProcessHelper(t *testing.T) {
	dir := os.Getenv("OFFGRID_LOCK_TEST_DIR")
	if dir == "" {
		return
	}
	owner, err := AcquireOwnership(dir)
	if os.Getenv("OFFGRID_LOCK_TEST_MODE") == "blocked" {
		if !errors.Is(err, ErrWorkspaceInUse) {
			os.Exit(12)
		}
		os.Exit(0)
	}
	if err != nil {
		os.Exit(13)
	}
	_ = owner
	// Deliberately exit without cleanup; the kernel must release the lock.
	os.Exit(0)
}

func TestOwnershipAcrossProcessesAndCrash(t *testing.T) {
	dir := t.TempDir()
	owner, err := AcquireOwnership(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { owner.Close() })
	run := func(mode string) {
		t.Helper()
		cmd := exec.Command(os.Args[0], "-test.run=^TestOwnershipProcessHelper$")
		cmd.Env = append(os.Environ(), "OFFGRID_LOCK_TEST_DIR="+dir, "OFFGRID_LOCK_TEST_MODE="+mode)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s: %v %s", mode, err, out)
		}
	}
	run("blocked")
	owner.Close()
	run("crash")
	next, err := AcquireOwnership(dir)
	if err != nil {
		t.Fatal(err)
	}
	next.Close()
}
