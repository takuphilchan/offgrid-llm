package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/models"
)

func TestExportModelUsesDisposableSourceAndDestination(t *testing.T) {
	root := t.TempDir()
	modelsDir := filepath.Join(root, "models")
	destination := filepath.Join(root, "export")
	if err := os.MkdirAll(modelsDir, 0700); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(modelsDir, "fixture.gguf")
	payload := []byte("temporary fixture content")
	if err := os.WriteFile(source, payload, 0600); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	env := testEnvironment(&out, &errOut, &fakeRunner{})
	env.LocalAdmin.LoadConfig = func() *config.Config { return &config.Config{ModelsDir: modelsDir} }
	env.ModelVerify.NewRegistry = models.NewRegistry
	env.ModelTransfer = ModelTransferDependencies{
		MkdirAll: os.MkdirAll,
		Open: func(path string) (io.ReadCloser, error) {
			if path != source {
				t.Fatalf("unexpected source %q", path)
			}
			return os.Open(path)
		},
		Create: func(path string) (io.WriteCloser, error) {
			if path != filepath.Join(destination, "fixture.gguf") {
				t.Fatalf("unexpected destination %q", path)
			}
			return os.Create(path)
		},
	}
	if code := Execute(context.Background(), []string{"export", "fixture", destination}, env); code != 0 || !strings.Contains(out.String(), "Export complete") {
		t.Fatalf("export failed: code=%d stdout=%q stderr=%q", code, out.String(), errOut.String())
	}
	if copied, err := os.ReadFile(filepath.Join(destination, "fixture.gguf")); err != nil || !bytes.Equal(copied, payload) {
		t.Fatalf("export content mismatch: %v", err)
	}
	if original, err := os.ReadFile(source); err != nil || !bytes.Equal(original, payload) {
		t.Fatalf("source was modified: %v", err)
	}
}
