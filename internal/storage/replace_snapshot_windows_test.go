//go:build windows

package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSnapshotReplacementSettlesAfterWindowsReaderCloses(t *testing.T) {
	p := filepath.Join(t.TempDir(), "snapshot.json")
	if err := WriteJSON(p, map[string]int{"old": 1}); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- WriteJSON(p, map[string]int{"new": 2}) }()
	select {
	case err := <-done:
		f.Close()
		t.Fatalf("replacement did not wait for reader: %v", err)
	case <-time.After(60 * time.Millisecond):
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	if err = <-done; err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p)
	if err != nil || string(b) != `{"new":2}` {
		t.Fatalf("snapshot: %s %v", b, err)
	}
}

func TestSnapshotReplacementPreservesOriginalOnPersistentLock(t *testing.T) {
	p := filepath.Join(t.TempDir(), "snapshot.json")
	if err := WriteJSON(p, map[string]int{"old": 1}); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	if err = WriteJSON(p, map[string]int{"new": 2}); err == nil {
		f.Close()
		t.Fatal("persistent lock ignored")
	}
	f.Close()
	b, err := os.ReadFile(p)
	if err != nil || string(b) != `{"old":1}` {
		t.Fatalf("old snapshot lost: %s %v", b, err)
	}
	files, _ := os.ReadDir(filepath.Dir(p))
	if len(files) != 1 {
		t.Fatal("temporary snapshot leaked")
	}
}
