package models

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func fixturePackageSource(_ context.Context, a PackageArtifact) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(a.Role)), nil
}

func TestPackageStoreLifecycle(t *testing.T) {
	store := NewPackageStore(t.TempDir())
	ctx := context.Background()
	m := testPackageManifest("piper")
	state, err := store.Import(ctx, m, fixturePackageSource)
	if err != nil {
		t.Fatal(err)
	}
	if !state.Installed || state.Integrity != "checked" || state.RuntimeCompatible || state.Qualified || state.SmokeTested || state.Provenance != "local_untrusted" {
		t.Fatalf("incorrect readiness: %+v", state)
	}
	if _, err := store.Import(ctx, m, fixturePackageSource); !errors.Is(err, ErrPackageConflict) {
		t.Fatalf("immutable conflict: %v", err)
	}
	items, err := store.List()
	if err != nil || len(items) != 1 {
		t.Fatalf("list: %v %v", items, err)
	}
	_, release, err := store.Acquire(ctx, m.ID, m.Revision, "test-job")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Remove(m.ID, m.Revision); !errors.Is(err, ErrPackageInUse) {
		t.Fatalf("deleted leased model: %v", err)
	}
	release()
	release()
	if err := store.Remove(m.ID, m.Revision); err != nil {
		t.Fatal(err)
	}
	items, err = store.List()
	if err != nil || len(items) != 0 {
		t.Fatalf("list after removal: %v %v", items, err)
	}
}

func TestPackageStoreRejectsCorruptionAndCancellation(t *testing.T) {
	for _, mode := range []string{"short", "long", "digest", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			store := NewPackageStore(t.TempDir())
			m := testPackageManifest("whisper")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			source := func(_ context.Context, a PackageArtifact) (io.ReadCloser, error) {
				data := a.Role
				switch mode {
				case "short":
					data = ""
				case "long":
					data += "!"
				case "digest":
					data = strings.Repeat("x", len(data))
				case "cancel":
					cancel()
				}
				return io.NopCloser(strings.NewReader(data)), nil
			}
			if _, err := store.Import(ctx, m, source); err == nil {
				t.Fatal("import unexpectedly succeeded")
			}
			items, err := store.List()
			if err != nil || len(items) != 0 {
				t.Fatalf("partial installation visible: %v %v", items, err)
			}
			entries, _ := os.ReadDir(filepath.Join(store.modelsDir, PackageDirectory))
			if len(entries) != 0 {
				t.Fatalf("staging not cleaned: %v", entries)
			}
		})
	}
}

