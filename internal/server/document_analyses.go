package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/takuphilchan/offgrid-llm/internal/document"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
)

var analysisResponseReaders = make(chan struct{}, 2)

func writeAnalysisError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, rag.ErrAnalysisInvalid):
		writeErrorWithCode(w, "Provide valid source revisions and analysis pagination.", 400, "document_analysis_invalid")
	case errors.Is(err, rag.ErrAnalysisUnavailable):
		writeErrorWithCode(w, "Document analysis not found.", 404, "document_analysis_not_found")
	case errors.Is(err, rag.ErrAnalysisConflict), errors.Is(err, rag.ErrCompositionReviewsChanged):
		writeErrorWithCode(w, "The analysis changed or this step is not available. Inspect its current status.", 409, "document_analysis_conflict")
	case errors.Is(err, rag.ErrAnalysisLimit):
		writeErrorWithCode(w, "Document analysis capacity is currently exhausted.", 429, "document_analysis_limit")
	default:
		writeErrorWithCode(w, "Document analysis is temporarily unavailable.", 503, "document_analysis_unavailable")
	}
}

func (s *Server) handleDocumentAnalyses(w http.ResponseWriter, r *http.Request) {
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
	owner := modelActor(r)
	var result *rag.AnalysisRecord
	if r.Method == "GET" {
		id := r.URL.Query().Get("id")
		if !r.URL.Query().Has("id") {
			offset, limit := 0, 20
			for key, target := range map[string]*int{"offset": &offset, "limit": &limit} {
				if r.URL.Query().Has(key) {
					value, e := strconv.Atoi(r.URL.Query().Get(key))
					if e != nil {
						writeAnalysisError(w, rag.ErrAnalysisInvalid)
						return
					}
					*target = value
				}
			}
			page, e := store.AnalysisHistory(owner, offset, limit)
			if e != nil {
				writeAnalysisError(w, e)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(page)
			return
		}
		if id == "" || len(id) > 128 {
			writeAnalysisError(w, rag.ErrAnalysisInvalid)
			return
		}
		result, err = store.Analysis(owner, id)
	} else {
		var body struct {
			RequestKey string               `json:"request_key"`
			Workflow   string               `json:"workflow"`
			Sources    []document.SourceRef `json:"sources"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&body) != nil || decoder.Decode(new(any)) != io.EOF {
			writeAnalysisError(w, rag.ErrAnalysisInvalid)
			return
		}
		if body.Workflow != "comparison" {
			// Do not imply that a deterministic diff implements model synthesis.
			writeErrorWithCode(w, "This analysis workflow is not available in this build.", 501, "document_analysis_workflow_unavailable")
			return
		}
		result, err = store.CreateComparison(owner, body.RequestKey, body.Sources)
		if err == nil && result.State == "queued" {
			err = s.ragEngine.StartComparison(owner, result.ID)
			if err == nil {
				result, err = store.Analysis(owner, result.ID)
			}
		}
	}
	if err != nil {
		writeAnalysisError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if r.Method == "POST" {
		w.WriteHeader(http.StatusAccepted)
	}
	_ = json.NewEncoder(w).Encode(result)
}

func (s *Server) handleDocumentAnalysisControl(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != "POST" {
		writeError(w, "Method not allowed", 405)
		return
	}
	if !s.requireKnowledgeStorage(w) {
		return
	}
	var body struct {
		ID string `json:"id"`
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	d.DisallowUnknownFields()
	if d.Decode(&body) != nil || d.Decode(new(any)) != io.EOF || body.ID == "" || len(body.ID) > 128 {
		writeAnalysisError(w, rag.ErrAnalysisInvalid)
		return
	}
	var err error
	if r.URL.Path == "/v1/document-analyses/cancel" {
		err = s.ragEngine.CancelAnalysis(modelActor(r), body.ID)
	} else {
		err = s.ragEngine.StartComparison(modelActor(r), body.ID)
	}
	if err != nil {
		writeAnalysisError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"accepted": true})
}

func (s *Server) handleDocumentAnalysisResults(w http.ResponseWriter, r *http.Request) {
	select {
	case analysisResponseReaders <- struct{}{}:
		defer func() { <-analysisResponseReaders }()
	case <-r.Context().Done():
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != "GET" {
		writeError(w, "Method not allowed", 405)
		return
	}
	if !s.requireKnowledgeStorage(w) {
		return
	}
	q := r.URL.Query()
	id := q.Get("id")
	if id == "" || len(id) > 128 {
		writeAnalysisError(w, rag.ErrAnalysisInvalid)
		return
	}
	values := map[string]int{"step": 0, "offset": 0, "limit": 100}
	for key := range values {
		if q.Has(key) {
			v, err := strconv.Atoi(q.Get(key))
			if err != nil {
				writeAnalysisError(w, rag.ErrAnalysisInvalid)
				return
			}
			values[key] = v
		}
	}
	store, err := s.ragEngine.ImportStore()
	if err != nil {
		writeAnalysisError(w, err)
		return
	}
	page, err := store.AnalysisResults(modelActor(r), id, values["step"], values["offset"], values["limit"])
	if err != nil {
		writeAnalysisError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(page)
}

func (s *Server) handleDocumentAnalysisSource(w http.ResponseWriter, r *http.Request) {
	select {
	case analysisResponseReaders <- struct{}{}:
		defer func() { <-analysisResponseReaders }()
	case <-r.Context().Done():
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != "GET" {
		writeError(w, "Method not allowed", 405)
		return
	}
	if !s.requireKnowledgeStorage(w) {
		return
	}
	q := r.URL.Query()
	step, err := strconv.Atoi(q.Get("step"))
	change, changeErr := strconv.Atoi(q.Get("change"))
	if err != nil || changeErr != nil || q.Get("id") == "" || len(q.Get("id")) > 128 {
		writeAnalysisError(w, rag.ErrAnalysisInvalid)
		return
	}
	store, err := s.ragEngine.ImportStore()
	if err != nil {
		writeAnalysisError(w, err)
		return
	}
	result, err := store.ReadAnalysisSource(modelActor(r), q.Get("id"), step, change, q.Get("side"))
	if err != nil {
		writeAnalysisError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}
