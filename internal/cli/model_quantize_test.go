package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/models"
)

func TestQuantizeUsesInjectedProcessAndDisposableModelDirectory(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "fixture.F16.gguf")
	if err := os.WriteFile(source, []byte("fixture model"), 0600); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	env := testEnvironment(&out, &errOut, &fakeRunner{})
	env.LocalAdmin.LoadConfig = func() *config.Config { return &config.Config{ModelsDir: root} }
	env.ModelVerify.NewRegistry = models.NewRegistry
	runs := 0
	clock := time.Unix(0, 0)
	env.Quantize = QuantizeDependencies{
		Stat: os.Stat,
		LookPath: func(name string) (string, error) {
			if name != "llama-quantize" {
				t.Fatalf("wrong binary %q", name)
			}
			return name, nil
		},
		Now: func() time.Time { clock = clock.Add(time.Second); return clock },
		Run: func(ctx context.Context, binary, src, dest, quant string, stdout, stderr io.Writer) error {
			if ctx == nil || binary != "llama-quantize" || src != source || quant != "Q4_K_M" || !strings.HasPrefix(dest, root) {
				t.Fatalf("wrong quantization request: %q %q %q %q", binary, src, dest, quant)
			}
			runs++
			return os.WriteFile(dest, []byte("result"), 0600)
		},
	}
	if code := Execute(context.Background(), []string{"quantize", "fixture.F16", "Q4_K_M"}, env); code != 0 || runs != 1 || !strings.Contains(out.String(), "Quantization Complete") {
		t.Fatalf("quantization did not use fake process: code=%d runs=%d out=%q err=%q", code, runs, out.String(), errOut.String())
	}
	out.Reset()
	errOut.Reset()
	if code := Execute(context.Background(), []string{"quantize", "fixture.F16", "INVALID"}, env); code != 1 || runs != 1 || !strings.Contains(errOut.String(), "Invalid quantization type") {
		t.Fatalf("invalid quantization contract changed: code=%d runs=%d err=%q", code, runs, errOut.String())
	}
	out.Reset()
	if code := Execute(context.Background(), []string{"quantization"}, env); code != 0 || !strings.Contains(out.String(), "Quantization Guide") {
		t.Fatalf("guide changed: code=%d out=%q", code, out.String())
	}
}
