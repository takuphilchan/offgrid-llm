package server

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/artifacts"
	"github.com/takuphilchan/offgrid-llm/internal/audio"
	"github.com/takuphilchan/offgrid-llm/internal/audit"
	"github.com/takuphilchan/offgrid-llm/internal/cache"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/computer"
	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/degradation"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/integrations"
	"github.com/takuphilchan/offgrid-llm/internal/mcp"
	"github.com/takuphilchan/offgrid-llm/internal/metrics"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/internal/p2p"
	"github.com/takuphilchan/offgrid-llm/internal/power"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"github.com/takuphilchan/offgrid-llm/internal/resource"
	"github.com/takuphilchan/offgrid-llm/internal/runs"
	"github.com/takuphilchan/offgrid-llm/internal/stats"
	"github.com/takuphilchan/offgrid-llm/internal/tools"
	"github.com/takuphilchan/offgrid-llm/internal/users"
	"github.com/takuphilchan/offgrid-llm/internal/websocket"
)

// Server represents the HTTP server
type Server struct {
	agentToolPolicy      *inference.AgentToolPolicy // Installed at startup; nil preserves constrained behavior.
	audioMutex           sync.Mutex
	audioEngine          *audio.Engine
	speechPackages       *audio.PackageRuntime
	httpServer           *http.Server
	config               *config.Config
	registry             *models.Registry
	engine               inference.Engine
	embeddingEngine      *inference.EmbeddingEngine
	ragEngine            *rag.Engine
	monitor              *resource.Monitor
	systemResources      *resource.SystemResources // Stable startup snapshot used for admission, never a live-task resize signal.
	statsTracker         *stats.Tracker
	cache                *cache.ResponseCache
	startTime            time.Time
	downloadProgress     map[string]*DownloadProgress
	downloadMutex        sync.RWMutex
	downloadCancelFuncs  map[string]context.CancelFunc // Cancel functions for active downloads
	downloadWorkers      sync.WaitGroup
	modelOperations      map[string]*ModelOperation // Same coordinator and snapshot as legacy downloads.
	downloadStateWriter  func(string, any) error    // Instance-local fault injection; nil uses durable storage.
	modelResolutions     map[string]modelResolutionRecord
	modelHub             *models.HuggingFaceClient
	exportProgress       map[string]*ExportProgress
	exportMutex          sync.RWMutex
	modelCache           *inference.ModelCache
	llamaProcess         inference.LlamaProcessLifecycle
	currentModelID       string
	currentContext       int
	currentPort          int
	currentGeneration    uint64
	modelMutex           sync.Mutex
	inferenceLifecycle   *inference.LifecycleGate
	providerScheduler    *inference.ProviderScheduler
	providerRegistry     *inference.ProviderRegistry
	nativeRuntimeMu      sync.RWMutex
	nativeInstallSealed  bool
	nativeChatSelector   nativeChatSelector // nil until trusted runtime and platform qualification are installed
	rateLimiter          *RateLimiter
	inferenceRateLimiter *InferenceRateLimiter
	sessionHandlers      *SessionHandlers
	authMiddleware       *users.Middleware
	// New feature managers
	userStore            *users.UserStore
	quotaManager         *users.QuotaManager
	kbManager            *users.KnowledgeBaseManager
	loraManager          *inference.LoRAManager
	agentManager         *agents.Manager
	agentRunner          *agents.Runner
	toolRegistry         *agents.ToolRegistry
	mcpHandler           http.Handler
	p2pDiscovery         *p2p.Discovery
	p2pTransfer          *p2p.TransferManager
	p2pIdentity          *p2p.Identity
	p2pTrust             *p2p.TrustStore
	offgridMetrics       *metrics.OffGridMetrics
	wsHub                *websocket.Hub
	powerManager         *power.PowerManager
	degradationMgr       *degradation.Manager     // Graceful degradation under resource pressure
	auditLogger          *audit.AuditLogger       // Enterprise audit logging (optional)
	ldapAuth             *users.LDAPAuthenticator // LDAP/Active Directory authentication
	mcpMarketplace       *mcp.Marketplace         // MCP server marketplace
	loadBalancer         *inference.LoadBalancer  // Multi-backend load balancer
	distributedRAG       *rag.DistributedRAG      // Distributed RAG index
	pluginManager        *tools.PluginManager     // Plugin system for custom tools
	capabilityBroker     *capabilities.Broker     // Shared authorization boundary for tools and agents
	runLog               *runs.Log                // Durable event stream for agent runs
	artifactStore        *artifacts.Store         // Content-addressed agent outputs and captures
	browserHub           *computer.BrowserHub
	executionHosts       *execution.Hosts
	executionEnrollment  *execution.Enrollment
	schedules            *scheduleController
	computerVisionChecks sync.Map               // exact runtime/model/projector smoke passes; never persisted
	computerModelChecks  computerCheckFlights   // synthetic in-flight work only; no completed-result cache
	integrationRegistry  *integrations.Registry // External agent adapters (Hermes, OpenClaw, ...)
	closeOnce            sync.Once
	closeErr             error
	runtimeCtx           context.Context
	runtimeCancel        context.CancelFunc
	workspaceID          string
	startupWorkers       sync.WaitGroup
	// Runtime tracking
	requestCount       int64
	wsConnections      int64
	tokensGenerated    int64
	errorsTotal        int64
	totalLatencyMicros int64 // For calculating average latency
	// Cached values (read once at startup)
	version string // Cached VERSION file content
}

