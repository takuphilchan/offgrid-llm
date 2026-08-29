package server

import (
	"net/http"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func handleOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	if r.Method == http.MethodGet {
		_, _ = w.Write(api.OpenAPISpec)
	}
}
