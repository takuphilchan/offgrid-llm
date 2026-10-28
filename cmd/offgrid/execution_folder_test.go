package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLaunchFolderCannotFollowReplacement(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "workspace")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	captured, check, err := captureNativeLaunchFolder(root)
	canonical, canonicalErr := filepath.EvalSymlinks(root)
	if err != nil || canonicalErr != nil || captured != canonical || check() != nil {
		t.Fatal(captured, err)
	}
	if err := os.Rename(root, filepath.Join(parent, "original")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	if check() == nil {
		t.Fatal("replaced folder accepted")
	}
}
