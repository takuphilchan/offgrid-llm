package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/cache"
	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/internal/resource"
	"github.com/takuphilchan/offgrid-llm/internal/stats"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Server represents the HTTP server
type Server struct {
	httpServer   *http.Server
	config       *config.Config
	registry     *models.Registry
	engine       inference.Engine
	monitor      *resource.Monitor
	statsTracker *stats.Tracker
	cache        *cache.ResponseCache
	startTime    time.Time
}

// New creates a new server instance
func New() *Server {
	cfg := config.LoadConfig()
	return NewWithConfig(cfg)
}

// NewWithConfig creates a new server instance with provided config
func NewWithConfig(cfg *config.Config) *Server {
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Initialize components
	registry := models.NewRegistry(cfg.ModelsDir)

	// Choose engine based on configuration
	var engine inference.Engine
	if cfg.UseMockEngine {
		log.Println("Using mock engine (no real inference)")
		engine = inference.NewMockEngine()
	} else {
		log.Println("Using llama.cpp engine")
		engine = inference.NewLlamaEngine()
	}

	monitor := resource.NewMonitor(5 * time.Second)
	statsTracker := stats.NewTracker()

	// Initialize response cache (max 1000 entries, 1 hour TTL)
	responseCache := cache.NewResponseCache(1000, 1*time.Hour)
	responseCache.StartCleanupRoutine(15 * time.Minute)

	// Scan for available models
	if err := registry.ScanModels(); err != nil {
		log.Printf("Warning: Failed to scan models: %v", err)
	}

	// Start resource monitor
	monitor.Start()

	return &Server{
		config:       cfg,
		registry:     registry,
		engine:       engine,
		monitor:      monitor,
		statsTracker: statsTracker,
		cache:        responseCache,
		startTime:    time.Now(),
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Health check endpoints
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/ready", s.handleReady)
	mux.HandleFunc("/livez", s.handleLiveness)   // Kubernetes-style
	mux.HandleFunc("/readyz", s.handleReadiness) // Kubernetes-style

	// API v1 routes (OpenAI-compatible)
	mux.HandleFunc("/v1/models", s.handleListModels)
	mux.HandleFunc("/v1/chat/completions", s.handleChatCompletions)
	mux.HandleFunc("/v1/completions", s.handleCompletions)

	// Model search and discovery (OffGrid-specific)
	mux.HandleFunc("/v1/search", s.handleModelSearch)
	mux.HandleFunc("/v1/benchmark", s.handleBenchmark)

	// Statistics endpoint
	mux.HandleFunc("/stats", s.handleStats)

	// Cache management endpoints
	mux.HandleFunc("/cache/stats", s.handleCacheStats)
	mux.HandleFunc("/cache/clear", s.handleCacheClear)

	// Web UI - serve HTML/CSS/JS
	uiPath := "/var/lib/offgrid/web/ui"
	// Fallback to local development path if installed path doesn't exist
	if _, err := os.Stat(uiPath); os.IsNotExist(err) {
		uiPath = "web/ui"
	}

	// Serve static files
	fs := http.FileServer(http.Dir(uiPath))
	mux.Handle("/ui/", http.StripPrefix("/ui/", fs))

	// Root endpoint
	mux.HandleFunc("/", s.handleRoot)

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.config.ServerPort),
		Handler:      s.loggingMiddleware(mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 300 * time.Second, // Long timeout for LLM inference
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown handler
	go s.handleShutdown()

	// Improved startup message with brand aesthetic
	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Printf("  \033[38;5;45m◆\033[0m \033[1mOffGrid LLM Server\033[0m\n")
	fmt.Println("  ──────────────────────────────────────────────────")
	fmt.Println()
	fmt.Printf("  \033[38;5;45m→\033[0m Server:     http://localhost:%d\n", s.config.ServerPort)
	fmt.Printf("  \033[38;5;45m→\033[0m Web UI:     http://localhost:%d/ui\n", s.config.ServerPort)
	fmt.Printf("  \033[38;5;45m→\033[0m API Docs:   http://localhost:%d/health\n", s.config.ServerPort)
	fmt.Println()
	fmt.Println("  \033[38;5;141m◆\033[0m \033[1mEndpoints\033[0m")
	fmt.Println("  ──────────────────────────────────────────────────")
	fmt.Println("    GET  /health")
	fmt.Println("    GET  /v1/models")
	fmt.Println("    POST /v1/chat/completions")
	fmt.Println("    POST /v1/completions")
	fmt.Println("    POST /v1/search")
	fmt.Println("    POST /v1/benchmark")
	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	log.Printf("✓ Server listening on port %d", s.config.ServerPort)

	if err := s.httpServer.ListenAndServe(); err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

// handleShutdown handles graceful shutdown
func (s *Server) handleShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	<-sigChan
	log.Println("\nShutdown signal received...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		log.Printf("Shutdown error: %v", err)
	}

	log.Println("Server stopped")
}

// loggingMiddleware logs all HTTP requests
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		duration := time.Since(start)
		log.Printf("%s %s · %.2fms", r.Method, r.URL.Path, float64(duration.Microseconds())/1000)
	})
}

