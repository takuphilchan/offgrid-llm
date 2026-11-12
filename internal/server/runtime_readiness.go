package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/users"
)

func (s *Server) handleRuntimePreparation(w http.ResponseWriter, r *http.Request) {
	permission := users.PermissionModels
	if r.Method != http.MethodGet {
		permission = users.PermissionModelsManage
	}
	s.requirePermissionWhenAuthEnabled(permission, s.serveRuntimePreparation)(w, r)
}

func (s *Server) serveRuntimePreparation(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		writeErrorWithCode(w, "Method not allowed", http.StatusMethodNotAllowed, "method_not_allowed")
		return
	}
	if s.modelCache == nil {
		writeErrorWithCode(w, "Native inference preparation is unavailable on this service.", http.StatusServiceUnavailable, "runtime_unavailable")
		return
	}
	if r.Method == http.MethodPost {
		if err := s.modelCache.PrepareRuntime(r.Context()); err != nil {
			if errors.Is(err, inference.ErrRuntimePreparationBusy) {
				w.WriteHeader(http.StatusConflict)
			} else {
				w.WriteHeader(http.StatusServiceUnavailable)
			}
			_ = json.NewEncoder(w).Encode(s.modelCache.RuntimeStatus())
			return
		}
	}
	_ = json.NewEncoder(w).Encode(s.modelCache.RuntimeStatus())
}
