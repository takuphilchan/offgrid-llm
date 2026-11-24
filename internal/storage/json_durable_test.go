package storage

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDurableSnapshotPublicationAndOwnedCleanup(t *testing.T) {
	for _, point := range []string{"ordinary", "before", "after", "recreated-source"} {
		t.Run(point, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "policy.json")
			if err := WriteDurableJSON(path, map[string]bool{"enabled": true}); err != nil {
				t.Fatal(err)
			}
			failure := errors.New("injected durability boundary")
			temporary := ""
			err := writeDurableJSON(path, map[string]bool{"enabled": false}, func(source, target string) (bool, error) {
				temporary = source
				if info, err := os.Stat(source); err != nil || !info.Mode().IsRegular() {
					t.Fatal("private source unavailable", err)
				}
				if point == "before" {
					return false, failure
				}
				moved, err := replaceDurableSnapshot(source, target)
				if err != nil {
					return moved, err
				}
				if point == "recreated-source" {
					if err := os.WriteFile(source, []byte("unrelated"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				if point == "after" {
					return moved, failure
				}
				return moved, nil
			})
			if (point == "before" || point == "after") != errors.Is(err, failure) {
				t.Fatal("publication error lost", err)
			}
			raw, readErr := os.ReadFile(path)
			expected := `{"enabled":false}`
			if point == "before" {
				expected = `{"enabled":true}`
			}
			if readErr != nil || string(raw) != expected {
				t.Fatal("unexpected published state", string(raw), readErr)
			}
			if point == "recreated-source" {
				raw, err := os.ReadFile(temporary)
				if err != nil || string(raw) != "unrelated" {
					t.Fatal("cleanup deleted a replacement", string(raw), err)
				}
			} else if _, err := os.Stat(temporary); !os.IsNotExist(err) {
				t.Fatal("temporary snapshot retained", err)
			}
		})
	}
}

func TestDurableSnapshotEncodingFailurePreservesExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "policy.json")
	if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := WriteDurableJSON(path, func() {}); err == nil {
		t.Fatal("unencodable state accepted")
	}
	raw, err := os.ReadFile(path)
	if err != nil || string(raw) != "original" {
		t.Fatal(string(raw), err)
	}
}
