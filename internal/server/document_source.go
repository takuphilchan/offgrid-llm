package server

import (
	"encoding/json"
	"net/http"
)

// The retained extraction is not represented as an original binary download.
// This route is protected by the same knowledge-read permission as retrieval.
func (s *Server) handleDocumentSource(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		writeError(w, "Method not allowed", 405)
		return
	}
	if !s.requireKnowledgeStorage(w) {
		return
	}
	doc := s.ragEngine.GetDocument(r.URL.Query().Get("id"))
	if doc == nil {
		writeError(w, "Document not found", 404)
		return
	}
	if !doc.SourceRetained {
		writeError(w, "Source text is unavailable for this legacy document", 410)
		return
	}
	content := []rune(doc.RawContent)
	truncated := len(content) > 200000
	if truncated {
		content = content[:200000]
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(map[string]any{"document": doc, "content": string(content), "truncated": truncated})
}
