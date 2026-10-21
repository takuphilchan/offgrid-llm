package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
)

func (s *Server) handleCacheStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Model cache stats
	modelCacheStats := s.modelCache.GetStats()

	// Response cache stats
	responseCacheStats := s.cache.Stats()

	stats := map[string]interface{}{
		"model_cache":    modelCacheStats,
		"response_cache": responseCacheStats,
	}

	if err := json.NewEncoder(w).Encode(stats); err != nil {
		log.Printf("Error encoding cache stats: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// handleLoadingProgress returns current model loading progress
func (s *Server) handleLoadingProgress(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	progress := s.modelCache.GetLoadingProgress()

	if err := json.NewEncoder(w).Encode(progress); err != nil {
		log.Printf("Error encoding loading progress: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// handleLoadingProgressStream provides SSE stream of loading progress
func (s *Server) handleLoadingProgressStream(w http.ResponseWriter, r *http.Request) {
	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Get progress tracker if available
	progress := s.modelCache.GetLoadingProgress()

	// Send initial state
	data, _ := json.Marshal(progress)
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()

	// Poll for updates (aggressive 100ms polling for fast feedback)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	ctx := r.Context()
	lastPhase := progress.Phase
	lastProgress := progress.Progress

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			progress := s.modelCache.GetLoadingProgress()

			// Only send if something changed
			if progress.Phase != lastPhase || progress.Progress != lastProgress {
				data, _ := json.Marshal(progress)
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()

				lastPhase = progress.Phase
				lastProgress = progress.Progress

				// Stop streaming when complete or failed
				if progress.Phase == inference.PhaseReady || progress.Phase == inference.PhaseFailed {
					return
				}
			}
		}
	}
}

// handleFastPrewarm initiates aggressive pre-warming of a model file
func (s *Server) handleFastPrewarm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ModelPath string `json:"model_path"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.ModelPath == "" {
		http.Error(w, "model_path required", http.StatusBadRequest)
		return
	}

	// Initiate fast pre-warming in background
	if err := s.modelCache.FastPrewarmModel(req.ModelPath); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "warming",
		"message": "Pre-warming initiated",
		"path":    req.ModelPath,
	})
}

func (s *Server) handleCacheClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.cache.Clear()

	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"status":  "success",
		"message": "Cache cleared",
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
