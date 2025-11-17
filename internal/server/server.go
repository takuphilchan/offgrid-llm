package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/cache"
	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/internal/resource"
	"github.com/takuphilchan/offgrid-llm/internal/stats"
	"github.com/takuphilchan/offgrid-llm/internal/templates"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Server represents the HTTP server
type Server struct {
	httpServer           *http.Server
	config               *config.Config
	registry             *models.Registry
	engine               inference.Engine
	embeddingEngine      *inference.EmbeddingEngine
	monitor              *resource.Monitor
	statsTracker         *stats.Tracker
	cache                *cache.ResponseCache
	startTime            time.Time
	downloadProgress     map[string]*DownloadProgress
	downloadMutex        sync.RWMutex
	exportProgress       map[string]*ExportProgress
	exportMutex          sync.RWMutex
	llamaServerCmd       *exec.Cmd
	currentModelID       string
	modelMutex           sync.Mutex
	inferenceMutex       sync.Mutex // Ensures only one inference runs at a time
	rateLimiter          *RateLimiter
	inferenceRateLimiter *InferenceRateLimiter
}

type DownloadProgress struct {
	FileName   string  `json:"file_name"`
	BytesTotal int64   `json:"bytes_total"`
	BytesDone  int64   `json:"bytes_done"`
	Percent    float64 `json:"percent"`
	Status     string  `json:"status"` // "downloading", "complete", "failed"
	Error      string  `json:"error,omitempty"`
}

type ExportProgress struct {
	FileName   string  `json:"file_name"`
	BytesTotal int64   `json:"bytes_total"`
	BytesDone  int64   `json:"bytes_done"`
	Percent    float64 `json:"percent"`
	Status     string  `json:"status"` // "exporting", "complete", "failed"
	Error      string  `json:"error,omitempty"`
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

	// Initialize embedding engine
	embeddingEngine := inference.NewEmbeddingEngine()

	// Scan for available models
	if err := registry.ScanModels(); err != nil {
		log.Printf("Warning: Failed to scan models: %v", err)
	}

	// Start resource monitor
	monitor.Start()

	// Initialize rate limiters
	// General API: 60 requests per minute with burst of 10
	rateLimiter := NewRateLimiter(60, time.Minute, 10)

	// Inference endpoints: max 2 concurrent per IP, 3 global concurrent
	inferenceRateLimiter := NewInferenceRateLimiter(2, 3)

	return &Server{
		config:               cfg,
		registry:             registry,
		engine:               engine,
		embeddingEngine:      embeddingEngine,
		monitor:              monitor,
		statsTracker:         statsTracker,
		cache:                responseCache,
		startTime:            time.Now(),
		downloadProgress:     make(map[string]*DownloadProgress),
		exportProgress:       make(map[string]*ExportProgress),
		rateLimiter:          rateLimiter,
		inferenceRateLimiter: inferenceRateLimiter,
	}
}

// startLlamaServer starts the llama-server process with a default model
func (s *Server) startLlamaServer() error {
	// Check if llama-server is already running
	resp, err := http.Get("http://localhost:42382/health")
	if err == nil {
		resp.Body.Close()
		log.Println("llama-server already running on port 42382")
		return nil
	}

	// Scan for models first
	if err := s.registry.ScanModels(); err != nil {
		return fmt.Errorf("failed to scan models: %w", err)
	}

	// Find a model to use (prefer TinyLlama for fast startup)
	installedModels := s.registry.ListModels()
	if len(installedModels) == 0 {
		return fmt.Errorf("no models installed - run 'offgrid download' first")
	}

	// Prioritize TinyLlama for fast startup, otherwise use first available
	var modelID string
	for _, model := range installedModels {
		if strings.Contains(strings.ToLower(model.ID), "tinyllama") {
			modelID = model.ID
			log.Printf("Using TinyLlama for fast startup: %s", model.ID)
			break
		}
	}

	// Fallback to first model if no TinyLlama found
	if modelID == "" {
		modelID = installedModels[0].ID
		log.Printf("Using model: %s", installedModels[0].ID)
	}

	// Get model metadata to get the actual path
	metadata, err := s.registry.GetModel(modelID)
	if err != nil {
		return fmt.Errorf("failed to get model metadata: %w", err)
	}

	// Check if model file exists
	if _, err := os.Stat(metadata.Path); os.IsNotExist(err) {
		return fmt.Errorf("model file not found: %s", metadata.Path)
	}

	// Kill any existing llama-server processes to prevent duplicates
	if s.llamaServerCmd != nil && s.llamaServerCmd.Process != nil {
		log.Println("Stopping existing llama-server process...")
		s.llamaServerCmd.Process.Kill()
		s.llamaServerCmd.Wait()
		s.llamaServerCmd = nil
		time.Sleep(1 * time.Second) // Give it time to clean up
	}

	// Start llama-server
	log.Printf("Starting llama-server on port 42382 with model: %s", modelID)
	cmd := exec.Command("llama-server",
		"-m", metadata.Path,
		"--port", "42382",
		"--host", "127.0.0.1",
		"-c", "2048",
		"-ngl", "0", // CPU-only for compatibility
	)

	// Set environment to bypass proxy
	cmd.Env = append(os.Environ(), "NO_PROXY=*")

	// Redirect output to logs
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start llama-server: %w", err)
	}

	s.llamaServerCmd = cmd

	// Wait for llama-server to be ready
	log.Println("Waiting for llama-server to load model...")
	for i := 0; i < 120; i++ { // Wait up to 120 seconds
		time.Sleep(1 * time.Second)
		resp, err := http.Get("http://localhost:42382/health")
		if err == nil {
			var health map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&health)
			resp.Body.Close()

			if status, ok := health["status"].(string); ok && status == "ok" {
				s.currentModelID = modelID
				log.Printf("llama-server ready (loaded in %d seconds)", i+1)
				return nil
			}
			log.Printf("Model loading... (%d seconds)", i+1)
		}
	}

	return fmt.Errorf("llama-server did not become ready within 120 seconds")
}