// Handler functions (placeholders for now)
func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"name":"OffGrid LLM","version":"0.1.0-alpha","status":"running"}`)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Get current resource usage
	stats := s.monitor.GetStats()

	// Get model count
	models := s.registry.ListModels()

	// Calculate uptime
	uptime := time.Since(s.startTime)
	uptimeStr := formatDuration(uptime)

	// Build detailed health response
	health := map[string]interface{}{
		"status":         "healthy",
		"version":        "0.1.0-alpha",
		"uptime":         uptimeStr,
		"uptime_seconds": int(uptime.Seconds()),
		"timestamp":      time.Now().Unix(),
		"system": map[string]interface{}{
			"cpu_percent":     stats.CPUUsagePercent,
			"memory_mb":       stats.MemoryUsedMB,
			"memory_total_mb": stats.MemoryTotalMB,
			"memory_percent":  stats.MemoryUsagePercent,
			"disk_free_gb":    stats.DiskTotalGB - stats.DiskUsedGB,
			"disk_total_gb":   stats.DiskTotalGB,
			"goroutines":      stats.NumGoroutines,
		},
		"models": map[string]interface{}{
			"available": len(models),
			"loaded":    0, // TODO: Track loaded models
		},
		"config": map[string]interface{}{
			"port":        s.config.ServerPort,
			"max_context": s.config.MaxContextSize,
			"threads":     s.config.NumThreads,
		},
		"stats": s.getAggregateStats(),
		"cache": s.cache.Stats(),
	}

	if err := json.NewEncoder(w).Encode(health); err != nil {
		log.Printf("Error encoding health response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// handleReady returns readiness status (Kubernetes readiness probe)
func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Check if models are available
	models := s.registry.ListModels()
	ready := len(models) > 0

	status := "ready"
	httpStatus := http.StatusOK

	if !ready {
		status = "not_ready"
		httpStatus = http.StatusServiceUnavailable
	}

	response := map[string]interface{}{
		"status":           status,
		"models_available": len(models),
		"timestamp":        time.Now().Unix(),
	}

	w.WriteHeader(httpStatus)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding readiness response: %v", err)
	}
}

// handleLiveness returns liveness status (Kubernetes liveness probe)
func (s *Server) handleLiveness(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

// handleReadiness is an alias for handleReady (Kubernetes-style)
func (s *Server) handleReadiness(w http.ResponseWriter, r *http.Request) {
	s.handleReady(w, r)
}

// formatDuration formats a duration in human-readable format
func formatDuration(d time.Duration) string {
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm %ds", days, hours, minutes, seconds)
	} else if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	} else if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	allStats := s.statsTracker.GetAllStats()

	response := map[string]interface{}{
		"models":    allStats,
		"aggregate": s.getAggregateStats(),
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding stats response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (s *Server) getAggregateStats() map[string]interface{} {
	allStats := s.statsTracker.GetAllStats()

	var totalRequests int64
	var totalTokens int64
	var totalDuration int64
	modelCount := len(allStats)

	for _, stat := range allStats {
		totalRequests += stat.TotalRequests
		totalTokens += stat.TotalTokens
		totalDuration += stat.TotalDurationMs
	}

	avgResponse := float64(0)
	if totalRequests > 0 {
		avgResponse = float64(totalDuration) / float64(totalRequests)
	}

	return map[string]interface{}{
		"total_requests":  totalRequests,
		"total_tokens":    totalTokens,
		"models_used":     modelCount,
		"avg_response_ms": avgResponse,
	}
}

func (s *Server) handleCacheStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	stats := s.cache.Stats()

	if err := json.NewEncoder(w).Encode(stats); err != nil {
		log.Printf("Error encoding cache stats: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
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

func (s *Server) handleListModels(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	models := s.registry.ListModels()
	response := api.ModelListResponse{
		Object: "list",
		Data:   models,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (s *Server) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Parse request
	var req api.ChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.Model == "" {
		writeError(w, "Model is required", http.StatusBadRequest)
		return
	}

	if len(req.Messages) == 0 {
		writeError(w, "Messages are required", http.StatusBadRequest)
		return
	}

	// Get model metadata
	modelMeta, err := s.registry.GetModel(req.Model)
	if err != nil {
		writeError(w, fmt.Sprintf("Model not found: %s", req.Model), http.StatusNotFound)
		return
	}

	// Load model if not loaded
	if !modelMeta.IsLoaded {
		if err := s.registry.LoadModel(req.Model); err != nil {
			writeError(w, fmt.Sprintf("Failed to load model: %v", err), http.StatusInternalServerError)
			return
		}

		// Load into engine
		ctx := context.Background()
		opts := inference.DefaultLoadOptions()
		opts.NumThreads = s.config.NumThreads
		opts.ContextSize = s.config.MaxContextSize

		if err := s.engine.Load(ctx, modelMeta.Path, opts); err != nil {
			writeError(w, fmt.Sprintf("Failed to load model into engine: %v", err), http.StatusInternalServerError)
			return
		}
	}

	// Handle streaming vs non-streaming
	if req.Stream {
		s.handleChatCompletionsStream(w, r, &req)
		return
	}

	// Perform inference
	ctx := context.Background()
	startTime := time.Now()
	response, err := s.engine.ChatCompletion(ctx, &req)
	duration := time.Since(startTime)

	if err != nil {
		writeError(w, fmt.Sprintf("Inference failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Record statistics
	totalTokens := int64(response.Usage.TotalTokens)
	s.statsTracker.RecordInference(req.Model, totalTokens, duration.Milliseconds())

	// Send response
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}

// handleChatCompletionsStream handles streaming chat completions using Server-Sent Events
func (s *Server) handleChatCompletionsStream(w http.ResponseWriter, r *http.Request, req *api.ChatCompletionRequest) {
	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	ctx := context.Background()
	chunkID := fmt.Sprintf("chatcmpl-%d", time.Now().Unix())
	tokenIndex := 0

	// Send tokens as they arrive
	err := s.engine.ChatCompletionStream(ctx, req, func(token string) error {
		chunk := api.ChatCompletionChunk{
			ID:      chunkID,
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Model:   req.Model,
			Choices: []api.ChatCompletionChoiceChunk{
				{
					Index: 0,
					Delta: api.ChatMessage{
						Role:    "assistant",
						Content: token,
					},
					FinishReason: nil,
				},
			},
		}

		// Encode and send chunk
		data, err := json.Marshal(chunk)
		if err != nil {
			return err
		}

		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
		tokenIndex++
		return nil
	})

	if err != nil {
		log.Printf("Streaming error: %v", err)
		// Send error as final chunk
		errChunk := map[string]interface{}{
			"error": map[string]string{
				"message": err.Error(),
				"type":    "stream_error",
			},
		}
		data, _ := json.Marshal(errChunk)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
		return
	}

	// Send final chunk with finish_reason
	finishReason := "stop"
	finalChunk := api.ChatCompletionChunk{
		ID:      chunkID,
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   req.Model,
		Choices: []api.ChatCompletionChoiceChunk{
			{
				Index:        0,
				Delta:        api.ChatMessage{},
				FinishReason: &finishReason,
			},
		},
	}

	data, _ := json.Marshal(finalChunk)
	fmt.Fprintf(w, "data: %s\n\n", data)
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

func (s *Server) handleCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Parse request
	var req api.CompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.Model == "" {
		writeError(w, "Model is required", http.StatusBadRequest)
		return
	}

	if req.Prompt == "" {
		writeError(w, "Prompt is required", http.StatusBadRequest)
		return
	}

	// Get model metadata
	modelMeta, err := s.registry.GetModel(req.Model)
	if err != nil {
		writeError(w, fmt.Sprintf("Model not found: %s", req.Model), http.StatusNotFound)
		return
	}

	// Load model if not loaded
	if !modelMeta.IsLoaded {
		if err := s.registry.LoadModel(req.Model); err != nil {
			writeError(w, fmt.Sprintf("Failed to load model: %v", err), http.StatusInternalServerError)
			return
		}

		// Load into engine
		ctx := context.Background()
		opts := inference.DefaultLoadOptions()
		opts.NumThreads = s.config.NumThreads
		opts.ContextSize = s.config.MaxContextSize

		if err := s.engine.Load(ctx, modelMeta.Path, opts); err != nil {
			writeError(w, fmt.Sprintf("Failed to load model into engine: %v", err), http.StatusInternalServerError)
			return
		}
	}

	// Perform inference
	ctx := context.Background()
	response, err := s.engine.Completion(ctx, &req)
	if err != nil {
		writeError(w, fmt.Sprintf("Inference failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Send response
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}

// handleModelSearch searches HuggingFace Hub for models
func (s *Server) handleModelSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Parse query parameters
	query := r.URL.Query().Get("query")
	author := r.URL.Query().Get("author")
	quant := r.URL.Query().Get("quantization")
	sortBy := r.URL.Query().Get("sort")

	filter := models.SearchFilter{
		Query:          query,
		Author:         author,
		Quantization:   quant,
		SortBy:         sortBy,
		OnlyGGUF:       true,
		ExcludeGated:   true,
		Limit:          20,
		ExcludePrivate: true,
	}

	// Allow JSON body for more complex filters
	if r.Method == http.MethodPost {
		if err := json.NewDecoder(r.Body).Decode(&filter); err != nil {
			writeError(w, "Invalid request body", http.StatusBadRequest)
			return
		}
	}

	hf := models.NewHuggingFaceClient()
	results, err := hf.SearchModels(filter)
	if err != nil {
		writeError(w, fmt.Sprintf("Search failed: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"total":   len(results),
		"results": results,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}

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
	modelMeta, err := s.registry.GetModel(req.Model)
	if err != nil {
		writeError(w, fmt.Sprintf("Model not found: %s", req.Model), http.StatusNotFound)
		return
	}

	// Load model if needed
	if !modelMeta.IsLoaded {
		if err := s.registry.LoadModel(req.Model); err != nil {
			writeError(w, fmt.Sprintf("Failed to load model: %v", err), http.StatusInternalServerError)
			return
		}

		ctx := context.Background()
		opts := inference.DefaultLoadOptions()
		opts.NumThreads = s.config.NumThreads
		opts.ContextSize = s.config.MaxContextSize

		if err := s.engine.Load(ctx, modelMeta.Path, opts); err != nil {
			writeError(w, fmt.Sprintf("Failed to load model into engine: %v", err), http.StatusInternalServerError)
			return
		}
	}

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

		ctx := context.Background()
		resp, err := s.engine.ChatCompletion(ctx, chatReq)
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

		// Small delay between runs
		time.Sleep(500 * time.Millisecond)
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

// handleWebUI serves the HTML UI
func (s *Server) handleWebUI(w http.ResponseWriter, r *http.Request) {
	// Determine UI path
	uiPath := "/var/lib/offgrid/web/ui/index.html"

	// Fallback to local development path
	if _, err := os.Stat(uiPath); os.IsNotExist(err) {
		uiPath = "web/ui/index.html"
	}

	// Serve index.html for SPA routing
	http.ServeFile(w, r, uiPath)
}

// writeError writes an error response
func writeError(w http.ResponseWriter, message string, statusCode int) {
	w.WriteHeader(statusCode)
	response := api.ErrorResponse{
		Error: api.ErrorDetail{
			Message: message,
			Type:    "api_error",
		},
	}
	json.NewEncoder(w).Encode(response)
}
