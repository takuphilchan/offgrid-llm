package models

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLegacyRegistryProjectsProviderWithoutMislabelingOlderFormats(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"chat.gguf", "old.bin"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	registry := NewRegistry(root)
	if err := registry.ScanModels(); err != nil {
		t.Fatal(err)
	}
	items := registry.ListModels()
	if len(items) != 2 {
		t.Fatalf("expected two legacy models, got %d", len(items))
	}
	for _, item := range items {
		if item.Engine != EngineLlamaCPP {
			t.Fatalf("legacy model lost its engine: %#v", item)
		}
		if item.ID == "chat" && item.Format != FormatGGUF {
			t.Fatalf("GGUF model lost its format: %#v", item)
		}
		if item.ID == "old" && item.Format != "" {
			t.Fatalf("older format was misrepresented as GGUF: %#v", item)
		}
	}
}