// switchModel stops the current llama-server and starts a new one with the requested model.
// This ensures complete context isolation between models - when switching models, the entire
// llama-server process is restarted with a fresh context. This prevents any context mixup
// or conversation leakage between different models. Clients should clear their chat history
// when switching models to maintain proper conversation boundaries.
func (s *Server) switchModel(modelID string) error {
	s.modelMutex.Lock()
	defer s.modelMutex.Unlock()

	// If already loaded, skip
	if s.currentModelID == modelID {
		log.Printf("Model %s already loaded, skipping switch", modelID)
		return nil
	}

	log.Printf("Switching from %s to %s...", s.currentModelID, modelID)

	// Stop current llama-server
	if s.llamaServerCmd != nil && s.llamaServerCmd.Process != nil {
		log.Println("Stopping current llama-server...")
		if err := s.llamaServerCmd.Process.Kill(); err != nil {
			log.Printf("Warning: error killing llama-server: %v", err)
		}
		s.llamaServerCmd.Wait()
		s.llamaServerCmd = nil
		s.currentModelID = ""
		time.Sleep(500 * time.Millisecond) // Brief pause for cleanup
	}

	// Get model metadata
	metadata, err := s.registry.GetModel(modelID)
	if err != nil {
		return fmt.Errorf("model not found: %w", err)
	}

	// Start llama-server with new model
	log.Printf("Starting llama-server with model: %s", modelID)
	cmd := exec.Command("llama-server",
		"-m", metadata.Path,
		"--port", "42382",
		"--host", "127.0.0.1",
		"-c", "2048",
		"-ngl", "0",
	)

	cmd.Env = append(os.Environ(), "NO_PROXY=*")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start llama-server: %w", err)
	}

	s.llamaServerCmd = cmd

	// Wait for ready
	log.Println("Waiting for model to load...")
	for i := 0; i < 120; i++ {
		time.Sleep(1 * time.Second)
		resp, err := http.Get("http://localhost:42382/health")
		if err == nil {
			var health map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&health)
			resp.Body.Close()

			if status, ok := health["status"].(string); ok && status == "ok" {
				s.currentModelID = modelID
				log.Printf("Model %s loaded successfully in %d seconds", modelID, i+1)
				return nil
			}
		}
	}

	return fmt.Errorf("model failed to load within 120 seconds")
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
	mux.HandleFunc("/v1/models", s.rateLimiter.Middleware(s.handleListModels))
	mux.HandleFunc("/v1/models/delete", s.rateLimiter.Middleware(s.handleDeleteModel))
	mux.HandleFunc("/v1/models/download", s.rateLimiter.Middleware(s.handleDownloadModel))
	mux.HandleFunc("/v1/models/download/progress", s.handleDownloadProgress)

	// Inference endpoints with strict rate limiting
	mux.HandleFunc("/v1/chat/completions", s.inferenceRateLimiter.Middleware(s.handleChatCompletions))
	mux.HandleFunc("/v1/completions", s.inferenceRateLimiter.Middleware(s.handleCompletions))
	mux.HandleFunc("/v1/embeddings", s.inferenceRateLimiter.Middleware(s.handleEmbeddings))

	// Model search and discovery (OffGrid-specific)
	mux.HandleFunc("/v1/search", s.handleModelSearch)
	mux.HandleFunc("/v1/catalog", s.handleModelCatalog)
	mux.HandleFunc("/v1/benchmark", s.handleBenchmark)
	mux.HandleFunc("/v1/terminal/exec", s.handleTerminalExec)

	// Templates endpoints
	mux.HandleFunc("/v1/templates", s.handleTemplates)
	mux.HandleFunc("/v1/templates/", s.handleTemplateDetails)

	// USB import/export
	mux.HandleFunc("/v1/usb/scan", s.handleUSBScan)
	mux.HandleFunc("/v1/usb/import", s.handleUSBImport)
	mux.HandleFunc("/v1/usb/export", s.handleUSBExport)
	mux.HandleFunc("/v1/usb/export/progress", s.handleExportProgress)

	// Statistics endpoint
	mux.HandleFunc("/stats", s.handleStats)

	// Cache management endpoints
	mux.HandleFunc("/cache/stats", s.handleCacheStats)
	mux.HandleFunc("/cache/clear", s.handleCacheClear)

	// Simplified UI endpoints (no /v1 prefix for easier frontend access)
	mux.HandleFunc("/models", s.handleListModels)
	mux.HandleFunc("/catalog", s.handleModelCatalog)

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

	// Auto-start llama-server if using LlamaHTTPEngine
	if err := s.startLlamaServer(); err != nil {
		log.Printf("Warning: Failed to auto-start llama-server: %v", err)
		log.Println("You may need to start llama-server manually")
	}

	// Clean startup message with colors
	const (
		colorReset   = "\033[0m"
		colorCyan    = "\033[36m"
		colorGreen   = "\033[32m"
		brandPrimary = "\033[38;5;45m"
	)

	fmt.Println()
	fmt.Printf("%s┌─%s OffGrid LLM Server\n", colorCyan, colorReset)
	fmt.Printf("%s│%s\n", colorCyan, colorReset)
	fmt.Printf("%s│%s Server:  http://localhost:%d\n", colorCyan, colorReset, s.config.ServerPort)
	fmt.Printf("%s│%s Web UI:  http://localhost:%d/ui/\n", colorCyan, colorReset, s.config.ServerPort)
	fmt.Printf("%s│%s Health:  http://localhost:%d/health\n", colorCyan, colorReset, s.config.ServerPort)
	fmt.Printf("%s│%s\n", colorCyan, colorReset)
	fmt.Printf("%s│%s OpenAI-Compatible API Endpoints:\n", colorCyan, colorReset)
	fmt.Printf("%s│%s   POST /v1/chat/completions\n", colorCyan, colorReset)
	fmt.Printf("%s│%s   POST /v1/completions\n", colorCyan, colorReset)
	fmt.Printf("%s│%s   POST /v1/embeddings\n", colorCyan, colorReset)
	fmt.Printf("%s│%s   GET  /v1/models\n", colorCyan, colorReset)
	fmt.Printf("%s│%s\n", colorCyan, colorReset)
	fmt.Printf("%s│%s %s[OK]%s Server ready on port %d\n", colorCyan, colorReset, colorGreen, colorReset, s.config.ServerPort)
	fmt.Printf("%s└─%s\n", colorCyan, colorReset)
	fmt.Println()

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

	// Stop llama-server if we started it
	if s.llamaServerCmd != nil && s.llamaServerCmd.Process != nil {
		log.Println("Stopping llama-server...")
		if err := s.llamaServerCmd.Process.Kill(); err != nil {
			log.Printf("Error stopping llama-server: %v", err)
		} else {
			s.llamaServerCmd.Wait() // Clean up zombie process
			log.Println("llama-server stopped")
		}
	}

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
		// Set CORS headers for browser access
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		start := time.Now()
		next.ServeHTTP(w, r)
		duration := time.Since(start)
		log.Printf("%s %s · %.2fms", r.Method, r.URL.Path, float64(duration.Microseconds())/1000)
	})
}

