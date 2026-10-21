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

type fixtureUSBImporter struct {
	files       []string
	single, all int
}

func (f *fixtureUSBImporter) ScanUSBDrive(string) ([]string, error) { return f.files, nil }
func (f *fixtureUSBImporter) GetModelInfo(string) (string, string)  { return "fixture", "Q4_K_M" }
func (f *fixtureUSBImporter) ImportAll(_ string, progress func(models.ImportProgress)) (int, error) {
	f.all++
	progress(models.ImportProgress{Status: "complete", FileName: "fixture.gguf"})
	return len(f.files), nil
}
func (f *fixtureUSBImporter) ImportModel(_ string, _ string, progress func(models.ImportProgress)) error {
	f.single++
	progress(models.ImportProgress{Status: "complete", FileName: "fixture.gguf"})
	return nil
}

func TestImportModelUsesInjectedDisposableSourceAndReload(t *testing.T) {
	root := t.TempDir()
	modelsDir := filepath.Join(root, "models")
	usbDir := filepath.Join(root, "usb")
	if err := os.MkdirAll(usbDir, 0700); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(usbDir, "fixture.gguf")
	if err := os.WriteFile(source, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	env := testEnvironment(&out, &errOut, &fakeRunner{})
	env.LocalAdmin.LoadConfig = func() *config.Config { return &config.Config{ModelsDir: modelsDir} }
	env.ModelVerify.NewRegistry = models.NewRegistry
	importer := &fixtureUSBImporter{files: []string{source}}
	reloadedModel, reloadedServer := "", 0
	env.ModelTransfer = ModelTransferDependencies{
		Stat: os.Stat,
		NewUSBImporter: func(dir string, _ *models.Registry) USBImporter {
			if dir != modelsDir {
				t.Fatalf("wrong models dir %q", dir)
			}
			return importer
		},
		RenderProgress: func(int64, int64, float64, int) {},
		ReloadModel:    func(path string) error { reloadedModel = path; return nil },
		ReloadServer:   func() error { reloadedServer++; return nil },
	}
	if code := Execute(context.Background(), []string{"import", source}, env); code != 0 || importer.single != 1 || reloadedModel != filepath.Join(modelsDir, "fixture.gguf") {
		t.Fatalf("single import failed: code=%d single=%d reload=%q stderr=%q", code, importer.single, reloadedModel, errOut.String())
	}
	out.Reset()
	errOut.Reset()
	if code := Execute(context.Background(), []string{"import", usbDir}, env); code != 0 || importer.all != 1 || reloadedServer != 1 || !strings.Contains(out.String(), "Imported 1 model") {
		t.Fatalf("directory import failed: code=%d all=%d reload=%d out=%q stderr=%q", code, importer.all, reloadedServer, out.String(), errOut.String())
	}
}
