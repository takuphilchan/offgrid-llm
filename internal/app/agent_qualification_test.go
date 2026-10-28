package app

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestInvalidAgentQualificationDoesNotDisableService(t *testing.T) {
	cfg := disposableCoreConfig(t)
	dir := filepath.Join(cfg.DataDir, "model-capabilities")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "native-agent-installations.json"), []byte(`{"version":999}`), 0600); err != nil {
		t.Fatal(err)
	}
	application, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer application.Close()
	response := httptest.NewRecorder()
	application.transport.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health", nil))
	if response.Code != http.StatusOK {
		t.Fatal(response.Code, response.Body.String())
	}
}