// Handler functions (placeholders for now)
func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	// Serve the UI at root
	if r.URL.Path == "/" {
		uiPath := "/var/lib/offgrid/web/ui/index.html"
		// Fallback to local development path if installed path doesn't exist
		if _, err := os.Stat(uiPath); os.IsNotExist(err) {
			uiPath = "web/ui/index.html"
		}
		http.ServeFile(w, r, uiPath)
		return
	}

	// API info for other paths
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"name":"OffGrid LLM","version":"0.1.5","status":"running"}`)
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

	// Acquire inference lock to ensure only one inference runs at a time
	// Use TryLock to avoid blocking - return error if busy
	if !s.inferenceMutex.TryLock() {
		writeError(w, "Server is busy processing another request. Please try again in a moment.", http.StatusServiceUnavailable)
		log.Println("Rejected chat completion request - inference already in progress")
		return
	}
	defer s.inferenceMutex.Unlock()

	// Get model metadata
	modelMeta, err := s.registry.GetModel(req.Model)
	if err != nil {
		writeError(w, fmt.Sprintf("Model not found: %s", req.Model), http.StatusNotFound)
		return
	}

	// Switch to requested model if different from current
	if err := s.switchModel(req.Model); err != nil {
		writeError(w, fmt.Sprintf("Failed to switch model: %v", err), http.StatusInternalServerError)
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

	// Acquire inference lock to ensure only one inference runs at a time
	if !s.inferenceMutex.TryLock() {
		writeError(w, "Server is busy processing another request. Please try again in a moment.", http.StatusServiceUnavailable)
		log.Println("Rejected completion request - inference already in progress")
		return
	}
	defer s.inferenceMutex.Unlock()

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

// handleEmbeddings handles POST /v1/embeddings for generating text embeddings
func (s *Server) handleEmbeddings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Parse request
	var req api.EmbeddingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.Model == "" {
		writeError(w, "Model is required", http.StatusBadRequest)
		return
	}

	if req.Input == nil {
		writeError(w, "Input is required", http.StatusBadRequest)
		return
	}

	// Check if it's an embedding model
	modelMeta, err := s.registry.GetModel(req.Model)
	if err != nil {
		writeError(w, fmt.Sprintf("Model not found: %s", req.Model), http.StatusNotFound)
		return
	}

	// Load embedding model if not loaded
	if !s.embeddingEngine.IsLoaded() || s.embeddingEngine.GetModelInfo()["model_path"] != modelMeta.Path {
		log.Printf("Loading embedding model: %s", req.Model)

		ctx := context.Background()
		opts := inference.DefaultEmbeddingOptions()
		opts.NumThreads = s.config.NumThreads

		// Check for GPU availability
		gpuMonitor := resource.NewGPUMonitor()
		if gpuMonitor.IsAvailable() {
			// Offload some layers to GPU if available
			opts.NumGPULayers = 10 // Conservative for embedding models
		}

		if err := s.embeddingEngine.Load(ctx, modelMeta.Path, opts); err != nil {
			writeError(w, fmt.Sprintf("Failed to load embedding model: %v", err), http.StatusInternalServerError)
			return
		}

		// Mark model as loaded in registry
		if err := s.registry.LoadModel(req.Model); err != nil {
			log.Printf("Warning: Failed to mark model as loaded: %v", err)
		}
	}

	// Generate embeddings
	ctx := context.Background()
	startTime := time.Now()
	response, err := s.embeddingEngine.GenerateEmbeddings(ctx, &req)
	if err != nil {
		writeError(w, fmt.Sprintf("Failed to generate embeddings: %v", err), http.StatusInternalServerError)
		return
	}

	// Track stats
	duration := time.Since(startTime)
	s.statsTracker.RecordInference(req.Model, int64(response.Usage.TotalTokens), duration.Milliseconds())

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
	if query == "" {
		// Return empty results instead of error
		response := map[string]interface{}{
			"total":   0,
			"results": []interface{}{},
		}
		json.NewEncoder(w).Encode(response)
		return
	}

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
		// Return empty results on error instead of 500
		log.Printf("Search error: %v", err)
		response := map[string]interface{}{
			"total":   0,
			"results": []interface{}{},
			"error":   err.Error(),
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Transform results to include computed fields for UI
	type UIModel struct {
		ID              string   `json:"id"`
		Author          string   `json:"author"`
		Name            string   `json:"name"`
		Description     string   `json:"description,omitempty"`
		Downloads       int64    `json:"downloads"`
		Likes           int      `json:"likes"`
		Tags            []string `json:"tags,omitempty"`
		TotalSize       int64    `json:"total_size,omitempty"`
		SizeGB          string   `json:"size_gb,omitempty"`
		BestFile        string   `json:"best_file,omitempty"`
		BestQuant       string   `json:"best_quant,omitempty"`
		DownloadCommand string   `json:"download_command,omitempty"`
	}

	uiModels := make([]UIModel, 0, len(results))
	for _, result := range results {
		// Extract author from model ID
		modelID := result.Model.ID
		if modelID == "" {
			modelID = result.Model.ModelID
		}

		author := result.Model.Author
		name := modelID

		// Parse author from ID if not explicitly set
		if author == "" && strings.Contains(modelID, "/") {
			parts := strings.SplitN(modelID, "/", 2)
			if len(parts) == 2 {
				author = parts[0]
				name = parts[1]
			}
		}

		// Calculate size in GB from best variant
		sizeGB := float64(result.TotalSize) / (1024 * 1024 * 1024)
		bestFile := ""
		bestQuant := ""
		downloadCmd := ""

		if result.BestVariant != nil {
			sizeGB = result.BestVariant.SizeGB
			bestFile = result.BestVariant.Filename
			bestQuant = result.BestVariant.Quantization
			downloadCmd = fmt.Sprintf("offgrid download-hf %s --file %s", modelID, bestFile)
		}

		uiModels = append(uiModels, UIModel{
			ID:              modelID,
			Author:          author,
			Name:            name,
			Description:     result.Model.Description,
			Downloads:       result.Model.Downloads,
			Likes:           result.Model.Likes,
			Tags:            result.Model.Tags,
			TotalSize:       result.TotalSize,
			SizeGB:          fmt.Sprintf("%.2f", sizeGB),
			BestFile:        bestFile,
			BestQuant:       bestQuant,
			DownloadCommand: downloadCmd,
		})
	}

	response := map[string]interface{}{
		"total":   len(uiModels),
		"results": uiModels,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}

// handleDeleteModel deletes a model from the registry and filesystem
func (s *Server) handleDeleteModel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	var req struct {
		ModelID string `json:"model_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.ModelID == "" {
		writeError(w, "model_id is required", http.StatusBadRequest)
		return
	}

	if err := s.registry.DeleteModel(req.ModelID); err != nil {
		writeError(w, fmt.Sprintf("Failed to delete model: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Model %s deleted successfully", req.ModelID),
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}

// handleTerminalExec executes offgrid commands from the browser terminal
func (s *Server) handleTerminalExec(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	var req struct {
		Command string   `json:"command"`
		Args    []string `json:"args"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Security: Only allow offgrid commands
	if req.Command != "offgrid" {
		writeError(w, "Only 'offgrid' commands are allowed", http.StatusForbidden)
		return
	}

	// Find offgrid binary
	offgridPath, err := exec.LookPath("offgrid")
	if err != nil {
		writeError(w, "offgrid binary not found in PATH", http.StatusInternalServerError)
		return
	}

	// Execute command
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, offgridPath, req.Args...)

	// Capture output
	output, err := cmd.CombinedOutput()

	response := map[string]interface{}{
		"output":   string(output),
		"exitCode": 0,
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			response["exitCode"] = exitErr.ExitCode()
		} else {
			response["error"] = err.Error()
			response["exitCode"] = 1
		}
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}

// handleDownloadModel downloads a model from HuggingFace
func (s *Server) handleDownloadModel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	var req struct {
		Repository   string `json:"repository"`
		FileName     string `json:"file_name"`
		Quantization string `json:"quantization"` // Optional: just the quant like "Q4_K_M"
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Repository == "" {
		writeError(w, "repository is required", http.StatusBadRequest)
		return
	}

	// If only quantization is provided, fetch the model to find the actual filename
	if req.FileName == "" && req.Quantization != "" {
		hf := models.NewHuggingFaceClient()
		modelInfo, err := hf.GetModelInfo(req.Repository)
		if err == nil {
			// Find GGUF file with matching quantization
			for _, sibling := range modelInfo.Siblings {
				if strings.HasSuffix(sibling.Filename, ".gguf") &&
					strings.Contains(strings.ToUpper(sibling.Filename), req.Quantization) {
					req.FileName = sibling.Filename
					break
				}
			}
		}
	}

	if req.FileName == "" {
		writeError(w, "file_name is required or could not be determined", http.StatusBadRequest)
		return
	}

	// Start download in background
	go s.downloadModelAsync(req.Repository, req.FileName)

	response := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Download started for %s/%s", req.Repository, req.FileName),
		"status":  "downloading",
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}

func (s *Server) downloadModelAsync(repository, fileName string) {
	// Ensure fileName doesn't have double .gguf extension
	fileName = strings.TrimSuffix(fileName, ".gguf") + ".gguf"

	log.Printf("Starting download: %s/%s", repository, fileName)

	// Initialize progress tracking
	s.downloadMutex.Lock()
	s.downloadProgress[fileName] = &DownloadProgress{
		FileName: fileName,
		Status:   "downloading",
	}
	s.downloadMutex.Unlock()

	// Try different URL formats (some repos use different naming conventions)
	urls := []string{
		// Standard format: repo/resolve/main/filename.gguf
		fmt.Sprintf("https://huggingface.co/%s/resolve/main/%s", repository, fileName),
	}

	// Get models directory from config
	modelsDir := s.config.ModelsDir

	// Use just the base filename for the destination
	destFileName := filepath.Base(fileName)
	destPath := filepath.Join(modelsDir, destFileName)

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 0, // No timeout for large downloads
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return nil // Follow redirects
		},
	}

	var resp *http.Response
	var err error
	var successURL string

	// Try each URL
	for _, url := range urls {
		log.Printf("Trying URL: %s", url)
		resp, err = client.Get(url)
		if err != nil {
			log.Printf("Failed to fetch %s: %v", url, err)
			continue
		}

		if resp.StatusCode == http.StatusOK {
			successURL = url
			break
		}

		resp.Body.Close()
		log.Printf("URL returned status %d: %s", resp.StatusCode, url)
	}

	if resp == nil || resp.StatusCode != http.StatusOK {
		log.Printf("All download attempts failed for %s/%s", repository, fileName)
		s.downloadMutex.Lock()
		s.downloadProgress[fileName].Status = "failed"
		s.downloadProgress[fileName].Error = "All download URLs failed"
		s.downloadMutex.Unlock()
		return
	}
	defer resp.Body.Close()

	log.Printf("Successfully connected to: %s", successURL)

	// Update progress with total size
	total := resp.ContentLength
	s.downloadMutex.Lock()
	s.downloadProgress[fileName].BytesTotal = total
	s.downloadMutex.Unlock()

	// Create destination file
	out, err := os.Create(destPath)
	if err != nil {
		log.Printf("Failed to create file: %v", err)
		s.downloadMutex.Lock()
		s.downloadProgress[fileName].Status = "failed"
		s.downloadProgress[fileName].Error = err.Error()
		s.downloadMutex.Unlock()
		return
	}
	defer out.Close()

	// Copy with progress logging
	var downloaded int64
	buffer := make([]byte, 32*1024) // 32KB buffer
	lastLog := time.Now()

	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			_, writeErr := out.Write(buffer[:n])
			if writeErr != nil {
				log.Printf("Write error: %v", writeErr)
				s.downloadMutex.Lock()
				s.downloadProgress[fileName].Status = "failed"
				s.downloadProgress[fileName].Error = writeErr.Error()
				s.downloadMutex.Unlock()
				return
			}
			downloaded += int64(n)

			// Update progress
			s.downloadMutex.Lock()
			s.downloadProgress[fileName].BytesDone = downloaded
			if total > 0 {
				s.downloadProgress[fileName].Percent = float64(downloaded) / float64(total) * 100
			}
			s.downloadMutex.Unlock()

			// Log progress every 100MB or every 5 seconds
			if downloaded%(100*1024*1024) == 0 || time.Since(lastLog) > 5*time.Second {
				percent := float64(downloaded) / float64(total) * 100
				log.Printf("Download progress: %.1f%% (%d/%d bytes)", percent, downloaded, total)
				lastLog = time.Now()
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("Read error: %v", err)
			s.downloadMutex.Lock()
			s.downloadProgress[fileName].Status = "failed"
			s.downloadProgress[fileName].Error = err.Error()
			s.downloadMutex.Unlock()
			return
		}
	}

	log.Printf("Download completed: %s (%d bytes)", fileName, downloaded)

	// Mark as complete
	s.downloadMutex.Lock()
	s.downloadProgress[fileName].Status = "complete"
	s.downloadProgress[fileName].Percent = 100
	s.downloadMutex.Unlock()

	// Rescan models to pick up the new file
	if err := s.registry.ScanModels(); err != nil {
		log.Printf("Failed to rescan models: %v", err)
	}
}

// handleDownloadProgress returns download progress for all active downloads
func (s *Server) handleDownloadProgress(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	s.downloadMutex.RLock()
	progress := make(map[string]*DownloadProgress)
	for k, v := range s.downloadProgress {
		progress[k] = v
	}
	s.downloadMutex.RUnlock()

	if err := json.NewEncoder(w).Encode(progress); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}

// handleModelCatalog returns the curated model catalog
func (s *Server) handleModelCatalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Get the default catalog
	catalog := models.DefaultCatalog()

	// Transform catalog entries to a simpler format for the UI
	type SimpleCatalogEntry struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Category    string `json:"category"`
		Size        string `json:"size"`
		Repo        string `json:"repo"`
		File        string `json:"file"`
		Quant       string `json:"quant"`
	}

	simpleModels := []SimpleCatalogEntry{}
	for _, entry := range catalog.Models {
		if len(entry.Variants) > 0 {
			// Use the first variant (usually recommended quantization)
			variant := entry.Variants[0]
			var repo, file string

			// Extract repo and file from sources
			if len(variant.Sources) > 0 {
				for _, source := range variant.Sources {
					if source.Type == "huggingface" {
						// Parse HuggingFace URL
						// Format: huggingface.co/owner/repo/resolve/main/file.gguf
						parts := strings.Split(source.URL, "/")
						if len(parts) >= 7 {
							repo = parts[3] + "/" + parts[4]
							file = parts[len(parts)-1]
							break
						}
					}
				}
			}

			simpleModels = append(simpleModels, SimpleCatalogEntry{
				Name:        entry.Name,
				Description: entry.Description,
				Category:    strings.Join(entry.Tags, ", "),
				Size:        entry.Parameters,
				Repo:        repo,
				File:        file,
				Quant:       variant.Quantization,
			})
		}
	}

	response := map[string]interface{}{
		"total":  len(simpleModels),
		"models": simpleModels,
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

// handleUSBScan scans a USB drive for GGUF models
func (s *Server) handleUSBScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		USBPath string `json:"usb_path"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.USBPath == "" {
		writeError(w, "usb_path is required", http.StatusBadRequest)
		return
	}

	importer := models.NewUSBImporter(s.config.ModelsDir, s.registry)
	modelFiles, err := importer.ScanUSBDrive(req.USBPath)
	if err != nil {
		writeError(w, fmt.Sprintf("Failed to scan USB drive: %v", err), http.StatusInternalServerError)
		return
	}

	// Extract model info
	type ModelInfo struct {
		FilePath     string `json:"file_path"`
		FileName     string `json:"file_name"`
		Size         int64  `json:"size"`
		ModelID      string `json:"model_id"`
		Quantization string `json:"quantization"`
	}

	var models []ModelInfo
	for _, path := range modelFiles {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}

		fileName := filepath.Base(path)
		modelID, quant := importer.GetModelInfo(fileName)

		models = append(models, ModelInfo{
			FilePath:     path,
			FileName:     fileName,
			Size:         info.Size(),
			ModelID:      modelID,
			Quantization: quant,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"usb_path": req.USBPath,
		"models":   models,
		"count":    len(models),
	})
}

// handleUSBImport imports models from USB
func (s *Server) handleUSBImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		FilePaths []string `json:"file_paths"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.FilePaths) == 0 {
		writeError(w, "file_paths is required", http.StatusBadRequest)
		return
	}

	importer := models.NewUSBImporter(s.config.ModelsDir, s.registry)

	type ImportResult struct {
		FileName string `json:"file_name"`
		Success  bool   `json:"success"`
		Error    string `json:"error,omitempty"`
	}

	results := []ImportResult{}

	for _, path := range req.FilePaths {
		fileName := filepath.Base(path)
		err := importer.ImportModel(path, nil)

		if err != nil {
			results = append(results, ImportResult{
				FileName: fileName,
				Success:  false,
				Error:    err.Error(),
			})
		} else {
			results = append(results, ImportResult{
				FileName: fileName,
				Success:  true,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"results": results,
	})
}

// handleUSBExport exports models to USB
func (s *Server) handleUSBExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ModelNames []string `json:"model_names"`
		USBPath    string   `json:"usb_path"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.ModelNames) == 0 {
		writeError(w, "model_names is required", http.StatusBadRequest)
		return
	}

	if req.USBPath == "" {
		writeError(w, "usb_path is required", http.StatusBadRequest)
		return
	}

	// Verify USB path exists
	if _, err := os.Stat(req.USBPath); os.IsNotExist(err) {
		writeError(w, fmt.Sprintf("USB path does not exist: %s", req.USBPath), http.StatusBadRequest)
		return
	}

	// Start export in background
	go s.exportModelsAsync(req.ModelNames, req.USBPath)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "started",
		"count":  len(req.ModelNames),
	})
}

