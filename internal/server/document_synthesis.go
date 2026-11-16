package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/document"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"github.com/takuphilchan/offgrid-llm/internal/users"
)

// This contract deliberately exposes reviewed-source candidates, not a claim
// that full-document synthesis/support qualification has already passed.
type synthesisCreateRequest struct {
	RequestKey string               `json:"request_key"`
	Workflow   string               `json:"workflow"`
	Sources    []document.SourceRef `json:"sources"`
	Model      string               `json:"model"`
	Language   string               `json:"language"`
}

func (s *Server) synthesisProvider(actor, work string) documentSynthesisRuntime {
	if s.documentSynthesisFactory != nil {
		return s.documentSynthesisFactory(actor, work)
	}
	return &localDocumentSynthesis{server: s, actor: actor, work: work}
}

func (s *Server) handleDocumentSynthesis(w http.ResponseWriter, r *http.Request) {
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
		if !r.URL.Query().Has("id") {
			offset, err := analysisQueryInteger(r, "offset", 0)
			if err != nil {
				writeAnalysisError(w, err)
				return
			}
			page, err := store.SynthesisHistory(owner, offset, 20)
			if err != nil {
				writeAnalysisError(w, err)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(page)
			return
		}
		id := r.URL.Query().Get("id")
		if id == "" || len(id) > 128 {
			writeAnalysisError(w, rag.ErrAnalysisInvalid)
			return
		}
		job, err := store.Synthesis(owner, id)
		if err != nil {
			writeAnalysisError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(job)
		return
	}
	if !s.scheduleActorPermission(owner, users.PermissionChat) || !s.scheduleActorPermission(owner, users.PermissionRAG) {
		writeAnalysisError(w, rag.ErrAnalysisUnavailable)
		return
	}
	var body synthesisCreateRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&body) != nil || decoder.Decode(new(any)) != io.EOF || (body.Workflow != "summary" && body.Workflow != "requirements") || len(body.Sources) < 1 || len(body.Sources) > 16 || body.RequestKey == "" || len(body.RequestKey) > 128 || body.Model == "" || len(body.Model) > 256 {
		writeAnalysisError(w, rag.ErrAnalysisInvalid)
		return
	}
	// Check audience and pinned revisions before admitting any inference work.
	if err = store.ValidateAnalysisSources(owner, body.Workflow, body.Sources); err != nil {
		writeAnalysisError(w, err)
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
	job, err := store.CreateSynthesis(owner, body.RequestKey, body.Workflow, body.Sources, identity, 1024)
	if err == nil && job.State == "queued" {
		err = s.ragEngine.StartSynthesis(owner, job.ID, provider)
		if err == nil {
			job, err = store.Synthesis(owner, job.ID)
		}
	}
	if err != nil {
		writeAnalysisError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(202)
	_ = json.NewEncoder(w).Encode(job)
}

func analysisQueryInteger(r *http.Request, key string, fallback int) (int, error) {
	if !r.URL.Query().Has(key) {
		return fallback, nil
	}
	v, err := strconv.Atoi(r.URL.Query().Get(key))
	if err != nil {
		return 0, rag.ErrAnalysisInvalid
	}
	return v, nil
}

func (s *Server) handleDocumentSynthesisResults(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != "GET" {
		writeError(w, "Method not allowed", 405)
		return
	}
	if !s.requireKnowledgeStorage(w) {
		return
	}
	select {
	case analysisResponseReaders <- struct{}{}:
		defer func() { <-analysisResponseReaders }()
	case <-r.Context().Done():
		return
	}
	id := r.URL.Query().Get("id")
	offset, err := analysisQueryInteger(r, "offset", 0)
	if err != nil || id == "" || len(id) > 128 {
		writeAnalysisError(w, rag.ErrAnalysisInvalid)
		return
	}
	store, err := s.ragEngine.ImportStore()
	if err != nil {
		writeAnalysisError(w, err)
		return
	}
	page, err := store.SynthesisResults(modelActor(r), id, offset, 2)
	if err != nil {
		writeAnalysisError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(page)
}

func (s *Server) handleDocumentSynthesisRetry(w http.ResponseWriter, r *http.Request) {
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
	if !s.scheduleActorPermission(owner, users.PermissionChat) || !s.scheduleActorPermission(owner, users.PermissionRAG) {
		writeAnalysisError(w, rag.ErrAnalysisUnavailable)
		return
	}
	err := s.ragEngine.StartSynthesis(owner, body.ID, s.synthesisProvider(owner, body.ID))
	if err != nil {
		writeAnalysisError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"accepted": true})
}
