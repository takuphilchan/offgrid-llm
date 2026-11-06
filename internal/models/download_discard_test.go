package models

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDiscardGGUFPartialPreservesPublishedAndOtherDownloads(t *testing.T) {
	f, hf, _ := newPackageHubFixture(t, "vision")
	f.add("model.gguf", []byte("weights"), true)
	f.add("mmproj-F16.gguf", []byte("projector"), true)
	b, err := hf.ResolveGGUFBundle(context.Background(), f.meta.ID, "", "model.gguf")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	local := "selected.gguf"
	projector := companionPath(local, *b)
	partials := []string{local + ".tmp", local + "." + b.Weights.SHA256[:16] + ".tmp", projector + "." + b.Projector.SHA256[:16] + ".tmp"}
	kept := []string{local, projector, "other.gguf.tmp", "receipt.json"}
	for _, name := range append(partials, kept...) {
		p := filepath.Join(dir, name)
		if err = os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(p, []byte("preserve"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err = DiscardGGUFPartial(dir, local, b); err != nil {
		t.Fatal(err)
	}
	for _, name := range partials {
		if _, err = os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Fatalf("partial remains %s: %v", name, err)
		}
	}
	for _, name := range kept {
		data, e := os.ReadFile(filepath.Join(dir, name))
		if e != nil || string(data) != "preserve" {
			t.Fatalf("changed %s", name)
		}
	}
	if err = DiscardGGUFPartial(dir, local, b); err != nil {
		t.Fatal("retry", err)
	}
	if err = DiscardGGUFPartial(t.TempDir(), local, b); err != nil {
		t.Fatal("missing projector directory", err)
	}
}
func TestDiscardGGUFPartialRejectsUnsafeEntries(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"../outside.gguf", "sub/file.gguf", "C:/outside.gguf", ""} {
		if DiscardGGUFPartial(dir, name, nil) == nil {
			t.Fatalf("accepted %q", name)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "model.gguf.tmp"), 0700); err != nil {
		t.Fatal(err)
	}
	if DiscardGGUFPartial(dir, "model.gguf", nil) == nil {
		t.Fatal("accepted directory")
	}
}
