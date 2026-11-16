package server

import (
	"context"
	"encoding/json"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"net/http"
	"strings"
	"time"
)

var analysisExportReaders = make(chan struct{}, 1)

func (s *Server) handleDocumentAnalysisExport(w http.ResponseWriter, r *http.Request) {
	s.handleAnalysisExport(w, r, false)
}

func (s *Server) handleDocumentSynthesisExport(w http.ResponseWriter, r *http.Request) {
	s.handleAnalysisExport(w, r, true)
}

func (s *Server) handleAnalysisExport(w http.ResponseWriter, r *http.Request, synthesis bool) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet {
		writeError(w, "Method not allowed", 405)
		return
	}
	if !s.requireKnowledgeStorage(w) {
		return
	}
	q := r.URL.Query()
	id, format := q.Get("id"), q.Get("format")
	if id == "" || len(id) > 128 || (format != "json" && format != "csv" && format != "markdown") {
		writeAnalysisError(w, rag.ErrAnalysisInvalid)
		return
	}
	select {
	case analysisExportReaders <- struct{}{}:
		defer func() { <-analysisExportReaders }()
	default:
		writeAnalysisError(w, rag.ErrAnalysisLimit)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	store, err := s.ragEngine.ImportStore()
	if err != nil {
		writeAnalysisError(w, err)
		return
	}
	var result *rag.AnalysisExport
	if strings.Contains(r.URL.Path, "/compositions/") {
		result, err = store.ExportComposition(ctx, modelActor(r), id, format)
		if err == nil {
			_, err = store.Composition(ctx, modelActor(r), id)
		}
	} else if synthesis {
		result, err = store.ExportSynthesis(ctx, modelActor(r), id, format)
		if err == nil {
			_, err = store.Synthesis(modelActor(r), id)
		}
	} else {
		result, err = store.ExportComparison(ctx, modelActor(r), id, format)
		if err == nil {
			_, err = store.Analysis(modelActor(r), id)
		}
	}
	if err != nil {
		writeAnalysisError(w, err)
		return
	}
	if ctx.Err() != nil {
		writeAnalysisError(w, ctx.Err())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}