type DownloadProgress struct {
	FileName        string  `json:"file_name"`
	BytesTotal      int64   `json:"bytes_total"`
	BytesDone       int64   `json:"bytes_done"`
	Percent         float64 `json:"percent"`
	Speed           float64 `json:"speed"`      // bytes per second
	StartedAt       int64   `json:"started_at"` // unix timestamp
	Status          string  `json:"status"`     // "downloading", "complete", "failed"
	Error           string  `json:"error,omitempty"`
	Repository      string  `json:"repository,omitempty"`
	SourceFile      string  `json:"source_file,omitempty"`
	ModelID         string  `json:"model_id,omitempty"`
	Quantization    string  `json:"quantization,omitempty"`
	EnableKnowledge bool    `json:"enable_knowledge,omitempty"`
}

type ExportProgress struct {
	FileName   string  `json:"file_name"`
	BytesTotal int64   `json:"bytes_total"`
	BytesDone  int64   `json:"bytes_done"`
	Percent    float64 `json:"percent"`
	Status     string  `json:"status"` // "exporting", "complete", "failed"
	Error      string  `json:"error,omitempty"`
}

// Start starts the HTTP server
func (s *Server) Start() error {
	return s.Serve(context.Background())
}

// Handler exposes the same router and middleware chain for the composed
// runtime and for isolated HTTP tests. It has no listener side effect.
func (s *Server) Handler() http.Handler {
	return s.buildRouter()
}

