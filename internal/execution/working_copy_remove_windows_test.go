package execution

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorkingCopyWindowsDispositionPreservesReplacement(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "entry")
	if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	expected, err := root.Lstat("entry")
	if err != nil {
		t.Fatal(err)
	}
	file, err := openWorkingCopyWindows(root, "entry", false, true)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err = os.Rename(path, filepath.Join(directory, "renamed-original")); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, []byte("replacement"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = disposeCopyHandle(file, expected); err != nil {
		t.Fatal(err)
	}
	if err = file.Close(); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "replacement" {
		t.Fatal("handle disposition deleted replacement", string(data), err)
	}
	if _, err = os.Stat(filepath.Join(directory, "renamed-original")); !os.IsNotExist(err) {
		t.Fatal("original handle was not disposed", err)
	}
}

func TestWorkingCopyWindowsDispositionExcludesWriters(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "entry")
	if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	expected, err := root.Lstat("entry")
	if err != nil {
		t.Fatal(err)
	}
	writer, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err = removeCopyEntry(root, "entry", expected); err == nil {
		t.Fatal("disposed an actively writable file")
	}
	writer.Close()
	file, err := openWorkingCopyWindows(root, "entry", false, true)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if writer, err = os.OpenFile(path, os.O_WRONLY, 0); err == nil {
		writer.Close()
		t.Fatal("new writer entered disposition interval")
	}
	if err = disposeCopyHandle(file, expected); err != nil {
		t.Fatal(err)
	}
}
