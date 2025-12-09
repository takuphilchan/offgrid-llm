package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/cache"
	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/metrics"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/internal/platform"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"github.com/takuphilchan/offgrid-llm/internal/resource"
	"github.com/takuphilchan/offgrid-llm/internal/stats"
	"github.com/takuphilchan/offgrid-llm/internal/templates"
	"github.com/takuphilchan/offgrid-llm/internal/users"
	"github.com/takuphilchan/offgrid-llm/internal/websocket"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Server represents the HTTP server
type Server struct {
	httpServer           *http.Server
	config               *config.Config
	registry             *models.Registry
	engine               inference.Engine
	embeddingEngine      *inference.EmbeddingEngine
	ragEngine            *rag.Engine
	monitor              *resource.Monitor
	statsTracker         *stats.Tracker
	cache                *cache.ResponseCache
	startTime            time.Time
	downloadProgress     map[string]*DownloadProgress
	downloadMutex        sync.RWMutex
	exportProgress       map[string]*ExportProgress
	exportMutex          sync.RWMutex
	modelCache           *inference.ModelCache
	currentModelID       string
	currentPort          int
	modelMutex           sync.Mutex
	inferenceMutex       sync.Mutex // Ensures only one inference runs at a time
	rateLimiter          *RateLimiter
	inferenceRateLimiter *InferenceRateLimiter
	sessionHandlers      *SessionHandlers
	authMiddleware       *users.Middleware
	// New feature managers
	userStore      *users.UserStore
	quotaManager   *users.QuotaManager
	kbManager      *users.KnowledgeBaseManager
	loraManager    *inference.LoRAManager
	agentManager   *agents.Manager
	toolRegistry   *agents.ToolRegistry
	offgridMetrics *metrics.OffGridMetrics
	wsHub          *websocket.Hub
	// Runtime tracking
	requestCount       int64
	wsConnections      int64
	tokensGenerated    int64
	errorsTotal        int64
	totalLatencyMicros int64 // For calculating average latency
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

	// Initialize RAG engine
	ragEngine := rag.NewEngine(embeddingEngine, cfg.ModelsDir)

	// Initialize session handlers
	sessionsDir := filepath.Join(cfg.ModelsDir, "..", "sessions")
	sessionHandlers := NewSessionHandlers(sessionsDir)

	// Initialize new feature components
	dataDir := filepath.Join(cfg.ModelsDir, "..", "data")
	userStore := users.NewUserStore(dataDir)
	quotaManager := users.NewQuotaManager(dataDir)
	kbManager := users.NewKnowledgeBaseManager(dataDir)
	loraManager := inference.NewLoRAManager(dataDir, engine)
	agentManager := agents.NewManager(nil, nil, nil) // Tools and LLM caller can be set later
	toolRegistry := agents.NewToolRegistry()         // Unified tool registry
	// Load user-defined tools from config if exists
	toolsConfigPath := filepath.Join(dataDir, "tools.json")
	if err := toolRegistry.LoadUserTools(toolsConfigPath); err != nil {
		log.Printf("Warning: Failed to load user tools: %v", err)
	}
	offgridMetrics := metrics.NewOffGridMetrics() // Uses DefaultRegistry
	wsHub := websocket.NewHub()
	ctx := context.Background()
	go wsHub.Run(ctx)

	// Initialize auth middleware
	authMiddleware := users.NewMiddleware(userStore)
	authMiddleware.SetRequireAuth(cfg.RequireAuth)
	authMiddleware.SetGuestEnabled(cfg.GuestAccess)
	// Add bypass paths for public endpoints
	authMiddleware.AddBypassPath("/health")
	authMiddleware.AddBypassPath("/ready")
	authMiddleware.AddBypassPath("/livez")
	authMiddleware.AddBypassPath("/readyz")
	authMiddleware.AddBypassPath("/metrics")

	return &Server{
		config:               cfg,
		registry:             registry,
		engine:               engine,
		embeddingEngine:      embeddingEngine,
		ragEngine:            ragEngine,
		monitor:              monitor,
		statsTracker:         statsTracker,
		cache:                responseCache,
		startTime:            time.Now(),
		downloadProgress:     make(map[string]*DownloadProgress),
		exportProgress:       make(map[string]*ExportProgress),
		modelCache:           inference.NewModelCache(3, cfg.NumGPULayers), // Cache up to 3 models, use configured GPU layers
		rateLimiter:          rateLimiter,
		inferenceRateLimiter: inferenceRateLimiter,
		sessionHandlers:      sessionHandlers,
		authMiddleware:       authMiddleware,
		userStore:            userStore,
		quotaManager:         quotaManager,
		kbManager:            kbManager,
		loraManager:          loraManager,
		agentManager:         agentManager,
		toolRegistry:         toolRegistry,
		offgridMetrics:       offgridMetrics,
		wsHub:                wsHub,
	}
}

// startLlamaServer is deprecated - models are now loaded on-demand via cache
func (s *Server) startLlamaServer() error {
	// Kill any pre-existing llama-server instances to avoid port conflicts
	// The model cache will start fresh instances on demand
	exec.Command("pkill", "-9", "llama-server").Run()
	log.Println("Cleared any pre-existing llama-server instances - will load models on first request")
	return nil
}

