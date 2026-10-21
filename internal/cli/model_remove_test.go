package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/models"
)

func TestRemoveModelUsesOnlySelectedDisposableDirectory(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "fixture.gguf")
	if err := os.WriteFile(path, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	env := testEnvironment(&out, &errOut, &fakeRunner{})
	env.LocalAdmin.LoadConfig = func() *config.Config { return &config.Config{ModelsDir: root} }
	env.ModelVerify.NewRegistry = models.NewRegistry
	removed := 0
	env.ModelTransfer.Remove = func(target string) error {
		if target != path {
			t.Fatalf("unexpected delete target %q", target)
		}
		removed++
		return os.Remove(target)
	}
	if code := Execute(context.Background(), []string{"remove", "fixture", "--yes"}, env); code != 0 || removed != 1 || !strings.Contains(out.String(), "Model removed") {
		t.Fatalf("remove failed: code=%d removed=%d out=%q err=%q", code, removed, out.String(), errOut.String())
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("fixture file still present: %v", err)
	}
	out.Reset()
	if code := Execute(context.Background(), []string{"remove", "missing", "--yes"}, env); code != 1 || removed != 1 || !strings.Contains(out.String(), "Model not found") {
		t.Fatalf("missing-model contract changed: code=%d removed=%d out=%q", code, removed, out.String())
	}
}