func TestPackageStoreRechecksBeforeLease(t *testing.T) {
	store := NewPackageStore(t.TempDir())
	m := testPackageManifest("whisper")
	ctx := context.Background()
	if _, err := store.Import(ctx, m, fixturePackageSource); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(store.modelsDir, PackageDirectory, m.ID, m.Revision, m.Artifacts[0].Path)
	if err := os.WriteFile(file, []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	state, err := store.Verify(ctx, m.ID, m.Revision)
	if err == nil || state.Integrity != "failed" {
		t.Fatalf("verification: %+v %v", state, err)
	}
	if _, _, err := store.Acquire(ctx, m.ID, m.Revision, "job"); err == nil {
		t.Fatal("leased corrupt package")
	}
	items, _ := NewPackageStore(store.modelsDir).List()
	if len(items) != 1 || items[0].Integrity == "checked" {
		t.Fatal("restart trusted previous digest check")
	}
}

func makePackageDirectory(t *testing.T, m ModelPackageManifest) string {
	t.Helper()
	dir := t.TempDir()
	b, _ := json.Marshal(m)
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
	for _, a := range m.Artifacts {
		if err := os.WriteFile(filepath.Join(dir, a.Path), []byte(a.Role), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestPackageDirectoryImport(t *testing.T) {
	m := testPackageManifest("piper")
	source := makePackageDirectory(t, m)
	store := NewPackageStore(t.TempDir())
	if _, err := store.ImportDirectory(context.Background(), source); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(source, m.Artifacts[0].Path)); err != nil {
		t.Fatal("source was modified")
	}
	if err := os.WriteFile(filepath.Join(source, "installer.py"), []byte("do not execute"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewPackageStore(t.TempDir()).ImportDirectory(context.Background(), source); err == nil {
		t.Fatal("accepted undeclared file")
	}
}

func TestPackageDirectoryRejectsSymlink(t *testing.T) {
	m := testPackageManifest("whisper")
	source := makePackageDirectory(t, m)
	target := filepath.Join(source, m.Artifacts[0].Path)
	linked := filepath.Join(t.TempDir(), "weights.bin")
	if err := os.Rename(target, linked); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(linked, target); err != nil {
		t.Skipf("OS does not permit test symlinks: %v", err)
	}
	if _, err := NewPackageStore(t.TempDir()).ImportDirectory(context.Background(), source); err == nil {
		t.Fatal("accepted linked weights")
	}
}

func TestPackageConcurrentLeases(t *testing.T) {
	store := NewPackageStore(t.TempDir())
	m := testPackageManifest("whisper")
	ctx := context.Background()
	if _, err := store.Import(ctx, m, fixturePackageSource); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, release, err := store.Acquire(ctx, m.ID, m.Revision, "job")
			if err != nil {
				t.Error(err)
				return
			}
			if err := store.Remove(m.ID, m.Revision); !errors.Is(err, ErrPackageInUse) {
				t.Errorf("remove: %v", err)
			}
			release()
		}()
	}
	wg.Wait()
	if err := store.Remove(m.ID, m.Revision); err != nil {
		t.Fatal(err)
	}
}

func TestPackageStoreRejectsRemovalEscape(t *testing.T) {
	store := NewPackageStore(t.TempDir())
	for _, id := range []string{"..", "../x", "/", `C:\`, "con"} {
		if err := store.Remove(id, "one"); err == nil {
			t.Errorf("accepted %q", id)
		}
	}
}

func TestPackageExcludedFromLegacyScan(t *testing.T) {
	dir := t.TempDir()
	registry := NewRegistry(dir)
	if _, err := registry.Packages().Import(context.Background(), testPackageManifest("whisper"), fixturePackageSource); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "legacy-chat.gguf"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := registry.ScanModels(); err != nil {
		t.Fatal(err)
	}
	items := registry.ListModels()
	if len(items) != 1 || items[0].ID != "legacy-chat" {
		t.Fatalf("speech package leaked into legacy registry: %+v", items)
	}
}

func TestPackageStorageRejectsLinkedRoot(t *testing.T) {
	dir, outside := t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dir, PackageDirectory)); err != nil {
		t.Skipf("OS does not permit test symlinks: %v", err)
	}
	if _, err := NewPackageStore(dir).Import(context.Background(), testPackageManifest("whisper"), fixturePackageSource); err == nil {
		t.Fatal("accepted linked package storage")
	}
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 0 {
		t.Fatalf("wrote outside managed root: %v %v", entries, err)
	}
}

func TestPackageCorruptManifestRemainsVisible(t *testing.T) {
	store := NewPackageStore(t.TempDir())
	m := testPackageManifest("whisper")
	if _, err := store.Import(context.Background(), m, fixturePackageSource); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(store.modelsDir, PackageDirectory, m.ID, m.Revision, "manifest.json")
	if err := os.WriteFile(manifest, []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	items, err := store.List()
	if err != nil || len(items) != 1 || items[0].Integrity != "failed" {
		t.Fatalf("corrupt package hidden: %+v %v", items, err)
	}
}

func TestPackageRemovalRejectsReplacedParent(t *testing.T) {
	dir := t.TempDir()
	store := NewPackageStore(dir)
	m := testPackageManifest("whisper")
	if _, err := store.Import(context.Background(), m, fixturePackageSource); err != nil {
		t.Fatal(err)
	}
	original := filepath.Join(dir, PackageDirectory, m.ID)
	other := filepath.Join(dir, PackageDirectory, "other-package")
	if err := os.Rename(original, other); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(other, original); err != nil {
		t.Skipf("OS does not permit test symlinks: %v", err)
	}
	if err := store.Remove(m.ID, m.Revision); err == nil {
		t.Fatal("removed through linked parent")
	}
	if _, err := os.Stat(filepath.Join(other, m.Revision, "manifest.json")); err != nil {
		t.Fatal("deleted different package")
	}
}