// switchModel uses the model cache to load or switch to a model
// Returns the port of the llama-server instance running this model
func (s *Server) switchModel(modelID string) error {
	s.modelMutex.Lock()
	defer s.modelMutex.Unlock()

	// Always check model status via cache to ensure process is still alive
	// The cache will handle liveness checks and reloading if needed
	log.Printf("Switching to model: %s", modelID)

	// Get model metadata
	metadata, err := s.registry.GetModel(modelID)
	if err != nil {
		return fmt.Errorf("model not found: %w", err)
	}

	// Load or get cached model instance
	instance, err := s.modelCache.GetOrLoad(modelID, metadata.Path, metadata.ProjectorPath)
	if err != nil {
		return fmt.Errorf("failed to load model: %w", err)
	}

	// Update current model tracking
	s.currentModelID = modelID
	s.currentPort = instance.Port

	// Update engine to point to the correct port
	type PortSetter interface {
		SetPort(int)
	}

	if ps, ok := s.engine.(PortSetter); ok {
		ps.SetPort(instance.Port)
	} else {
		// Fallback for direct LlamaHTTPEngine usage (if not wrapped)
		if llamaEngine, ok := s.engine.(*inference.LlamaHTTPEngine); ok {
			llamaEngine.SetPort(instance.Port)
		}
	}

	log.Printf("Now using model %s on port %d", modelID, instance.Port)
	return nil
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
	mux.HandleFunc("/v1/terminal/exec/stream", s.handleTerminalExecStream)

	// Templates endpoints
	mux.HandleFunc("/v1/templates", s.handleTemplates)
	mux.HandleFunc("/v1/templates/", s.handleTemplateDetails)

	// USB import/export
	mux.HandleFunc("/v1/usb/scan", s.handleUSBScan)
	mux.HandleFunc("/v1/usb/import", s.handleUSBImport)
	mux.HandleFunc("/v1/usb/export", s.handleUSBExport)
	mux.HandleFunc("/v1/usb/export/progress", s.handleExportProgress)
	mux.HandleFunc("/v1/filesystem/browse", s.handleFilesystemBrowse)
	mux.HandleFunc("/v1/filesystem/common-paths", s.handleCommonPaths)

	// RAG (Retrieval Augmented Generation) endpoints
	mux.HandleFunc("/v1/rag/status", s.handleRAGStatus)
	mux.HandleFunc("/v1/rag/enable", s.handleRAGEnable)
	mux.HandleFunc("/v1/rag/disable", s.handleRAGDisable)
	mux.HandleFunc("/v1/documents", s.handleDocumentsList)
	mux.HandleFunc("/v1/documents/ingest", s.handleDocumentIngest)
	mux.HandleFunc("/v1/documents/delete", s.handleDocumentDelete)
	mux.HandleFunc("/v1/documents/search", s.handleDocumentSearch)

	// Statistics endpoint
	mux.HandleFunc("/stats", s.handleStats)
	mux.HandleFunc("/v1/stats", s.handleStatsV1)
	mux.HandleFunc("/v1/system/info", s.handleSystemInfo)

	// Sessions endpoints
	mux.HandleFunc("/v1/sessions", s.sessionHandlers.HandleSessions)
	mux.HandleFunc("/v1/sessions/", s.sessionHandlers.HandleSessions)

	// Model cache statistics
	mux.HandleFunc("/v1/cache/stats", s.handleCacheStats)

	// Cache management endpoints
	mux.HandleFunc("/cache/stats", s.handleCacheStats)
	mux.HandleFunc("/cache/clear", s.handleCacheClear)

	// Prometheus metrics endpoint
	mux.HandleFunc("/metrics", s.handleMetrics)
	mux.HandleFunc("/v1/system/stats", s.handleSystemStats)
	mux.HandleFunc("/v1/system/config", s.handleSystemConfig) // UI feature flags

	// User management endpoints
	mux.HandleFunc("/v1/users/me", s.handleCurrentUser)
	mux.HandleFunc("/v1/users", s.handleUsers)
	mux.HandleFunc("/v1/users/", s.handleUsers)
	mux.HandleFunc("/v1/auth/login", s.handleLogin)
	mux.HandleFunc("/v1/auth/logout", s.handleLogout)
	mux.HandleFunc("/v1/quota", s.handleQuota)

	// LoRA adapter endpoints
	mux.HandleFunc("/v1/lora", s.handleLoRAList)
	mux.HandleFunc("/v1/lora/", s.handleLoRA)

	// Agent endpoints
	mux.HandleFunc("/v1/agents/run", s.handleAgentRun)
	mux.HandleFunc("/v1/agents/tasks", s.handleAgentTasks)
	mux.HandleFunc("/v1/agents/workflows", s.handleAgentWorkflows)
	mux.HandleFunc("/v1/agents/tools", s.handleAgentTools)
	mux.HandleFunc("/v1/agents/mcp", s.handleAgentMCP)
	mux.HandleFunc("/v1/agents/mcp/test", s.handleAgentMCPTest)

	// Audio endpoints (OpenAI-compatible TTS/ASR)
	mux.HandleFunc("/v1/audio/transcriptions", s.handleAudioTranscriptions)
	mux.HandleFunc("/v1/audio/speech", s.handleAudioSpeech)
	mux.HandleFunc("/v1/audio/voices", s.handleAudioVoices)
	mux.HandleFunc("/v1/audio/whisper-models", s.handleAudioWhisperModels)
	mux.HandleFunc("/v1/audio/models", s.handleAudioModels)
	mux.HandleFunc("/v1/audio/status", s.handleAudioStatus)
	mux.HandleFunc("/v1/audio/download", s.handleAudioDownload)
	mux.HandleFunc("/v1/audio/setup/whisper", s.handleAudioSetupWhisper)
	mux.HandleFunc("/v1/audio/setup/piper", s.handleAudioSetupPiper)

	// WebSocket endpoint
	mux.HandleFunc("/v1/ws", s.handleWebSocket)

	// Simplified UI endpoints (no /v1 prefix for easier frontend access)
	mux.HandleFunc("/models", s.handleListModels)
	mux.HandleFunc("/models/refresh", s.handleRefreshModels)
	mux.HandleFunc("/catalog", s.handleModelCatalog)

	// Web UI - serve HTML/CSS/JS
	uiPath := "/var/lib/offgrid/web/ui"
	// Fallback to local development path if installed path doesn't exist
	if _, err := os.Stat(uiPath); os.IsNotExist(err) {
		uiPath = "web/ui"
	}

	// Force local path if running from source (check for go.mod)
	if _, err := os.Stat("go.mod"); err == nil {
		if _, err := os.Stat("web/ui"); err == nil {
			uiPath = "web/ui"
			log.Println("Running from source, using local web/ui directory")
		}
	}

	// Serve static files
	fs := http.FileServer(http.Dir(uiPath))
	mux.Handle("/ui/", http.StripPrefix("/ui/", fs))

	// Root endpoint
	mux.HandleFunc("/", s.handleRoot)

	// Build handler chain: logging -> auth -> routes
	var handler http.Handler = mux
	handler = s.authMiddleware.Wrap(handler) // Auth middleware
	handler = s.loggingMiddleware(handler)   // Logging middleware (outermost)

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.config.ServerPort),
		Handler:      handler,
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

	// Auto-restore RAG state if there's persisted data
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if err := s.ragEngine.AutoRestore(ctx); err != nil {
			log.Printf("Warning: Failed to auto-restore RAG: %v", err)
		}
	}()

	// Start background metrics collection
	go s.collectSystemMetrics()

	// Create listener first to ensure port is available
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", s.config.ServerPort))
	if err != nil {
		return fmt.Errorf("failed to bind to port %d: %w", s.config.ServerPort, err)
	}

	// Clean startup message with colors
	const (
		colorReset   = "\033[0m"
		colorCyan    = "\033[36m"
		colorGreen   = "\033[32m"
		brandPrimary = "\033[38;5;45m"
	)

	fmt.Println()
	fmt.Printf("%sOffGrid LLM Server%s\n", colorCyan, colorReset)
	fmt.Println()
	fmt.Printf("Server:  http://localhost:%d\n", s.config.ServerPort)
	fmt.Printf("Web UI:  http://localhost:%d/ui/\n", s.config.ServerPort)
	fmt.Printf("Health:  http://localhost:%d/health\n", s.config.ServerPort)
	fmt.Println()
	fmt.Printf("OpenAI-Compatible API Endpoints:\n")
	fmt.Printf("  POST /v1/chat/completions\n")
	fmt.Printf("  POST /v1/completions\n")
	fmt.Printf("  POST /v1/embeddings\n")
	fmt.Printf("  GET  /v1/models\n")
	fmt.Println()
	fmt.Printf("%s[OK]%s Server ready on port %d\n", colorGreen, colorReset, s.config.ServerPort)
	fmt.Println()

	if err := s.httpServer.Serve(listener); err != http.ErrServerClosed {
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

	// Stop all cached llama-server instances
	log.Println("Stopping all llama-server instances...")
	s.modelCache.UnloadAll()
	log.Println("All llama-server instances stopped")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		log.Printf("Shutdown error: %v", err)
	}

	log.Println("Server stopped")
}

