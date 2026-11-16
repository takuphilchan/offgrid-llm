package server

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/takuphilchan/offgrid-llm/internal/rag"
)

func (s *Server) handleDocumentSynthesisReview(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != "GET" && r.Method != "POST" {
		writeError(w, "Method not allowed", 405)
		return
	}
	if !s.requireKnowledgeStorage(w) {
		return
	}
	store, err := s.ragEngine.ImportStore()
	if err != nil {
		writeAnalysisError(w, err)
		return
	}
	select {
	case analysisResponseReaders <- struct{}{}:
		defer func() { <-analysisResponseReaders }()
	case <-r.Context().Done():
		return
	}
	var review *rag.SynthesisReview
	if r.Method == "GET" {
		id := r.URL.Query().Get("id")
		batch, e := analysisQueryInteger(r, "batch", -1)
		if e != nil || id == "" || len(id) > 128 || batch < 0 {
			writeAnalysisError(w, rag.ErrAnalysisInvalid)
			return
		}
		version, e := analysisQueryInteger(r, "revision", 0)
		if e != nil || version < 0 || version > 256 {
			writeAnalysisError(w, rag.ErrAnalysisInvalid)
			return
		}
		review, err = store.SynthesisReview(r.Context(), modelActor(r), id, batch, version)
	} else {
		var body rag.SynthesisReviewRequest
		d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 128<<10))
		d.DisallowUnknownFields()
		if d.Decode(&body) != nil || d.Decode(new(any)) != io.EOF || body.ID == "" || len(body.ID) > 128 {
			writeAnalysisError(w, rag.ErrAnalysisInvalid)
			return
		}
		review, err = store.SaveSynthesisReview(r.Context(), modelActor(r), body)
	}
	if err != nil {
		writeAnalysisError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(review)
}
