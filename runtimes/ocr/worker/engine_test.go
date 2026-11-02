package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/documentocr"
	"github.com/tetratelabs/wazero"
)

func TestWASMImportInventory(t *testing.T) {
	ctx := context.Background()
	runtime := wazero.NewRuntime(ctx)
	defer runtime.Close(ctx)
	module, err := runtime.CompileModule(ctx, engineBytes)
	if err != nil {
		t.Fatal(err)
	}
	for _, function := range module.ImportedFunctions() {
		namespace, name, _ := function.Import()
		t.Log(namespace, name, function.ParamTypes(), function.ResultTypes())
	}
}

func TestWASMOCRAuthoredCorpus(t *testing.T) {
	modelPath, workerPath := os.Getenv("OFFGRID_TEST_OCR_LANGUAGE"), os.Getenv("OFFGRID_TEST_DOCUMENT_WORKER")
	if modelPath == "" || workerPath == "" {
		t.Skip("requires pinned test language and actual PDF worker")
	}
	model, err := os.ReadFile(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	if digest(model) != languageSHA {
		t.Fatal("incorrect test language digest")
	}
	// Report runtime/import initialization failures directly, without exposing a
	// private document. The process protocol intentionally returns closed codes.
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	engine, err := newEngine(ctx, model)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	engine.close()
	cancel()
	base := "../../../internal/rag/testdata/documents"
	for _, row := range []struct {
		file, text string
		page       int
		partial    bool
	}{
		{"corpus/scan.pdf", "Scanned receipt total 42", 1, false},
		{"corpus/mixed.pdf", "Scanned page total 34", 2, false},
		{"ocr/inputs/visible-rotation-0.pdf", "Harvest total 37 bags", 1, false},
		{"ocr/inputs/visible-rotation-90.pdf", "Harvest total 37 bags", 1, false},
		{"ocr/inputs/visible-rotation-180.pdf", "Harvest total 37 bags", 1, false},
		{"ocr/inputs/visible-rotation-270.pdf", "Harvest total 37 bags", 1, false},
		{"ocr/inputs/mixed-same-page.pdf", "Native stock total 12 bags Harvest total 37 bags", 1, false},
		{"corpus/scan-rotated.pdf", "", 1, true},
	} {
		t.Run(row.file, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			pdf, err := os.ReadFile(filepath.Join(base, row.file))
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.CommandContext(ctx, workerPath, "--raster-page", strconv.Itoa(row.page))
			cmd.Stdin = bytes.NewReader(pdf)
			cmd.Env = []string{"PATH=", "OFFGRID_DOCUMENT_CODE_CACHE=" + t.TempDir(), "SystemRoot=" + os.Getenv("SystemRoot")}
			raw, err := cmd.Output()
			if err != nil {
				t.Fatal("real PDF rendering failed", err)
			}
			var raster struct {
				PNG   []byte `json:"png"`
				SHA   string `json:"sha256"`
				Error string `json:"error"`
			}
			if json.Unmarshal(raw, &raster) != nil || raster.Error != "" || raster.SHA != digest(pdf) {
				t.Fatal("bad raster receipt")
			}
			input, err := json.Marshal(request{Protocol: 1, Language: "eng", LanguageSHA: languageSHA, Model: model, PNG: raster.PNG, PNG_SHA: digest(raster.PNG)})
			if err != nil {
				t.Fatal(err)
			}
			result := process(ctx, input)
			if result.Error != "" {
				t.Fatal(result.Error)
			}
			w, h := result.Width, result.Height
			if result.Rotation == 90 || result.Rotation == 270 {
				w, h = h, w
			}
			words, err := documentocr.ParseHOCR(result.HOCR, w, h)
			if err != nil {
				t.Fatal(err)
			}
			var parts []string
			for _, word := range words {
				parts = append(parts, word.Text)
				if _, err := documentocr.OriginalBounds(word.Bounds, result.Width, result.Height, result.Rotation); err != nil {
					t.Fatal(err)
				}
			}
			actual := strings.Join(parts, " ")
			if row.partial {
				if result.State != "partial" {
					t.Fatal("clipped source claimed complete", actual)
				}
			} else if actual != row.text || result.State != "complete" {
				t.Fatalf("wrong content/state %q: %+v", actual, result)
			}
			if result.Attempts < 1 || result.Attempts > 4 {
				t.Fatal("unbounded attempts")
			}
			t.Logf("state=%s attempts=%d rotation=%d text=%q", result.State, result.Attempts, result.Rotation, actual)
		})
	}
}

func TestOCRRejectsRequestsBeforeExecution(t *testing.T) {
	for _, row := range []struct{ input, code string }{
		{`{`, "document_invalid"}, {`{"protocol":1,"path":"/private"}`, "document_invalid"},
		{`{"protocol":1,"language":"fra"}`, "document_ocr_language_unavailable"},
		{`{"protocol":1,"language":"eng"}`, "document_ocr_language_missing"},
		{`{"protocol":1,"language":"eng","model":"YQ=="}`, "document_ocr_language_invalid"},
	} {
		result := process(context.Background(), []byte(row.input))
		if result.Error != row.code || result.HOCR != "" || result.State != "" {
			t.Fatal(result, row.code)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if result := process(ctx, []byte(`{}`)); result.Error != "document_timeout" {
		t.Fatal(result)
	}
}
