package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// handleBenchmark benchmarks a model's performance
func (s *Server) handleBenchmark(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	var req struct {
		Model        string `json:"model"`
		PromptTokens int    `json:"prompt_tokens"` // Default 512
		OutputTokens int    `json:"output_tokens"` // Default 128
		Iterations   int    `json:"iterations"`    // Default 3
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Model == "" {
		writeError(w, "Model is required", http.StatusBadRequest)
		return
	}

	// Set defaults
	if req.PromptTokens == 0 {
		req.PromptTokens = 512
	}
	if req.OutputTokens == 0 {
		req.OutputTokens = 128
	}
	if req.Iterations == 0 {
		req.Iterations = 3
	}

	// Get model
	_, err := s.registry.GetModel(req.Model)
	if err != nil {
		writeError(w, fmt.Sprintf("Model not found: %s", req.Model), http.StatusNotFound)
		return
	}
	lease, err := s.acquireInferenceLease(r.Context(), req.Model, s.modelContextWindow(req.Model), nil)
	if err != nil {
		writeServiceError(w, newServiceError(http.StatusServiceUnavailable, "Model could not be loaded; check runtime availability", err))
		return
	}
	defer lease.Release()

	// Keep every iteration and the pause between iterations bound to the
	// request. A disconnected caller must not retain the shared model lease.
	ctx := r.Context()
	// Run benchmark
	log.Printf("Running benchmark: %s (prompt=%d, output=%d, iterations=%d)",
		req.Model, req.PromptTokens, req.OutputTokens, req.Iterations)

	type BenchmarkRun struct {
		PromptTokensPerSec     float64 `json:"prompt_tokens_per_sec"`
		GenerationTokensPerSec float64 `json:"generation_tokens_per_sec"`
		TotalTimeMs            int64   `json:"total_time_ms"`
		MemoryUsedMB           int64   `json:"memory_used_mb"`
	}

	runs := make([]BenchmarkRun, 0, req.Iterations)

	// Generate test prompt of appropriate length
	testPrompt := generateTestPrompt(req.PromptTokens)

	for i := 0; i < req.Iterations; i++ {
		startMem := s.monitor.GetStats().MemoryUsedMB
		startTime := time.Now()

		// Create chat request
		chatReq := &api.ChatCompletionRequest{
			Model: req.Model,
			Messages: []api.ChatMessage{
				{Role: "user", Content: testPrompt},
			},
			MaxTokens: &req.OutputTokens,
		}

		var resp *api.ChatCompletionResponse
		err := lease.Run(ctx, func(ctx context.Context) error {
			var callErr error
			resp, callErr = s.engine.ChatCompletion(ctx, chatReq)
			return callErr
		})
		if err != nil {
			writeError(w, fmt.Sprintf("Benchmark failed: %v", err), http.StatusInternalServerError)
			return
		}

		duration := time.Since(startTime)
		endMem := s.monitor.GetStats().MemoryUsedMB

		promptTPS := float64(resp.Usage.PromptTokens) / duration.Seconds()
		genTPS := float64(resp.Usage.CompletionTokens) / duration.Seconds()

		runs = append(runs, BenchmarkRun{
			PromptTokensPerSec:     promptTPS,
			GenerationTokensPerSec: genTPS,
			TotalTimeMs:            duration.Milliseconds(),
			MemoryUsedMB:           int64(endMem - startMem),
		})

		// Small delay between runs; Stop must also interrupt this idle period.
		if i+1 < req.Iterations {
			pause := time.NewTimer(500 * time.Millisecond)
			select {
			case <-pause.C:
			case <-ctx.Done():
				pause.Stop()
				return
			}
		}
	}

	// Calculate averages
	var avgPromptTPS, avgGenTPS, avgTimeMs, avgMemMB float64
	for _, run := range runs {
		avgPromptTPS += run.PromptTokensPerSec
		avgGenTPS += run.GenerationTokensPerSec
		avgTimeMs += float64(run.TotalTimeMs)
		avgMemMB += float64(run.MemoryUsedMB)
	}
	n := float64(len(runs))
	avgPromptTPS /= n
	avgGenTPS /= n
	avgTimeMs /= n
	avgMemMB /= n

	response := map[string]interface{}{
		"model": req.Model,
		"config": map[string]int{
			"prompt_tokens": req.PromptTokens,
			"output_tokens": req.OutputTokens,
			"iterations":    req.Iterations,
		},
		"results": map[string]interface{}{
			"avg_prompt_tokens_per_sec":     avgPromptTPS,
			"avg_generation_tokens_per_sec": avgGenTPS,
			"avg_total_time_ms":             avgTimeMs,
			"avg_memory_mb":                 avgMemMB,
			"runs":                          runs,
		},
		"system": s.monitor.GetStats(),
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}

// generateTestPrompt creates a test prompt of approximately the target token count
func generateTestPrompt(targetTokens int) string {
	// Rough estimate: 1 token ≈ 4 characters
	chars := targetTokens * 4
	base := "This is a test prompt for benchmarking purposes. "
	result := ""
	for len(result) < chars {
		result += base
	}
	return result[:chars]
}