// Serve runs the existing HTTP lifecycle and stops dispatch when ctx ends.
// Start remains a compatibility entry until app.Runtime owns the process.
func (s *Server) Serve(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("serve context is required")
	}
	defer s.Close()
	if s.executionEnrollment != nil && s.config.EnableExecutionPreview {
		go s.executionEnrollment.Run(s.runtimeCtx)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	s.nativeRuntimeMu.Lock()
	s.nativeInstallSealed = true
	s.nativeRuntimeMu.Unlock()

	if err := validateServerExposure(s.config); err != nil {
		return err
	}

	handler := s.Handler()

	listenAddr := serverListenAddress(s.config.ServerHost, s.config.ServerPort)
	if !s.config.RequireAuth && s.config.AllowUnauthenticatedRemote && !isLoopbackHost(s.config.ServerHost) {
		log.Printf("SECURITY WARNING: authentication is disabled while listening on non-loopback host %q", s.config.ServerHost)
	}

	s.httpServer = &http.Server{
		BaseContext:  func(net.Listener) context.Context { return s.runtimeCtx },
		Addr:         listenAddr,
		Handler:      handler,
		ReadTimeout:  5 * time.Minute,  // Increased for low-end machines
		WriteTimeout: 15 * time.Minute, // Long timeout for LLM inference on low-end machines
		IdleTimeout:  5 * time.Minute,  // Increased for low-end machines
	}

	// Graceful shutdown handler
	go s.handleShutdown()

	// Auto-start llama-server if using LlamaHTTPEngine
	if err := s.startLlamaServer(); err != nil {
		log.Printf("Warning: Failed to auto-start llama-server: %v", err)
		log.Println("You may need to start llama-server manually")
	}

	// Auto-enable RAG if an embedding model is available
	s.startRAGAutoRestore()

	// Start P2P services if enabled
	if s.config.EnableP2P && s.p2pDiscovery != nil && s.p2pTransfer != nil {
		// Update local models for discovery
		modelIDs := make([]string, 0)
		for _, m := range s.registry.ListModels() {
			modelIDs = append(modelIDs, m.ID)
		}
		s.p2pDiscovery.SetLocalModels(modelIDs)

		// Start discovery
		if err := s.p2pDiscovery.Start(s.runtimeCtx); err != nil {
			log.Printf("Failed to start P2P discovery: %v", err)
		}

		// Start transfer server
		if err := s.p2pTransfer.StartServer(s.runtimeCtx); err != nil {
			log.Printf("Failed to start P2P transfer server: %v", err)
		}
	}

	// Start background metrics collection
	go s.collectSystemMetrics()

	// Create listener first to ensure port is available
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("failed to bind to port %d: %w", s.config.ServerPort, err)
	}
	stopOnCancel := context.AfterFunc(ctx, func() { _ = s.Close() })
	defer stopOnCancel()
	if err := ctx.Err(); err != nil {
		_ = listener.Close()
		return err
	}

	fmt.Println()
	fmt.Println("OffGrid server")
	fmt.Println()

	// Show hardware info
	if res, err := resource.DetectResources(); err == nil {
		if res.GPUAvailable && res.GPUName != "" {
			fmt.Printf("Device:  %s (%dMB VRAM)\n", res.GPUName, res.GPUMemory)
		} else {
			fmt.Printf("Device:  CPU (%d cores, %dMB RAM)\n", res.CPUCores, res.AvailableRAM)
		}
	}

	fmt.Printf("Server:  http://%s\n", listenAddr)
	fmt.Printf("Web UI:  http://%s/ui/\n", listenAddr)
	fmt.Println()
	fmt.Println("API:")
	fmt.Printf("  POST /v1/chat/completions\n")
	fmt.Printf("  POST /v1/embeddings\n")
	fmt.Printf("  GET  /v1/models\n")
	fmt.Println()
	fmt.Printf("OK Server ready on port %d\n", s.config.ServerPort)
	fmt.Println()

	if err := s.httpServer.Serve(listener); err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	return nil
}

// Close releases long-lived runtime resources owned by the server. It is safe
// to call more than once and is also used by tests that construct a server
// without starting its HTTP listener.
func (s *Server) Close() error {
	s.closeOnce.Do(func() {
		if s.executionEnrollment != nil {
			_ = s.executionEnrollment.Close()
		}
		if s.executionHosts != nil {
			_ = s.executionHosts.Close()
		}
		if s.browserHub != nil {
			s.browserHub.Stop("")
		}
		if s.runtimeCancel != nil {
			s.runtimeCancel()
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if s.httpServer != nil {
			if err := s.httpServer.Shutdown(ctx); err != nil {
				// Retain ownership if a handler can still write. Process exit
				// releases the OS lock; a new service must not race cleanup.
				s.closeErr = fmt.Errorf("drain HTTP requests: %w", err)
				return
			}
		}
		if s.agentRunner != nil {
			if err := s.agentRunner.Shutdown(ctx); err != nil {
				s.closeErr = fmt.Errorf("drain agent workers: %w", err)
				return
			}
		}
		if err := s.computerModelChecks.shutdown(ctx); err != nil {
			s.closeErr = fmt.Errorf("drain synthetic model checks: %w", err)
			return
		}
		s.startupWorkers.Wait()
		s.audioMutex.Lock()
		if s.speechPackages != nil {
			s.speechPackages.Close()
		}
		if s.audioEngine != nil {
			s.audioEngine.Close()
		}
		s.audioMutex.Unlock()
		s.downloadWorkers.Wait()
	})
	return s.closeErr
}
