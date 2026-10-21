package server

import (
	"encoding/json"
	"net/http"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
)

// handleHotSwap performs a fast model switch with minimal downtime
func (s *Server) handleHotSwap(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		http.Error(w, `{"error": "method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ModelID string `json:"model_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "invalid request"}`, http.StatusBadRequest)
		return
	}

	if req.ModelID == "" {
		http.Error(w, `{"error": "model_id required"}`, http.StatusBadRequest)
		return
	}

	// Look up model
	model, err := s.registry.GetModel(req.ModelID)
	if err != nil || model == nil {
		http.Error(w, `{"error": "model not found"}`, http.StatusNotFound)
		return
	}

	// Perform hot-swap under the scheduler's exclusive process owner. Older
	// isolated fixtures without a scheduler retain the compatibility path.
	var result *inference.HotSwapResult
	if s.providerScheduler != nil {
		_, err = s.providerScheduler.WithLegacyMaintenance(r.Context(), func(owner inference.LegacyMaintenance) error {
			var swapErr error
			result, swapErr = owner.HotSwap(req.ModelID, model.Path, model.ProjectorPath)
			return swapErr
		})
	} else {
		result, err = s.modelCache.HotSwapContext(r.Context(), req.ModelID, model.Path, model.ProjectorPath)
	}
	if err != nil {
		json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(result)
}

// handleHotSwapStatus returns hot-swap readiness information
func (s *Server) handleHotSwapStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	status := s.modelCache.GetHotSwapStatus()
	json.NewEncoder(w).Encode(status)
}

// handleHotSwapPrepare pre-warms a model for faster hot-swap later
func (s *Server) handleHotSwapPrepare(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		http.Error(w, `{"error": "method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ModelID string `json:"model_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "invalid request"}`, http.StatusBadRequest)
		return
	}

	if req.ModelID == "" {
		http.Error(w, `{"error": "model_id required"}`, http.StatusBadRequest)
		return
	}

	// Look up model
	model, err := s.registry.GetModel(req.ModelID)
	if err != nil || model == nil {
		http.Error(w, `{"error": "model not found"}`, http.StatusNotFound)
		return
	}

	// Prepare hot-swap
	err = s.modelCache.PrepareHotSwap(model.Path)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]any{
		"success":  true,
		"model_id": req.ModelID,
		"message":  "Model pre-warmed for hot-swap",
	})
}
