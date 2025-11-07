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
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", s.handleHealth)

	// API v1 routes (OpenAI-compatible)
	mux.HandleFunc("/v1/models", s.handleListModels)
	mux.HandleFunc("/v1/chat/completions", s.handleChatCompletions)
	mux.HandleFunc("/v1/completions", s.handleCompletions)

	// Statistics endpoint
	mux.HandleFunc("/stats", s.handleStats)

	// Web UI
	mux.HandleFunc("/ui", s.handleWebUI)
	mux.HandleFunc("/ui/", s.handleWebUI)

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

	log.Printf("Server starting on http://localhost:%d", s.config.ServerPort)
	log.Printf("API endpoints:")
	log.Printf("  GET  /health")
	log.Printf("  GET  /v1/models")
	log.Printf("  POST /v1/chat/completions")
	log.Printf("  POST /v1/completions")
	log.Printf("Web UI:")
	log.Printf("  http://localhost:%d/ui", s.config.ServerPort)

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

	// Build detailed health response
	health := map[string]interface{}{
		"status":  "healthy",
		"version": "0.1.0-alpha",
		"uptime":  "running", // TODO: Track actual uptime
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
	}

	if err := json.NewEncoder(w).Encode(health); err != nil {
		log.Printf("Error encoding health response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
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

// handleWebUI serves the web dashboard
func (s *Server) handleWebUI(w http.ResponseWriter, r *http.Request) {
	// Serve the web UI from web/ui directory
	http.ServeFile(w, r, "web/ui/index.html")
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