// collectSystemMetrics periodically updates system metrics
func (s *Server) collectSystemMetrics() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)

			// Update memory metrics
			s.offgridMetrics.MemoryUsage.Set(float64(memStats.Alloc))

			// Update CPU usage (approximate from goroutine count)
			numGoroutines := runtime.NumGoroutine()
			s.offgridMetrics.CPUUsage.Set(float64(numGoroutines) / 100.0 * 10)

			// Update model loaded status
			if s.engine != nil && s.engine.IsLoaded() {
				s.offgridMetrics.ModelLoaded.Set(1)
			} else {
				s.offgridMetrics.ModelLoaded.Set(0)
			}

			// Update RAG metrics
			if s.ragEngine != nil {
				docs := s.ragEngine.ListDocuments()
				s.offgridMetrics.RAGDocumentsTotal.Set(float64(len(docs)))
			}

			// Update session count
			if s.sessionHandlers != nil {
				s.offgridMetrics.ActiveSessions.Set(float64(s.sessionHandlers.GetActiveSessionCount()))
			}

			// Update WebSocket connections
			s.offgridMetrics.WebSocketConnections.Set(float64(s.wsConnections))

			// Update user counts
			users := s.userStore.ListUsers()
			s.offgridMetrics.TotalUsers.Set(float64(len(users)))
			// Active users approximation - count users who logged in recently
			activeUsers := 0
			for _, u := range users {
				if u.LastLoginAt != nil && time.Since(*u.LastLoginAt) < 24*time.Hour {
					activeUsers++
				}
			}
			s.offgridMetrics.ActiveUsers.Set(float64(activeUsers))
		}
	}
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

		// Track in-flight requests
		s.offgridMetrics.RequestsInFlight.Inc()
		defer s.offgridMetrics.RequestsInFlight.Dec()

		// Wrap response writer to capture status code
		wrapped := &statusResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		start := time.Now()
		next.ServeHTTP(wrapped, r)
		duration := time.Since(start)

		// Record metrics
		s.offgridMetrics.RequestsTotal.Inc(r.Method, r.URL.Path, fmt.Sprintf("%d", wrapped.statusCode))
		s.offgridMetrics.RequestDuration.Observe(duration.Seconds(), r.Method, r.URL.Path)

		// Track errors
		if wrapped.statusCode >= 400 {
			s.offgridMetrics.ErrorsTotal.Inc(r.Method, r.URL.Path, fmt.Sprintf("%d", wrapped.statusCode))
			atomic.AddInt64(&s.errorsTotal, 1)
		}

		// Increment request count and latency tracking (atomic)
		atomic.AddInt64(&s.requestCount, 1)
		atomic.AddInt64(&s.totalLatencyMicros, duration.Microseconds())

		log.Printf("%s %s %d · %.2fms", r.Method, r.URL.Path, wrapped.statusCode, float64(duration.Microseconds())/1000)
	})
}

// statusResponseWriter wraps http.ResponseWriter to capture status code
type statusResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *statusResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

// Flush implements http.Flusher to support streaming
func (w *statusResponseWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
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
	fmt.Fprintf(w, `{"name":"OffGrid LLM","version":"0.2.4","status":"running"}`)
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

// handleSystemInfo returns detailed system information
func (s *Server) handleSystemInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	sysInfo := platform.GetSystemInfo()

	json.NewEncoder(w).Encode(sysInfo)
}

