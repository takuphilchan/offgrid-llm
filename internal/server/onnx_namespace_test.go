package server

import (
	"encoding/json"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestONNXNamespaceCannotReachCollidingLegacyPublicModel(t *testing.T) {
	s := profileFixture(t)
	root := t.TempDir()
	file := filepath.Join(root, "retained.gguf")
	if err := os.WriteFile(file, []byte("unchanged collider"), 0600); err != nil {
		t.Fatal(err)
	}
	selector := "embedding:missing@v1"
	raw, _ := json.Marshal(map[string]*api.ModelMetadata{selector: {ID: selector, Path: file, Type: "llm", Format: "gguf", ContextSize: 8192}})
	registryFile := filepath.Join(root, "registry.json")
	if err := os.WriteFile(registryFile, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.registry.LoadRegistry(registryFile); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name    string
		handler http.HandlerFunc
		payload string
		status  int
	}{
		{"completion", s.handleCompletions, `{"model":"` + selector + `","prompt":"hello"}`, 404},
		{"embedding", s.handleEmbeddings, `{"model":"` + selector + `","input":"hello"}`, 404},
		{"prepare", s.handleInferencePrepare, `{"model":"` + selector + `"}`, 400},
		{"agent", s.handleAgentRun, `{"model":"` + selector + `","prompt":"hello"}`, 404},
		{"delete", s.handleDeleteModel, `{"model_id":"` + selector + `"}`, 409},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			test.handler(response, httptest.NewRequest(http.MethodPost, "/fixture", strings.NewReader(test.payload)))
			if response.Code != test.status {
				t.Fatalf("reserved model reached legacy operation: %d %s", response.Code, response.Body.String())
			}
		})
	}
	if raw, err := os.ReadFile(file); err != nil || string(raw) != "unchanged collider" {
		t.Fatalf("collider changed: %q %v", raw, err)
	}
}
