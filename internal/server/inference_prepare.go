package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Preparation uses ordinary chat authority and admission, never management
// authority or a default-model write. It runs no prompt and grants no tools.
func (s *Server) handleInferencePrepare(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var request struct {
		Model string `json:"model"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&request) != nil || decoder.Decode(new(any)) != io.EOF || request.Model == "" {
		writeError(w, "A model ID is required; preparation accepts no prompt or tool controls", http.StatusBadRequest)
		return
	}
	if err := s.authorizeChat(r.Context(), &api.ChatCompletionRequest{Model: request.Model}); err != nil {
		writeServiceError(w, err)
		return
	}
	metadata, err := s.registry.GetChatModel(request.Model)
	if err != nil || metadata.Format != "" && metadata.Format != "gguf" {
		writeErrorWithCode(w, "Selected chat model is unavailable for preparation", http.StatusBadRequest, "model_unavailable")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	started := time.Now()
	lease, err := s.acquireInferenceLease(ctx, request.Model, s.modelContextWindow(request.Model), nil)
	if err != nil {
		writeServiceError(w, newServiceError(http.StatusServiceUnavailable, "Model preparation failed; no external runtime was started", err))
		return
	}
	defer lease.Release()
	writeJSON(w, http.StatusOK, map[string]any{"model": request.Model, "prepared": true, "context_window": s.modelContextWindow(request.Model), "elapsed_ms": time.Since(started).Milliseconds()})
}
