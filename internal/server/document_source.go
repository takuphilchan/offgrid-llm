package server

import (
	"encoding/json"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
)

// The retained extraction is not represented as an original binary download.
// This route is protected by the same knowledge-read permission as retrieval.
func (s *Server) handleDocumentSource(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != "GET" {
		writeError(w, "Method not allowed", 405)
		return
	}
	if !s.requireKnowledgeStorage(w) {
		return
	}
	view := s.ragEngine.ForActor(modelActor(r))
	doc := view.GetDocument(r.URL.Query().Get("id"))
	if doc == nil {
		writeError(w, "Document not found", 404)
		return
	}
	query := r.URL.Query()
	passage := false
	for _, key := range []string{"start_byte", "end_byte", "source_sha256", "extractor_revision", "content_hash"} {
		passage = passage || query.Has(key)
	}
	if passage {
		start, startErr := strconv.Atoi(query.Get("start_byte"))
		end, endErr := strconv.Atoi(query.Get("end_byte"))
		if startErr != nil || endErr != nil || start < 0 || end <= start || end-start > 800000 || len(query.Get("source_sha256")) != 64 || len(query.Get("content_hash")) != 32 || query.Get("extractor_revision") == "" || len(query.Get("extractor_revision")) > 512 || (query.Get("format") != "" && query.Get("format") != "text") {
			writeErrorWithCode(w, "A complete text citation is required to view this passage.", 400, "document_citation_invalid")
			return
		}
		content, err := doc.ReadSourcePassage(start, end, query.Get("source_sha256"), query.Get("extractor_revision"), query.Get("content_hash"))
		if err != nil {
			writeErrorWithCode(w, "This source no longer matches the saved citation. The original answer has not been changed.", 409, "document_citation_stale")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		if !view.CanReadDocument(doc.ID) {
			writeError(w, "Document not found", 404)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"document": doc, "content": content, "truncated": false, "citation_verified": true})
		return
	}
	if r.URL.Query().Get("format") == "original" {
		source, err := view.Original(doc.ID)
		if err != nil {
			writeErrorWithCode(w, "Original file is unavailable. Retained source text may still be viewed.", 410, "document_original_unavailable")
			return
		}
		if !view.CanReadDocument(doc.ID) {
			writeError(w, "Document not found", 404)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filepath.Base(source.Name)}))
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-OffGrid-Source-SHA256", source.SHA256)
		_, _ = w.Write(source.Bytes)
		return
	}
	if query.Get("format") == "preview" {
		source, err := view.Original(doc.ID)
		if err != nil {
			writeErrorWithCode(w, "Original file is unavailable.", 410, "document_original_unavailable")
			return
		}
		serveDocumentPreview(w, r, s.ragEngine.DocumentParser(), source.Name, source.Bytes, func() bool {
			current, err := view.Original(doc.ID)
			return err == nil && current.SHA256 == source.SHA256 && view.CanReadDocument(doc.ID)
		})
		return
	}
	if format := r.URL.Query().Get("format"); format != "" && format != "text" {
		writeError(w, "Unknown source format", 400)
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
	if !view.CanReadDocument(doc.ID) {
		writeError(w, "Document not found", 404)
		return
	}
	json.NewEncoder(w).Encode(map[string]any{"document": doc, "content": string(content), "truncated": truncated})
}
