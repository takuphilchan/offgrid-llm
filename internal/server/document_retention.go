package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/takuphilchan/offgrid-llm/internal/rag"
)

func writeRetentionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, rag.ErrCollectionUnavailable):
		writeErrorWithCode(w, "Document not found", 404, "document_not_found")
	case errors.Is(err, rag.ErrDocumentHeld):
		writeErrorWithCode(w, "This document is on legal hold. Its retention owner must release the hold before deletion.", 409, "document_held")
	case errors.Is(err, rag.ErrCollectionConflict):
		writeErrorWithCode(w, "Retention changed. Reload the policy before editing.", 409, "document_retention_conflict")
	default:
		writeErrorWithCode(w, "Document retention is unavailable. No deletion was confirmed.", 503, "document_retention_unavailable")
	}
}

func (s *Server) handleDocumentRetention(w http.ResponseWriter, r *http.Request) {
	// Denials and stale-policy responses are actor-specific too. In particular,
	// a cached 404 must not outlive an explicit access or retention change.
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet && r.Method != http.MethodPut {
		writeError(w, "Method not allowed", 405)
		return
	}
	if !s.requireKnowledgeStorage(w) {
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" || len(id) > 256 {
		writeErrorWithCode(w, "Choose a document", 400, "invalid_request")
		return
	}
	view := s.ragEngine.ForActor(modelActor(r))
	var policy *rag.DocumentRetention
	var err error
	if r.Method == http.MethodGet {
		policy, err = view.Retention(id)
	} else {
		var body struct {
			Version     int    `json:"version"`
			Revision    int    `json:"revision"`
			DeleteAfter *int64 `json:"delete_after"`
			LegalHold   *bool  `json:"legal_hold"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&body) != nil || decoder.Decode(new(any)) != io.EOF || body.Version != 1 || body.Revision < 1 || body.DeleteAfter == nil || body.LegalHold == nil || *body.DeleteAfter < 0 || *body.DeleteAfter > 253402300799 {
			writeErrorWithCode(w, "Provide the current policy revision, expiry and hold state", 400, "invalid_request")
			return
		}
		policy, err = view.SetRetention(id, body.Revision, *body.DeleteAfter, *body.LegalHold)
	}
	if err != nil {
		writeRetentionError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(policy)
}
