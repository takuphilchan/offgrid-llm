package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/models"
)

func TestComputerModelCheckResponseTiming(t *testing.T) {
	for _, mode := range []string{"prose", "runtime"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "fixture.gguf"), []byte("test-only"), 0600); err != nil {
				t.Fatal(err)
			}
			registry := models.NewRegistry(dir)
			if err := registry.ScanModels(); err != nil {
				t.Fatal(err)
			}
			gate := inference.NewLifecycleGate(1)
			release, err := gate.AcquireWithContext(context.Background(), "fixture", 8192, func(context.Context, string) error { return nil })
			if err != nil {
				t.Fatal(err)
			}
			release()
			s := &Server{config: &config.Config{MaxContextSize: 8192}, registry: registry, inferenceLifecycle: gate, engine: &probeEngine{mode: mode}}
			w := httptest.NewRecorder()
			s.handleComputerModelCheck(w, httptest.NewRequest("POST", "/api/v2/computer/model-check", strings.NewReader(`{"model":"fixture","driver":"browser"}`)))
			want := 200
			if mode == "runtime" {
				want = 503
			}
			if w.Code != want {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
			var body struct {
				Timings *computerCheckTimings `json:"timings"`
				Passed  bool                  `json:"passed"`
				Error   *struct {
					Retryable bool   `json:"retryable"`
					Message   string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Timings == nil || body.Timings.AdmissionMS < 0 || body.Timings.ProbeMS < 0 || body.Passed {
				t.Fatalf("wrong projection: %s", w.Body.String())
			}
			if mode == "runtime" && (body.Error == nil || !body.Error.Retryable) {
				t.Fatal("operational failure became incompatibility")
			}
			if mode == "runtime" && (body.Error.Message == "" || strings.HasPrefix(body.Error.Message, "fixture:")) {
				t.Fatal("HTTP model-check changed its legacy error message format")
			}
		})
	}
}