// handleStatsV1 returns comprehensive server statistics (v1 API)
func (s *Server) handleStatsV1(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Get all model stats
	allStats := s.statsTracker.GetAllStats()

	// Get system info
	sysInfo := platform.GetSystemInfo()

	// Get resource usage
	resourceStats := s.monitor.GetStats()

	// Get model cache stats
	modelCacheStats := s.modelCache.GetStats()

	// Get response cache stats
	responseCacheStats := s.cache.Stats()

	// Get RAG stats
	ragStats := map[string]interface{}{
		"enabled": s.ragEngine.IsEnabled(),
	}
	if s.ragEngine.IsEnabled() {
		ragStats["documents"] = len(s.ragEngine.ListDocuments())
	}

	// Calculate uptime
	uptime := time.Since(s.startTime)

	// Read version from VERSION file if available
	version := "unknown"
	if versionData, err := os.ReadFile("VERSION"); err == nil {
		version = strings.TrimSpace(string(versionData))
	}

	response := map[string]interface{}{
		"server": map[string]interface{}{
			"uptime":         uptime.String(),
			"uptime_seconds": int64(uptime.Seconds()),
			"start_time":     s.startTime.Format(time.RFC3339),
			"version":        version,
			"current_model":  s.currentModelID,
		},
		"inference": map[string]interface{}{
			"models":    allStats,
			"aggregate": s.getAggregateStats(),
		},
		"system": map[string]interface{}{
			"os":              sysInfo.OS,
			"arch":            sysInfo.Architecture,
			"cpu_cores":       sysInfo.CPUCores,
			"total_memory_gb": sysInfo.TotalMemory / (1024 * 1024 * 1024),
		},
		"resources": resourceStats,
		"cache": map[string]interface{}{
			"models":   modelCacheStats,
			"response": responseCacheStats,
		},
		"rag": ragStats,
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

func (s *Server) handleRefreshModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Rescan models directory
	if err := s.registry.ScanModels(); err != nil {
		log.Printf("Error scanning models: %v", err)
		http.Error(w, "Failed to refresh models", http.StatusInternalServerError)
		return
	}

	// Return updated model list
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
	// Use Lock to queue requests instead of rejecting them
	s.inferenceMutex.Lock()
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

	// Apply RAG enhancement if enabled
	log.Printf("🔍 RAG Check: UseKnowledgeBase=%v, RAGEnabled=%v", req.UseKnowledgeBase, s.ragEngine.IsEnabled())
	if req.UseKnowledgeBase != nil && *req.UseKnowledgeBase {
		if !s.ragEngine.IsEnabled() {
			log.Printf("⚠️ Knowledge Base requested but RAG is not enabled")
		} else {
			// Find the last user message
			for i := len(req.Messages) - 1; i >= 0; i-- {
				if req.Messages[i].Role == "user" {
					userContent := req.Messages[i].StringContent()
					log.Printf("🔍 RAG searching for: %s", userContent)
					enhancedContent, ragCtx, err := s.ragEngine.EnhancePrompt(r.Context(), userContent)
					if err != nil {
						log.Printf("❌ RAG enhancement failed: %v", err)
					} else if ragCtx != nil && len(ragCtx.Results) > 0 {
						// Replace the user message with enhanced version
						req.Messages[i].Content = enhancedContent
						log.Printf("📚 RAG injected %d chunks for query (context length: %d chars)", len(ragCtx.Results), len(ragCtx.Context))
					} else {
						log.Printf("⚠️ RAG found no relevant results for query")
					}
					break
				}
			}
		}
	}

	// Handle streaming vs non-streaming
	if req.Stream {
		s.handleChatCompletionsStream(w, r, &req)
		return
	}

	// Perform inference
	ctx := r.Context()
	startTime := time.Now()
	response, err := s.engine.ChatCompletion(ctx, &req)
	duration := time.Since(startTime)

	if err != nil {
		if handleEngineError(w, err) {
			return
		}
		writeError(w, fmt.Sprintf("Inference failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Record statistics
	totalTokens := int64(response.Usage.TotalTokens)
	s.statsTracker.RecordInference(req.Model, totalTokens, duration.Milliseconds())

	// Track tokens generated for metrics
	atomic.AddInt64(&s.tokensGenerated, int64(response.Usage.CompletionTokens))
	s.offgridMetrics.TokensOutputTotal.Add(float64(response.Usage.CompletionTokens))
	s.offgridMetrics.TokensInputTotal.Add(float64(response.Usage.PromptTokens))

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

	ctx := r.Context()
	chunkID := fmt.Sprintf("chatcmpl-%d", time.Now().Unix())
	tokenIndex := 0

	// Define callback to reuse
	callback := func(token string) error {
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
	}

	// Send tokens as they arrive
	err := s.engine.ChatCompletionStream(ctx, req, callback)

	// Retry logic: If we haven't sent any tokens yet and encountered a network error,
	// it likely means the llama-server process crashed or was dead.
	// We should try to reload the model and retry the request once.
	if err != nil && tokenIndex == 0 {
		errMsg := err.Error()
		if strings.Contains(errMsg, "EOF") || strings.Contains(errMsg, "connection refused") || strings.Contains(errMsg, "connection reset") {
			log.Printf("Inference failed before generation started: %v. Attempting to reload model and retry...", err)

			// Force unload to kill any zombie process
			s.modelCache.Unload(req.Model)

			// Switch model (reloads it)
			if reloadErr := s.switchModel(req.Model); reloadErr == nil {
				log.Printf("Model reloaded successfully. Retrying inference...")
				err = s.engine.ChatCompletionStream(ctx, req, callback)
			} else {
				log.Printf("Failed to reload model during retry: %v", reloadErr)
			}
		}
	}

	if err != nil {
		log.Printf("Streaming error: %v", err)
		streamType := "stream_error"
		code := ""
		message := err.Error()
		if engineErr := inference.AsEngineError(err); engineErr != nil {
			if engineErr.Code != "" {
				streamType = engineErr.Code
				code = engineErr.Code
			}
			if engineErr.Message != "" {
				message = engineErr.Message
			}
		}
		// Send error as final chunk
		errChunk := map[string]interface{}{
			"error": map[string]string{
				"message": message,
				"type":    streamType,
				"code":    code,
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

	// Track tokens generated for metrics (approximate from tokenIndex)
	if tokenIndex > 0 {
		atomic.AddInt64(&s.tokensGenerated, int64(tokenIndex))
		s.offgridMetrics.TokensOutputTotal.Add(float64(tokenIndex))
	}
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
	s.inferenceMutex.Lock()
	defer s.inferenceMutex.Unlock()

	// Switch to requested model if different from current
	if err := s.switchModel(req.Model); err != nil {
		writeError(w, fmt.Sprintf("Failed to switch model: %v", err), http.StatusInternalServerError)
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
	ctx := r.Context()
	response, err := s.engine.Completion(ctx, &req)
	if err != nil {
		if handleEngineError(w, err) {
			return
		}
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
	ctx := r.Context()
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
	offgridPath, err := os.Executable()
	if err != nil {
		// Fallback to PATH
		offgridPath, err = exec.LookPath("offgrid")
		if err != nil {
			writeError(w, "offgrid binary not found", http.StatusInternalServerError)
			return
		}
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

// handleTerminalExecStream executes offgrid commands with real-time streaming output
func (s *Server) handleTerminalExecStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Command string   `json:"command"`
		Args    []string `json:"args"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Security: Only allow offgrid commands
	if req.Command != "offgrid" {
		http.Error(w, "Only 'offgrid' commands are allowed", http.StatusForbidden)
		return
	}

	// Find offgrid binary
	offgridPath, err := os.Executable()
	if err != nil {
		// Fallback to PATH
		offgridPath, err = exec.LookPath("offgrid")
		if err != nil {
			http.Error(w, "offgrid binary not found", http.StatusInternalServerError)
			return
		}
	}

	// Set up Server-Sent Events
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Execute command
	// Use a very long timeout for downloads (1 hour), shorter for others
	timeout := 120 * time.Second
	if len(req.Args) > 0 && (req.Args[0] == "download" || req.Args[0] == "download-hf") {
		timeout = 1 * time.Hour
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, offgridPath, req.Args...)

	// Create pipes for stdout and stderr
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		sendSSE(w, flusher, "error", fmt.Sprintf("Failed to create stdout pipe: %v", err))
		return
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		sendSSE(w, flusher, "error", fmt.Sprintf("Failed to create stderr pipe: %v", err))
		return
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		sendSSE(w, flusher, "error", fmt.Sprintf("Failed to start command: %v", err))
		return
	}

	// Channel to signal completion
	done := make(chan bool)

	// Mutex and flag to prevent SSE writes after disconnect
	var sseMu sync.Mutex
	sseOpen := true

	// Safe SSE sender that checks if connection is still open
	safeSendSSE := func(eventType, data string) {
		sseMu.Lock()
		defer sseMu.Unlock()
		if sseOpen {
			sendSSE(w, flusher, eventType, data)
		}
	}

	// Stream stdout
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stdoutPipe.Read(buf)
			if n > 0 {
				safeSendSSE("output", string(buf[:n]))
			}
			if err != nil {
				break
			}
		}
	}()

	// Stream stderr
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stderrPipe.Read(buf)
			if n > 0 {
				safeSendSSE("output", string(buf[:n]))
			}
			if err != nil {
				break
			}
		}
	}()

	// Wait for command to complete
	go func() {
		defer close(done)
		err := cmd.Wait()
		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = 1
			}
		}
		safeSendSSE("exit", fmt.Sprintf("%d", exitCode))
	}()

	// Wait for completion or client disconnect
	select {
	case <-done:
		// Command finished normally
	case <-r.Context().Done():
		// Client disconnected - mark SSE as closed before killing process
		sseMu.Lock()
		sseOpen = false
		sseMu.Unlock()
		cmd.Process.Kill()
	}
}

// sendSSE sends a Server-Sent Event with the given event type and data
func sendSSE(w http.ResponseWriter, flusher http.Flusher, eventType, data string) {
	// Recover from any panics (e.g., writing to closed connection)
	defer func() {
		if r := recover(); r != nil {
			log.Printf("SSE send recovered from panic: %v", r)
		}
	}()

	// SSE format: multi-line data must have each line prefixed with "data: "
	// Split by newlines and prefix each line
	fmt.Fprintf(w, "event: %s\n", eventType)
	lines := strings.Split(data, "\n")
	for _, line := range lines {
		fmt.Fprintf(w, "data: %s\n", line)
	}
	fmt.Fprintf(w, "\n")
	if flusher != nil {
		flusher.Flush()
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
	writeErrorWithCode(w, message, statusCode, "")
}

func writeErrorWithCode(w http.ResponseWriter, message string, statusCode int, code string) {
	log.Printf("API Error: %s (Status: %d, Code: %s)", message, statusCode, code)
	w.WriteHeader(statusCode)
	response := api.ErrorResponse{
		Error: api.ErrorDetail{
			Message: message,
			Type:    "api_error",
			Code:    code,
		},
	}
	json.NewEncoder(w).Encode(response)
}

func handleEngineError(w http.ResponseWriter, err error) bool {
	engineErr := inference.AsEngineError(err)
	if engineErr == nil {
		return false
	}

	status := http.StatusBadRequest
	log.Printf("Engine error (%s): %s", engineErr.Code, engineErr.Details)
	writeErrorWithCode(w, engineErr.Message, status, engineErr.Code)
	return true
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
		Path string `json:"path"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Path == "" {
		writeError(w, "path is required", http.StatusBadRequest)
		return
	}

	// Verify path exists
	if _, err := os.Stat(req.Path); os.IsNotExist(err) {
		writeError(w, fmt.Sprintf("Path does not exist: %s", req.Path), http.StatusBadRequest)
		return
	}

	importer := models.NewUSBImporter(s.config.ModelsDir, s.registry)

	// Import all models from the USB path
	imported, err := importer.ImportAll(req.Path, nil)
	if err != nil {
		writeError(w, fmt.Sprintf("Import failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Rescan models to update registry
	if err := s.registry.ScanModels(); err != nil {
		fmt.Printf("Warning: failed to rescan models: %v\n", err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"imported_count": imported,
		"status":         "success",
	})
}

// handleUSBExport exports models to USB
func (s *Server) handleUSBExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Path string `json:"path"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Path == "" {
		writeError(w, "path is required", http.StatusBadRequest)
		return
	}

	// Verify USB path exists
	if _, err := os.Stat(req.Path); os.IsNotExist(err) {
		writeError(w, fmt.Sprintf("Path does not exist: %s", req.Path), http.StatusBadRequest)
		return
	}

	exporter := models.NewUSBExporter(s.config.ModelsDir, s.registry)

	// Export all models
	exported, err := exporter.ExportAll(req.Path, nil)
	if err != nil {
		writeError(w, fmt.Sprintf("Export failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Calculate total size
	var totalSize int64
	models := s.registry.ListModels()
	for _, model := range models {
		meta, err := s.registry.GetModel(model.ID)
		if err == nil && meta != nil {
			totalSize += meta.Size
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"exported_count": exported,
		"total_size_gb":  float64(totalSize) / (1024 * 1024 * 1024),
		"status":         "success",
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

// handleFilesystemBrowse allows browsing the filesystem for path selection
func (s *Server) handleFilesystemBrowse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Path string `json:"path"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Default to user's home directory if no path provided
	browsePath := req.Path
	if browsePath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			browsePath = "/"
		} else {
			browsePath = homeDir
		}
	}

	// Clean the path
	browsePath = filepath.Clean(browsePath)

	// Check if path exists
	info, err := os.Stat(browsePath)
	if err != nil {
		writeError(w, fmt.Sprintf("Path does not exist: %s", browsePath), http.StatusBadRequest)
		return
	}

	// If it's a file, use its directory
	if !info.IsDir() {
		browsePath = filepath.Dir(browsePath)
	}

	// Read directory contents
	entries, err := os.ReadDir(browsePath)
	if err != nil {
		writeError(w, fmt.Sprintf("Cannot read directory: %v", err), http.StatusInternalServerError)
		return
	}

	type FileEntry struct {
		Name        string `json:"name"`
		Path        string `json:"path"`
		IsDirectory bool   `json:"is_directory"`
		Size        int64  `json:"size,omitempty"`
		ModTime     string `json:"mod_time,omitempty"`
		IsHidden    bool   `json:"is_hidden"`
	}

	var files []FileEntry
	var dirs []FileEntry

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		name := entry.Name()
		fullPath := filepath.Join(browsePath, name)
		isHidden := strings.HasPrefix(name, ".")

		fileEntry := FileEntry{
			Name:        name,
			Path:        fullPath,
			IsDirectory: entry.IsDir(),
			Size:        info.Size(),
			ModTime:     info.ModTime().Format("2006-01-02 15:04:05"),
			IsHidden:    isHidden,
		}

		if entry.IsDir() {
			dirs = append(dirs, fileEntry)
		} else {
			files = append(files, fileEntry)
		}
	}

	// Get parent directory
	parentDir := filepath.Dir(browsePath)
	if parentDir == browsePath {
		parentDir = "" // At root
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"current_path": browsePath,
		"parent_path":  parentDir,
		"directories":  dirs,
		"files":        files,
	})
}

// handleCommonPaths returns common mount points and paths for the current OS
func (s *Server) handleCommonPaths(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	type PathEntry struct {
		Path        string `json:"path"`
		Label       string `json:"label"`
		Description string `json:"description"`
		Exists      bool   `json:"exists"`
	}

	var commonPaths []PathEntry

	// Add home directory
	if homeDir, err := os.UserHomeDir(); err == nil {
		commonPaths = append(commonPaths, PathEntry{
			Path:        homeDir,
			Label:       "Home",
			Description: "User home directory",
			Exists:      true,
		})
	}

	// Add models directory
	modelsDir := s.config.ModelsDir
	_, err := os.Stat(modelsDir)
	commonPaths = append(commonPaths, PathEntry{
		Path:        modelsDir,
		Label:       "Models Directory",
		Description: "OffGrid models storage",
		Exists:      err == nil,
	})

	// Platform-specific common paths
	if runtime.GOOS == "linux" {
		// Linux USB mount points
		mediaPaths := []string{"/media", "/mnt", "/run/media"}
		for _, basePath := range mediaPaths {
			entries, err := os.ReadDir(basePath)
			if err == nil {
				for _, entry := range entries {
					if entry.IsDir() {
						fullPath := filepath.Join(basePath, entry.Name())
						commonPaths = append(commonPaths, PathEntry{
							Path:        fullPath,
							Label:       fmt.Sprintf("USB: %s", entry.Name()),
							Description: fmt.Sprintf("Removable storage at %s", fullPath),
							Exists:      true,
						})
					}
				}
			}
		}
		// Add common base paths even if empty
		commonPaths = append(commonPaths, PathEntry{
			Path:        "/media",
			Label:       "/media",
			Description: "Linux media mount point",
			Exists:      true,
		})
		commonPaths = append(commonPaths, PathEntry{
			Path:        "/mnt",
			Label:       "/mnt",
			Description: "Linux mount point",
			Exists:      true,
		})
	} else if runtime.GOOS == "darwin" {
		// macOS Volumes
		volumesPath := "/Volumes"
		entries, err := os.ReadDir(volumesPath)
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() && entry.Name() != "Macintosh HD" {
					fullPath := filepath.Join(volumesPath, entry.Name())
					commonPaths = append(commonPaths, PathEntry{
						Path:        fullPath,
						Label:       fmt.Sprintf("Volume: %s", entry.Name()),
						Description: fmt.Sprintf("Mounted volume at %s", fullPath),
						Exists:      true,
					})
				}
			}
		}
		commonPaths = append(commonPaths, PathEntry{
			Path:        "/Volumes",
			Label:       "/Volumes",
			Description: "macOS volumes directory",
			Exists:      true,
		})
	} else if runtime.GOOS == "windows" {
		// Windows drive letters
		for drive := 'C'; drive <= 'Z'; drive++ {
			drivePath := fmt.Sprintf("%c:\\", drive)
			if _, err := os.Stat(drivePath); err == nil {
				label := fmt.Sprintf("Drive %c:", drive)
				desc := "Local drive"
				if drive > 'C' {
					desc = "Removable/External drive"
				}
				commonPaths = append(commonPaths, PathEntry{
					Path:        drivePath,
					Label:       label,
					Description: desc,
					Exists:      true,
				})
			}
		}
	}

	// Add root
	commonPaths = append(commonPaths, PathEntry{
		Path:        "/",
		Label:       "Root",
		Description: "System root directory",
		Exists:      true,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"os":    runtime.GOOS,
		"paths": commonPaths,
	})
}

// ============================================================================
// Prometheus Metrics Handler
// ============================================================================

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	output := metrics.DefaultRegistry.Collect()
	w.Write([]byte(output))
}

// handleSystemStats returns real-time system statistics
func (s *Server) handleSystemStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Get number of goroutines as a proxy for activity
	numGoroutines := runtime.NumGoroutine()

	// Count users
	userList := s.userStore.ListUsers()
	adminCount := 0
	for _, u := range userList {
		if u.Role == "admin" {
			adminCount++
		}
	}

	// Get loaded models count (check if engine has a model loaded)
	modelsLoaded := 0
	if s.engine != nil && s.engine.IsLoaded() {
		modelsLoaded = 1
	}

	// Get RAG document count
	ragDocs := 0
	if s.ragEngine != nil {
		ragDocs = len(s.ragEngine.ListDocuments())
	}

	// Get active sessions - use session handlers
	activeSessions := 0
	if s.sessionHandlers != nil {
		activeSessions = s.sessionHandlers.GetActiveSessionCount()
	}

	// Calculate average latency
	reqCount := atomic.LoadInt64(&s.requestCount)
	totalLatency := atomic.LoadInt64(&s.totalLatencyMicros)
	avgLatencyMs := float64(0)
	if reqCount > 0 {
		avgLatencyMs = float64(totalLatency) / float64(reqCount) / 1000.0
	}

	stats := map[string]interface{}{
		"cpu_percent":           float64(numGoroutines) / 100.0 * 10, // Approximate
		"memory_bytes":          memStats.Alloc,
		"memory_total":          memStats.Sys,
		"heap_alloc":            memStats.HeapAlloc,
		"heap_sys":              memStats.HeapSys,
		"goroutines":            numGoroutines,
		"gc_cycles":             memStats.NumGC,
		"models_loaded":         modelsLoaded,
		"rag_documents":         ragDocs,
		"active_sessions":       activeSessions,
		"total_users":           len(userList),
		"admin_users":           adminCount,
		"uptime_seconds":        time.Since(s.startTime).Seconds(),
		"requests_total":        atomic.LoadInt64(&s.requestCount),
		"websocket_connections": atomic.LoadInt64(&s.wsConnections),
		"tokens_generated":      atomic.LoadInt64(&s.tokensGenerated),
		"errors_total":          atomic.LoadInt64(&s.errorsTotal),
		"avg_latency_ms":        avgLatencyMs,
	}

	json.NewEncoder(w).Encode(stats)
}

// handleSystemConfig returns feature flags for the UI
func (s *Server) handleSystemConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(map[string]interface{}{
		"multi_user_mode": s.config.MultiUserMode,
		"require_auth":    s.config.RequireAuth,
		"guest_access":    s.config.GuestAccess,
		"version":         "0.2.4",
		"features": map[string]bool{
			"users":   s.config.MultiUserMode,
			"metrics": true, // Always available but can be hidden
			"agent":   true,
			"lora":    true,
		},
	})
}

// ============================================================================
// User Management Handlers
// ============================================================================

// handleCurrentUser returns the currently authenticated user
func (s *Server) handleCurrentUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user := users.GetUser(r)
	if user == nil || user.ID == "guest" {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"user":          nil,
			"authenticated": false,
			"guest":         user != nil && user.ID == "guest",
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"user":          user.ToPublic(),
		"authenticated": true,
	})
}

func (s *Server) handleUsers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Parse path: /v1/users, /v1/users/{id}, /v1/users/{id}/regenerate-key, /v1/users/{id}/role
	path := strings.TrimPrefix(r.URL.Path, "/v1/users")
	path = strings.TrimPrefix(path, "/")
	parts := strings.Split(path, "/")

	// Handle special sub-paths
	if len(parts) >= 2 && parts[0] != "" {
		userID := parts[0]
		action := parts[1]

		// Verify user exists
		_, ok := s.userStore.GetUser(userID)
		if !ok {
			http.Error(w, `{"error": "user not found"}`, http.StatusNotFound)
			return
		}

		switch action {
		case "regenerate-key":
			if r.Method != http.MethodPost {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			newKey, err := s.userStore.RegenerateAPIKey(userID)
			if err != nil {
				http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusBadRequest)
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"api_key": newKey})
			return

		case "role":
			if r.Method != http.MethodPut {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			var req struct {
				Role users.Role `json:"role"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, `{"error": "invalid request"}`, http.StatusBadRequest)
				return
			}
			if err := s.userStore.UpdateUser(userID, map[string]any{"role": req.Role}); err != nil {
				http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusBadRequest)
				return
			}
			// Update quota limits for new role
			s.quotaManager.InitUserQuota(userID, req.Role)
			json.NewEncoder(w).Encode(map[string]string{"status": "role updated"})
			return

		default:
			http.Error(w, `{"error": "unknown action"}`, http.StatusNotFound)
			return
		}
	}

	switch r.Method {
	case http.MethodGet:
		// List users or get specific user
		if len(parts) > 0 && parts[0] != "" {
			// Get specific user
			user, ok := s.userStore.GetUser(parts[0])
			if !ok {
				http.Error(w, `{"error": "user not found"}`, http.StatusNotFound)
				return
			}
			json.NewEncoder(w).Encode(user.ToPublic())
			return
		}
		// List all users
		userList := s.userStore.ListUsers()
		publicUsers := make([]users.UserPublic, 0, len(userList))
		for _, u := range userList {
			publicUsers = append(publicUsers, u.ToPublic())
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"users": publicUsers,
		})

	case http.MethodPost:
		// Create user
		var req struct {
			Username string     `json:"username"`
			Password string     `json:"password"`
			Role     users.Role `json:"role"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error": "invalid request"}`, http.StatusBadRequest)
			return
		}
		if req.Role == "" {
			req.Role = users.RoleUser
		}
		user, apiKey, err := s.userStore.CreateUser(req.Username, req.Password, req.Role)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusBadRequest)
			return
		}
		// Initialize quota for user
		s.quotaManager.InitUserQuota(user.ID, req.Role)
		json.NewEncoder(w).Encode(map[string]any{
			"user":    user.ToPublic(),
			"api_key": apiKey,
		})

	case http.MethodDelete:
		// Delete user
		if len(parts) == 0 || parts[0] == "" {
			http.Error(w, `{"error": "user id required"}`, http.StatusBadRequest)
			return
		}
		if err := s.userStore.DeleteUser(parts[0]); err != nil {
			http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusBadRequest)
			return
		}
		s.quotaManager.DeleteUserQuota(parts[0])
		json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "invalid request"}`, http.StatusBadRequest)
		return
	}

	user, ok := s.userStore.ValidatePassword(req.Username, req.Password)
	if !ok {
		http.Error(w, `{"error": "invalid credentials"}`, http.StatusUnauthorized)
		return
	}

	// Create session
	session, token, err := s.userStore.CreateSession(user.ID, r.RemoteAddr, r.UserAgent(), 24*time.Hour)
	if err != nil {
		http.Error(w, `{"error": "failed to create session"}`, http.StatusInternalServerError)
		return
	}

	// Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		Expires:  session.ExpiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"user":       user.ToPublic(),
		"token":      token,
		"expires_at": session.ExpiresAt,
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get token from cookie or header
	token := ""
	if cookie, err := r.Cookie("session"); err == nil {
		token = cookie.Value
	}
	if auth := r.Header.Get("Authorization"); auth != "" {
		token = strings.TrimPrefix(auth, "Bearer ")
	}

	if token != "" {
		s.userStore.DeleteSession(token)
	}

	// Clear cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "logged out"})
}

func (s *Server) handleQuota(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Get user from context or query
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		// Try to get from auth
		if user := users.GetUser(r); user != nil {
			userID = user.ID
		}
	}

	if userID == "" {
		http.Error(w, `{"error": "user_id required"}`, http.StatusBadRequest)
		return
	}

	summary := s.quotaManager.GetUsageSummary(userID)
	json.NewEncoder(w).Encode(summary)
}

// ============================================================================
// LoRA Adapter Handlers
// ============================================================================

func (s *Server) handleLoRAList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.loraManager.GetStatus())
}

func (s *Server) handleLoRA(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	path := strings.TrimPrefix(r.URL.Path, "/v1/lora/")
	parts := strings.Split(path, "/")

	switch r.Method {
	case http.MethodGet:
		if len(parts) == 0 || parts[0] == "" || parts[0] == "adapters" {
			// List all adapters - GetStatus already returns the right format
			json.NewEncoder(w).Encode(s.loraManager.GetStatus())
			return
		}
		adapter, ok := s.loraManager.GetAdapter(parts[0])
		if !ok {
			http.Error(w, `{"error": "adapter not found"}`, http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(adapter)

	case http.MethodPost:
		// Register or load adapter
		var req struct {
			Action      string  `json:"action"` // "register" or "load"
			Name        string  `json:"name"`
			Path        string  `json:"path"`
			Scale       float32 `json:"scale"`
			Description string  `json:"description"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error": "invalid request"}`, http.StatusBadRequest)
			return
		}

		switch req.Action {
		case "register":
			if req.Scale == 0 {
				req.Scale = 1.0
			}
			adapter, err := s.loraManager.RegisterAdapter("", req.Name, req.Path, req.Scale, req.Description)
			if err != nil {
				http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusBadRequest)
				return
			}
			json.NewEncoder(w).Encode(adapter)
		case "load":
			adapterID := parts[0]
			if err := s.loraManager.LoadAdapter(r.Context(), adapterID); err != nil {
				http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusBadRequest)
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"status": "loaded", "id": adapterID})
		case "unload":
			adapterID := parts[0]
			if err := s.loraManager.UnloadAdapter(r.Context(), adapterID); err != nil {
				http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusBadRequest)
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"status": "unloaded", "id": adapterID})
		default:
			http.Error(w, `{"error": "invalid action"}`, http.StatusBadRequest)
		}

	case http.MethodDelete:
		if len(parts) == 0 || parts[0] == "" {
			http.Error(w, `{"error": "adapter id required"}`, http.StatusBadRequest)
			return
		}
		if err := s.loraManager.DeleteAdapter(parts[0]); err != nil {
			http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusBadRequest)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// ============================================================================