func (s *Server) exportModelsAsync(modelIDs []string, usbPath string) {
	for _, modelID := range modelIDs {
		// Get model metadata from registry to find actual filename
		metadata, err := s.registry.GetModel(modelID)
		if err != nil {
			s.exportMutex.Lock()
			s.exportProgress[modelID] = &ExportProgress{
				FileName: modelID,
				Status:   "failed",
				Error:    "Model not found in registry",
			}
			s.exportMutex.Unlock()
			continue
		}

		sourcePath := metadata.Path

		// Check if model file exists
		info, err := os.Stat(sourcePath)
		if err != nil {
			s.exportMutex.Lock()
			s.exportProgress[modelID] = &ExportProgress{
				FileName: modelID,
				Status:   "failed",
				Error:    "Model file not found",
			}
			s.exportMutex.Unlock()
			continue
		}

		fileName := filepath.Base(sourcePath)

		// Initialize progress
		s.exportMutex.Lock()
		s.exportProgress[modelID] = &ExportProgress{
			FileName:   fileName,
			BytesTotal: info.Size(),
			BytesDone:  0,
			Percent:    0,
			Status:     "exporting",
		}
		s.exportMutex.Unlock()

		// Copy to USB with original filename
		destPath := filepath.Join(usbPath, fileName)

		sourceFile, err := os.Open(sourcePath)
		if err != nil {
			s.exportMutex.Lock()
			s.exportProgress[modelID].Status = "failed"
			s.exportProgress[modelID].Error = fmt.Sprintf("Cannot open source: %v", err)
			s.exportMutex.Unlock()
			continue
		}

		destFile, err := os.Create(destPath)
		if err != nil {
			sourceFile.Close()
			s.exportMutex.Lock()
			s.exportProgress[modelID].Status = "failed"
			s.exportProgress[modelID].Error = fmt.Sprintf("Cannot create destination: %v", err)
			s.exportMutex.Unlock()
			continue
		}

		// Copy with progress tracking
		buffer := make([]byte, 1024*1024) // 1MB buffer
		var bytesCopied int64
		lastUpdate := time.Now()

		for {
			n, err := sourceFile.Read(buffer)
			if n > 0 {
				if _, writeErr := destFile.Write(buffer[:n]); writeErr != nil {
					sourceFile.Close()
					destFile.Close()
					os.Remove(destPath)
					s.exportMutex.Lock()
					s.exportProgress[modelID].Status = "failed"
					s.exportProgress[modelID].Error = fmt.Sprintf("Write error: %v", writeErr)
					s.exportMutex.Unlock()
					break
				}

				bytesCopied += int64(n)

				// Update progress every 5 seconds or 100MB
				if time.Since(lastUpdate) > 5*time.Second || bytesCopied-s.exportProgress[modelID].BytesDone > 100*1024*1024 {
					s.exportMutex.Lock()
					s.exportProgress[modelID].BytesDone = bytesCopied
					s.exportProgress[modelID].Percent = float64(bytesCopied) / float64(info.Size()) * 100
					s.exportMutex.Unlock()
					lastUpdate = time.Now()
				}
			}

			if err == io.EOF {
				break
			}

			if err != nil {
				sourceFile.Close()
				destFile.Close()
				os.Remove(destPath)
				s.exportMutex.Lock()
				s.exportProgress[modelID].Status = "failed"
				s.exportProgress[modelID].Error = fmt.Sprintf("Read error: %v", err)
				s.exportMutex.Unlock()
				break
			}
		}

		sourceFile.Close()
		destFile.Close()

		// Final update
		if s.exportProgress[modelID].Status != "failed" {
			s.exportMutex.Lock()
			s.exportProgress[modelID].BytesDone = bytesCopied
			s.exportProgress[modelID].Percent = 100
			s.exportProgress[modelID].Status = "complete"
			s.exportMutex.Unlock()
		}
	}
}

