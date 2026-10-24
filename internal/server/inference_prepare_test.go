package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInferencePreparationUsesChatAdmissionWithoutGeneration(t *testing.T) {
	s := newTestServer(t)
	if err := os.WriteFile(filepath.Join(s.config.ModelsDir, "prepare.gguf"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.registry.ScanModels(); err != nil {
		t.Fatal(err)
	}
	model := s.registry.ListModels()[0].ID
	p := &retryDetectLlamaProcess{}
	s.llamaProcess = p
	s.engine = &beforeOutputFailureEngine{}
	body, _ := json.Marshal(map[string]string{"model": model})
	for range 2 {
		w := httptest.NewRecorder()
		s.handleInferencePrepare(w, httptest.NewRequest("POST", "/v1/inference/prepare", strings.NewReader(string(body))))
		if w.Code != 200 || !strings.Contains(w.Body.String(), `"prepared":true`) {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if p.activations != 1 || s.engine.(*beforeOutputFailureEngine).calls != 0 {
		t.Fatal("preparation generated text or reloaded a healthy resident", p.activations)
	}
	for _, bad := range []string{`{}`, `{"model":"missing"}`, `{"model":"prepare","messages":[]}`, string(body) + string(body)} {
		w := httptest.NewRecorder()
		s.handleInferencePrepare(w, httptest.NewRequest("POST", "/v1/inference/prepare", strings.NewReader(bad)))
		if w.Code < 400 {
			t.Fatal("invalid preparation accepted", bad)
		}
	}
	s.config.RequireAuth = true
	w := httptest.NewRecorder()
	s.handleInferencePrepare(w, httptest.NewRequest("POST", "/v1/inference/prepare", strings.NewReader(string(body))))
	if w.Code != http.StatusUnauthorized || p.activations != 1 {
		t.Fatal("preparation bypassed authorization", w.Code)
	}
}