// Agent Handlers
// ============================================================================

func (s *Server) handleAgentRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Prompt        string `json:"prompt"`
		Task          string `json:"task"` // Alias for prompt
		Model         string `json:"model"`
		Style         string `json:"style"`
		Stream        bool   `json:"stream"`
		MaxIterations int    `json:"max_iterations"`
		MaxSteps      int    `json:"max_steps"`
		SystemPrompt  string `json:"system_prompt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"})
		return
	}

	// Use Task as fallback for Prompt
	if req.Prompt == "" && req.Task != "" {
		req.Prompt = req.Task
	}

	// Validate prompt
	if req.Prompt == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "prompt or task is required"})
		return
	}

	// Validate model
	if req.Model == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "model is required"})
		return
	}

	// Get model metadata
	modelMeta, err := s.registry.GetModel(req.Model)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("model not found: %s", req.Model)})
		return
	}

	// Switch to requested model if different
	if err := s.switchModel(req.Model); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("failed to switch model: %v", err)})
		return
	}

	// Load model if not loaded
	if !modelMeta.IsLoaded {
		log.Printf("[Agent] Loading model: %s", req.Model)
		if err := s.registry.LoadModel(req.Model); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("failed to load model: %v", err)})
			return
		}

		ctx := context.Background()
		opts := inference.DefaultLoadOptions()
		opts.NumThreads = s.config.NumThreads
		opts.ContextSize = s.config.MaxContextSize

		if err := s.engine.Load(ctx, modelMeta.Path, opts); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("failed to load model into engine: %v", err)})
			return
		}
	}

	style := "react"
	switch req.Style {
	case "cot":
		style = "cot"
	case "plan", "plan-execute":
		style = "plan-execute"
	}

	maxIter := req.MaxIterations
	if maxIter == 0 {
		maxIter = req.MaxSteps
	}
	if maxIter == 0 {
		maxIter = 10
	}

	// Get tools from registry (includes built-in + user-defined + MCP tools)
	tools := s.toolRegistry.GetTools()
	executor := func(ctx context.Context, name string, args json.RawMessage) (string, error) {
		return s.toolRegistry.Execute(ctx, name, args)
	}

	// Use ReAct system prompt if not provided
	systemPrompt := req.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = agents.ReActSystemPrompt(tools)
	}

	agentCfg := agents.AgentConfig{
		SystemPrompt:   systemPrompt,
		ReasoningStyle: style,
		MaxIterations:  maxIter,
		MaxTokens:      2048,
		Temperature:    0.7,
		TimeoutPerStep: 120 * time.Second, // 2 minutes per step for model loading
	}

	// Check if streaming is requested
	stream := req.Stream

	if stream {
		// Stream the agent output using SSE
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming not supported", http.StatusInternalServerError)
			return
		}

		// Send initial status
		statusData, _ := json.Marshal(map[string]interface{}{
			"type":   "status",
			"status": "thinking",
		})
		fmt.Fprintf(w, "data: %s\n\n", statusData)
		flusher.Flush()

		// Create streaming LLM caller
		var fullResponse strings.Builder
		llmCaller := func(ctx context.Context, messages []api.ChatMessage, opts map[string]interface{}) (string, error) {
			if s.engine == nil {
				return "", fmt.Errorf("no LLM configured - load a model first")
			}

			// Build streaming chat request
			chatReq := api.ChatCompletionRequest{
				Model:    req.Model,
				Messages: messages,
				Stream:   true,
			}
			if temp, ok := opts["temperature"].(float64); ok {
				t := float32(temp)
				chatReq.Temperature = &t
			}
			if maxTokens, ok := opts["max_tokens"].(int); ok {
				chatReq.MaxTokens = &maxTokens
			}

			// Stream tokens
			fullResponse.Reset()
			err := s.engine.ChatCompletionStream(ctx, &chatReq, func(chunk string) error {
				fullResponse.WriteString(chunk)
				// Send each token to the client
				tokenData, _ := json.Marshal(map[string]interface{}{
					"type":  "token",
					"token": chunk,
				})
				fmt.Fprintf(w, "data: %s\n\n", tokenData)
				flusher.Flush()
				return nil
			})

			if err != nil {
				return "", err
			}

			return fullResponse.String(), nil
		}

		// Create agent with streaming LLM
		agent := agents.NewAgent(agentCfg, tools, executor, llmCaller)

		// Set up step callback
		agent.SetStepCallback(func(step agents.Step) {
			stepData := map[string]interface{}{
				"type":        "step",
				"step_type":   step.Type,
				"step_id":     step.ID,
				"content":     step.Content,
				"tool_name":   step.ToolName,
				"tool_args":   step.ToolArgs,
				"tool_result": step.ToolResult,
			}
			jsonData, _ := json.Marshal(stepData)
			fmt.Fprintf(w, "data: %s\n\n", jsonData)
			flusher.Flush()
		})

		// Run the agent
		result, err := agent.Run(r.Context(), req.Prompt)

		// Send final result
		if err != nil {
			errData, _ := json.Marshal(map[string]interface{}{
				"type":  "error",
				"error": err.Error(),
			})
			fmt.Fprintf(w, "data: %s\n\n", errData)
		} else {
			doneData, _ := json.Marshal(map[string]interface{}{
				"type":   "done",
				"output": result,
			})
			fmt.Fprintf(w, "data: %s\n\n", doneData)
		}
		flusher.Flush()
		return
	}

	// Non-streaming: regular LLM caller
	llmCaller := func(ctx context.Context, messages []api.ChatMessage, opts map[string]interface{}) (string, error) {
		// Check if a model is loaded
		if s.engine == nil {
			return "", fmt.Errorf("no LLM configured - load a model first")
		}

		// Build chat request
		chatReq := api.ChatCompletionRequest{
			Model:    req.Model,
			Messages: messages,
		}
		if temp, ok := opts["temperature"].(float64); ok {
			t := float32(temp)
			chatReq.Temperature = &t
		}
		if maxTokens, ok := opts["max_tokens"].(int); ok {
			chatReq.MaxTokens = &maxTokens
		}

		// Use server's chat completion
		resp, err := s.engine.ChatCompletion(ctx, &chatReq)
		if err != nil {
			return "", err
		}

		if len(resp.Choices) > 0 {
			return resp.Choices[0].Message.StringContent(), nil
		}
		return "", fmt.Errorf("no response from model")
	}

	// Non-streaming agent
	agent := agents.NewAgent(agentCfg, tools, executor, llmCaller)
	result, err := agent.Run(r.Context(), req.Prompt)
	if err != nil {
		log.Printf("[Agent] Error: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"output": result,
		"steps":  agent.GetSteps(),
	})
}

func (s *Server) handleAgentTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.agentManager.ListTasks())
}

func (s *Server) handleAgentWorkflows(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		// Workflows are managed by WorkflowEngine - return info message
		json.NewEncoder(w).Encode(map[string]any{
			"workflows": []any{},
			"message":   "Use POST to register workflows",
		})

	case http.MethodPost:
		var wf agents.Workflow
		if err := json.NewDecoder(r.Body).Decode(&wf); err != nil {
			http.Error(w, `{"error": "invalid request"}`, http.StatusBadRequest)
			return
		}
		// Would need a WorkflowEngine instance to register
		json.NewEncoder(w).Encode(map[string]string{"status": "registered", "id": wf.ID})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// ============================================================================
// WebSocket Handler
// ============================================================================

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Upgrade(w, r)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	// Register with hub
	s.wsHub.Register(conn)

	// Set up message handler
	conn.SetMessageHandler(func(msgType websocket.MessageType, data []byte) {
		// Process message
		var request map[string]any
		if err := json.Unmarshal(data, &request); err != nil {
			return
		}

		// Handle different message types
		reqType, _ := request["type"].(string)
		switch reqType {
		case "subscribe":
			channel, _ := request["channel"].(string)
			s.wsHub.Subscribe(conn, channel)
		case "unsubscribe":
			channel, _ := request["channel"].(string)
			s.wsHub.Unsubscribe(conn, channel)
		case "ping":
			conn.WriteJSON(map[string]string{"type": "pong"})
		}
	})

	// Set close handler to unregister from hub
	conn.SetCloseHandler(func() {
		s.wsHub.Unregister(conn)
	})

	// Start the read loop (blocking)
	conn.ReadLoop()
}

// handleAgentTools lists and manages agent tools
func (s *Server) handleAgentTools(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		// Check for ?all=true to include disabled tools with status
		if r.URL.Query().Get("all") == "true" {
			toolsWithStatus := s.toolRegistry.GetAllToolsWithStatus()
			json.NewEncoder(w).Encode(map[string]interface{}{
				"tools":         toolsWithStatus,
				"total":         len(toolsWithStatus),
				"enabled_count": s.toolRegistry.GetEnabledCount(),
			})
			return
		}

		// List only enabled tools
		tools := s.toolRegistry.GetTools()

		type ToolInfo struct {
			Name        string                 `json:"name"`
			Description string                 `json:"description"`
			Parameters  map[string]interface{} `json:"parameters,omitempty"`
			Type        string                 `json:"type"`
		}

		var toolList []ToolInfo
		for _, tool := range tools {
			toolList = append(toolList, ToolInfo{
				Name:        tool.Function.Name,
				Description: tool.Function.Description,
				Parameters:  tool.Function.Parameters,
				Type:        tool.Type,
			})
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"tools": toolList,
			"count": len(toolList),
		})

	case http.MethodPatch:
		// Enable or disable a tool
		var req struct {
			Name    string `json:"name"`
			Enabled bool   `json:"enabled"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"})
			return
		}

		if req.Name == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "tool name is required"})
			return
		}

		if err := s.toolRegistry.SetToolEnabled(req.Name, req.Enabled); err != nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		action := "disabled"
		if req.Enabled {
			action = "enabled"
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":        action,
			"tool":          req.Name,
			"enabled_count": s.toolRegistry.GetEnabledCount(),
		})

	case http.MethodPost:
		// Register a new tool dynamically
		var req struct {
			Name        string                 `json:"name"`
			Description string                 `json:"description"`
			Parameters  map[string]interface{} `json:"parameters"`
			Type        string                 `json:"type"` // "shell", "http", "script"
			Command     string                 `json:"command,omitempty"`
			URL         string                 `json:"url,omitempty"`
			Script      string                 `json:"script,omitempty"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"})
			return
		}

		if req.Name == "" || req.Description == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "name and description are required"})
			return
		}

		// Create tool definition
		tool := api.Tool{
			Type: "function",
			Function: api.FunctionDef{
				Name:        req.Name,
				Description: req.Description,
				Parameters:  req.Parameters,
			},
		}

		// Create executor based on type
		var executor agents.SimpleExecutor
		switch req.Type {
		case "shell":
			command := req.Command
			executor = func(ctx context.Context, args json.RawMessage) (string, error) {
				return agents.ExecuteTool(ctx, "shell", json.RawMessage(fmt.Sprintf(`{"command": "%s"}`, command)))
			}
		case "http":
			url := req.URL
			executor = func(ctx context.Context, args json.RawMessage) (string, error) {
				return agents.ExecuteTool(ctx, "http_get", json.RawMessage(fmt.Sprintf(`{"url": "%s"}`, url)))
			}
		default:
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "unsupported tool type, use 'shell' or 'http'"})
			return
		}

		s.toolRegistry.RegisterTool(tool, executor)

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "registered",
			"tool":   req.Name,
		})

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
	}
}

// handleAgentMCP manages MCP server connections
func (s *Server) handleAgentMCP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		// List connected MCP servers
		tools := s.toolRegistry.GetTools()
		mcpServers := make(map[string]int)

		for _, tool := range tools {
			// Check if tool has MCP source
			if tool.Function.Name != "" {
				// This is a simple implementation - in production, track sources properly
				mcpServers["mcp"] = len(tools)
			}
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"servers": mcpServers,
		})

	case http.MethodPost:
		// Add new MCP server
		var req struct {
			Name string `json:"name"`
			URL  string `json:"url"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"})
			return
		}

		if req.Name == "" || req.URL == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "name and url are required"})
			return
		}

		// Try to connect to the MCP server and discover tools
		count, err := s.toolRegistry.LoadMCPTools(req.Name, req.URL)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": fmt.Sprintf("failed to connect to MCP server: %v", err),
			})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":      "connected",
			"server":      req.Name,
			"tools_added": count,
		})

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
	}
}

// handleAgentMCPTest tests connectivity to an MCP server without adding it
func (s *Server) handleAgentMCPTest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
		return
	}

	var req struct {
		URL string `json:"url"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"})
		return
	}

	if req.URL == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "url is required"})
		return
	}

	// Test connection to MCP server
	count, err := s.toolRegistry.TestMCPConnection(req.URL)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": fmt.Sprintf("Connection failed: %v", err),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "ok",
		"tools_count": count,
	})
}