// handleExportProgress returns export progress for all active exports
func (s *Server) handleExportProgress(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	s.exportMutex.RLock()
	progress := make(map[string]*ExportProgress)
	for k, v := range s.exportProgress {
		progress[k] = v
	}
	s.exportMutex.RUnlock()

	if err := json.NewEncoder(w).Encode(progress); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}

// handleTemplates returns all available prompt templates
func (s *Server) handleTemplates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	templateList := templates.ListTemplates()
	result := make([]map[string]interface{}, 0, len(templateList))

	for _, name := range templateList {
		tpl, err := templates.GetTemplate(name)
		if err != nil {
			continue
		}

		result = append(result, map[string]interface{}{
			"name":        tpl.Name,
			"description": tpl.Description,
			"system":      tpl.System,
			"variables":   tpl.Variables,
			"examples":    tpl.Examples,
		})
	}

	response := map[string]interface{}{
		"total":     len(result),
		"templates": result,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}

// handleTemplateDetails returns details for a specific template
func (s *Server) handleTemplateDetails(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Extract template name from URL path
	path := strings.TrimPrefix(r.URL.Path, "/v1/templates/")
	templateName := strings.TrimSpace(path)

	if templateName == "" {
		writeError(w, "Template name required", http.StatusBadRequest)
		return
	}

	tpl, err := templates.GetTemplate(templateName)
	if err != nil {
		writeError(w, fmt.Sprintf("Template not found: %s", templateName), http.StatusNotFound)
		return
	}

	// If POST, apply variables
	if r.Method == http.MethodPost {
		var request struct {
			Variables map[string]string `json:"variables"`
		}

		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeError(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		prompt, err := tpl.Apply(request.Variables)
		if err != nil {
			writeError(w, fmt.Sprintf("Failed to apply template: %v", err), http.StatusBadRequest)
			return
		}

		response := map[string]interface{}{
			"template": tpl.Name,
			"prompt":   prompt,
			"system":   tpl.System,
		}

		json.NewEncoder(w).Encode(response)
		return
	}

	// GET request - return template details
	response := map[string]interface{}{
		"name":        tpl.Name,
		"description": tpl.Description,
		"system":      tpl.System,
		"template":    tpl.Template,
		"variables":   tpl.Variables,
		"examples":    tpl.Examples,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}
