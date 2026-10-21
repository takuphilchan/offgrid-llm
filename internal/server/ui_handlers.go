package server

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Handler functions (placeholders for now)
func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	// Serve the UI at root by default, but allow API clients to request JSON.
	if r.URL.Path == "/" {
		accept := r.Header.Get("Accept")
		if strings.Contains(accept, "application/json") || r.URL.Query().Get("format") == "json" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"name":"OffGrid LLM","version":"%s","status":"running"}`, s.version)
			return
		}

		http.ServeFile(w, r, filepath.Join(resolveUIRoot(), "index.html"))
		return
	}

	http.NotFound(w, r)
}

// handleWebUI serves the HTML UI
func (s *Server) handleWebUI(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, filepath.Join(resolveUIRoot(), "index.html"))
}

func resolveUIRoot() string {
	candidates := []string{}
	if configured := strings.TrimSpace(os.Getenv("OFFGRID_UI_DIR")); configured != "" {
		candidates = append(candidates, configured)
	}
	candidates = append(candidates, "apps/web/dist", "/var/lib/offgrid/web/ui")
	for _, candidate := range candidates {
		if _, err := os.Stat(filepath.Join(candidate, "index.html")); err == nil {
			return candidate
		}
	}
	return "apps/web/dist"
}
