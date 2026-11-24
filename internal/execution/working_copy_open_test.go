package execution

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWorkingCopyOpenAndReplacementSyncRejectSubstitutedIdentity(t *testing.T) {
	directory := t.TempDir()
	name := filepath.Join(directory, "input")
	if err := os.WriteFile(name, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	_, expected, err := observeReplacementFile(context.Background(), root, "input", 1024)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(name, filepath.Join(directory, "retained")); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(name, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.Chtimes(name, expected.ModTime(), expected.ModTime()); err != nil {
		t.Fatal(err)
	}
	if file, err := openWorkingCopyFile(root, "input", expected); !errors.Is(err, ErrRevision) {
		if file != nil {
			file.Close()
		}
		t.Fatal("same-size, same-time replacement accepted", err)
	}
	if err = syncReplacementFile(root, "input", expected); !errors.Is(err, ErrRevision) {
		t.Fatal("sync forgot observed object identity", err)
	}
	for _, name := range []string{"input", "retained"} {
		if content, err := root.ReadFile(name); err != nil || string(content) != "original" {
			t.Fatal("uncertain sync modified or removed preserved content", name, string(content), err)
		}
	}
}

func TestWorkingCopyOpenRejectsLinksToSameObservedObject(t *testing.T) {
	for _, parent := range []bool{false, true} {
		t.Run(map[bool]string{false: "leaf", true: "ancestor"}[parent], func(t *testing.T) {
			directory := t.TempDir()
			if err := os.Mkdir(filepath.Join(directory, "folder"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(directory, "folder", "input"), []byte("retained"), 0600); err != nil {
				t.Fatal(err)
			}
			root, err := os.OpenRoot(directory)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			expected, err := root.Lstat("folder/input")
			if err != nil {
				t.Fatal(err)
			}
			from, to := filepath.Join(directory, "folder", "input"), filepath.Join(directory, "folder", "original")
			if parent {
				from, to = filepath.Join(directory, "folder"), filepath.Join(directory, "original")
			}
			if err = os.Rename(from, to); err != nil {
				t.Fatal(err)
			}
			relative, err := filepath.Rel(filepath.Dir(from), to)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.Symlink(relative, from); err != nil {
				t.Skip("isolated symlink creation unavailable:", err)
			}
			if file, err := openWorkingCopyFile(root, "folder/input", expected); err == nil {
				file.Close()
				t.Fatal("followed an in-root link to the same observed file")
			}
			if err = syncReplacementFile(root, "folder/input", expected); err == nil {
				t.Fatal("sync followed an in-root link")
			}
		})
	}
}

func TestWorkingCopyWalkRejectsDirectoryReplacedDuringVisit(t *testing.T) {
	directory := t.TempDir()
	if err := os.Mkdir(filepath.Join(directory, "folder"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "folder", "input"), []byte("retained"), 0600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	visited := 0
	err = walkWorkingCopy(context.Background(), root, 5, func(name string, _ os.FileInfo) error {
		visited++
		if name != "folder" {
			t.Fatal("descended into substituted directory", name)
		}
		if err := os.Rename(filepath.Join(directory, "folder"), filepath.Join(directory, "retained")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("retained", filepath.Join(directory, "folder")); err != nil {
			t.Skip("isolated symlink creation unavailable:", err)
		}
		return nil
	})
	if err == nil || visited != 1 {
		t.Fatal("directory substitution accepted", visited, err)
	}
	if data, err := root.ReadFile("retained/input"); err != nil || string(data) != "retained" {
		t.Fatal("rejected traversal changed preserved content", string(data), err)
	}
}

func TestWorkingCopyWalkRejectsParentSwapAfterEnumeration(t *testing.T) {
	directory := t.TempDir()
	for name, content := range map[string]string{"a/x": "1234", "a/y": "5678", "b/y": ""} {
		full := filepath.Join(directory, name)
		if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	var measured int64
	err = walkWorkingCopy(context.Background(), root, 10, func(name string, info os.FileInfo) error {
		if info.Mode().IsRegular() {
			measured += info.Size()
		}
		if name == "a/x" {
			if err := os.Rename(filepath.Join(directory, "a"), filepath.Join(directory, "retained")); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("b", filepath.Join(directory, "a")); err != nil {
				t.Skip("isolated symlink creation unavailable:", err)
			}
		}
		return nil
	})
	if err == nil {
		t.Fatalf("parent substitution accepted an inventory of %d bytes", measured)
	}
	if data, err := root.ReadFile("retained/y"); err != nil || string(data) != "5678" {
		t.Fatal("failed observation modified original content", string(data), err)
	}
}
