package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"github.com/takuphilchan/offgrid-llm/internal/users"
)

type compositionCreateRequest struct {
	RequestKey string `json:"request_key"`
	AnalysisID string `json:"analysis_id"`
	Model      string `json:"model"`
	Language   string `json:"language"`
}

func (s *Server) handleDocumentCompositions(w http.ResponseWriter, r *http.Request) {
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
	if r.Method == "GET" {
		if r.URL.Query().Has("id") {
			id := r.URL.Query().Get("id")
			if id == "" || len(id) > 128 {
				writeAnalysisError(w, rag.ErrAnalysisInvalid)
				return
			}
			job, e := store.Composition(r.Context(), owner, id)
			if e != nil {
				writeAnalysisError(w, e)
				return
			}
			writeCompositionJSON(w, 200, job.Status())
			return
		}
		parent := r.URL.Query().Get("analysis_id")
		offset, e := analysisQueryInteger(r, "offset", 0)
		if e != nil || parent == "" || len(parent) > 128 {
			writeAnalysisError(w, rag.ErrAnalysisInvalid)
			return
		}
		page, e := store.CompositionHistory(r.Context(), owner, parent, offset)
		if e != nil {
			writeAnalysisError(w, e)
			return
		}
		writeCompositionJSON(w, 200, page)
		return
	}
	if !s.scheduleActorPermission(owner, users.PermissionChat) || !s.scheduleActorPermission(owner, users.PermissionRAG) {
		writeAnalysisError(w, rag.ErrAnalysisUnavailable)
		return
	}
	var body compositionCreateRequest
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	d.DisallowUnknownFields()
	if d.Decode(&body) != nil || d.Decode(new(any)) != io.EOF || body.RequestKey == "" || len(body.RequestKey) > 128 || strings.ContainsAny(body.RequestKey, "\x00\r\n") || body.AnalysisID == "" || len(body.AnalysisID) > 128 || body.Model == "" || len(body.Model) > 256 || body.Language == "" || len(body.Language) > 35 {
		writeAnalysisError(w, rag.ErrAnalysisInvalid)
		return
	}
	// Check private ownership and every source before loading any model.
	parent, err := store.Synthesis(owner, body.AnalysisID)
	if err != nil {
		writeAnalysisError(w, err)
		return
	}
	if parent.Plan.Workflow != "summary" || parent.State != "awaiting_review" {
		writeAnalysisError(w, rag.ErrAnalysisConflict)
		return
	}
	select {
	case analysisResponseReaders <- struct{}{}:
		defer func() { <-analysisResponseReaders }()
	case <-r.Context().Done():
		return
	}
	provider := s.synthesisProvider(owner, body.RequestKey)
	if provider == nil {
		writeAnalysisError(w, rag.ErrAnalysisUnavailable)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	identity, err := provider.Prepare(ctx, body.Model, body.Language)
	if err != nil {
		writeAnalysisError(w, err)
		return
	}
	job, err := store.CreateComposition(ctx, owner, body.RequestKey, body.AnalysisID, identity)
	if err == nil && job.State == "queued" {
		err = s.ragEngine.StartComposition(owner, job.ID, provider)
		if err == nil {
			job, err = store.Composition(r.Context(), owner, job.ID)
		}
	}
	if err != nil {
		writeAnalysisError(w, err)
		return
	}
	writeCompositionJSON(w, 202, job.Status())
}

func writeCompositionJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (s *Server) handleDocumentCompositionReport(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != "GET" {
		writeError(w, "Method not allowed", 405)
		return
	}
	if !s.requireKnowledgeStorage(w) {
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" || len(id) > 128 {
		writeAnalysisError(w, rag.ErrAnalysisInvalid)
		return
	}
	select {
	case analysisResponseReaders <- struct{}{}:
		defer func() { <-analysisResponseReaders }()
	case <-r.Context().Done():
		return
	}
	store, err := s.ragEngine.ImportStore()
	if err != nil {
		writeAnalysisError(w, err)
		return
	}
	var result any
	if strings.HasSuffix(r.URL.Path, "/evidence") {
		section, e := analysisQueryInteger(r, "section", -1)
		offset, e2 := analysisQueryInteger(r, "offset", 0)
		if e != nil || e2 != nil {
			writeAnalysisError(w, rag.ErrAnalysisInvalid)
			return
		}
		result, err = store.CompositionEvidence(r.Context(), modelActor(r), id, section, offset)
	} else {
		result, err = store.CompositionReportView(r.Context(), modelActor(r), id)
	}
	if err != nil {
		writeAnalysisError(w, err)
		return
	}
	writeCompositionJSON(w, 200, result)
}

func (s *Server) handleDocumentCompositionControl(w http.ResponseWriter, r *http.Request) {
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
	owner := modelActor(r)
	var err error
	if strings.HasSuffix(r.URL.Path, "/cancel") {
		err = s.ragEngine.CancelComposition(owner, body.ID)
	} else {
		if !s.scheduleActorPermission(owner, users.PermissionChat) || !s.scheduleActorPermission(owner, users.PermissionRAG) {
			writeAnalysisError(w, rag.ErrAnalysisUnavailable)
			return
		}
		err = s.ragEngine.StartComposition(owner, body.ID, s.synthesisProvider(owner, body.ID))
	}
	if err != nil {
		writeAnalysisError(w, err)
		return
	}
	writeCompositionJSON(w, 200, map[string]bool{"accepted": true})
}
