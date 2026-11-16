package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/takuphilchan/offgrid-llm/internal/document"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"github.com/takuphilchan/offgrid-llm/internal/users"
)

func writeStructureError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, rag.ErrCollectionUnavailable):
		writeErrorWithCode(w, "Document not found", 404, "document_not_found")
	case errors.Is(err, rag.ErrExtractionConflict):
		writeErrorWithCode(w, "The extraction changed. Reload before saving a correction.", 409, "document_extraction_conflict")
	case errors.Is(err, rag.ErrExtractionUnavailable):
		writeErrorWithCode(w, "A retained structured extraction is unavailable for this revision.", 409, "document_extraction_unavailable")
	case errors.Is(err, rag.ErrExtractionInvalid):
		writeErrorWithCode(w, "Provide a current, complete text block and its revision to correct the extraction.", 400, "document_correction_invalid")
	case errors.Is(err, rag.ErrExtractionIndexUnavailable):
		writeErrorWithCode(w, "Load the index's configured embedding model before correcting this document.", 409, "document_index_unavailable")
	default:
		writeErrorWithCode(w, "Document review is temporarily unavailable. No correction was confirmed.", 503, "document_review_unavailable")
	}
}

func (s *Server) handleDocumentStructure(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet {
		writeError(w, "Method not allowed", 405)
		return
	}
	if !s.requireKnowledgeStorage(w) {
		return
	}
	q := r.URL.Query()
	offset, limit := 0, 100
	relationshipOffset := 0
	var err error
	if q.Has("offset") {
		offset, err = strconv.Atoi(q.Get("offset"))
		if err != nil {
			writeStructureError(w, rag.ErrExtractionInvalid)
			return
		}
	}
	if q.Has("limit") {
		limit, err = strconv.Atoi(q.Get("limit"))
		if err != nil {
			writeStructureError(w, rag.ErrExtractionInvalid)
			return
		}
	}
	if q.Get("id") == "" || len(q.Get("id")) > 256 || len(q.Get("revision")) > 128 {
		writeStructureError(w, rag.ErrExtractionInvalid)
		return
	}
	if q.Has("relationship_offset") {
		relationshipOffset, err = strconv.Atoi(q.Get("relationship_offset"))
		if err != nil {
			writeStructureError(w, rag.ErrExtractionInvalid)
			return
		}
	}
	navigationOffset := 0
	if q.Has("navigation_offset") {
		navigationOffset, err = strconv.Atoi(q.Get("navigation_offset"))
		if err != nil {
			writeStructureError(w, rag.ErrExtractionInvalid)
			return
		}
	}
	page, err := s.ragEngine.ForActor(modelActor(r)).Structure(r.Context(), q.Get("id"), q.Get("revision"), q.Get("unit"), offset, limit, relationshipOffset, navigationOffset)
	if err != nil {
		writeStructureError(w, err)
		return
	}
	if user := users.GetUser(r); user != nil && !user.HasPermission(users.PermissionRAGManage) {
		page.CanCorrect = false
		for i := range page.Blocks {
			page.Blocks[i].Editable = false
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(page)
}

func (s *Server) handleDocumentCorrections(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodPost {
		writeError(w, "Method not allowed", 405)
		return
	}
	if !s.requireKnowledgeStorage(w) {
		return
	}
	var body document.CorrectionRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&body) != nil || decoder.Decode(new(any)) != io.EOF || body.DocumentID == "" || len(body.DocumentID) > 256 || body.ExpectedRevision == "" || len(body.ExpectedRevision) > 128 || len(body.Edits) == 0 || len(body.Edits) > 256 {
		writeStructureError(w, rag.ErrExtractionInvalid)
		return
	}
	doc, err := s.ragEngine.ForActor(modelActor(r)).CorrectDocument(r.Context(), body.DocumentID, body.ExpectedRevision, body.Edits)
	if err != nil {
		writeStructureError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(IngestResponse{Success: true, Document: doc})
}
