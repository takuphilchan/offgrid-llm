package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/artifacts"
	"github.com/takuphilchan/offgrid-llm/internal/audit"
	"github.com/takuphilchan/offgrid-llm/internal/cache"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/computer"
	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/degradation"
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

// HTTPConfig carries the validated runtime configuration during the internal
// composition migration. The server reads it but never validates it by making
// directories or acquiring workspace ownership.
type HTTPConfig struct {
	Runtime *config.Config
}

// Dependencies is a typed, consumer-owned boundary. App owns construction and
// cleanup; server only borrows these components for HTTP transport and work.
// Optional capabilities may be nil, but the fields validated below may not.
type Dependencies struct {
	Context   context.Context
	Version   string
	Workspace string

	Registry             *models.Registry
	Engine               inference.Engine
	EmbeddingEngine      *inference.EmbeddingEngine
	RAGEngine            *rag.Engine
	Monitor              *resource.Monitor
	SystemResources      *resource.SystemResources
	StatsTracker         *stats.Tracker
	ResponseCache        *cache.ResponseCache
	ModelCache           *inference.ModelCache
	LlamaProcess         inference.LlamaProcessLifecycle
	InferenceLifecycle   *inference.LifecycleGate
	ProviderScheduler    *inference.ProviderScheduler
	ProviderRegistry     *inference.ProviderRegistry
	RateLimiter          *RateLimiter
	InferenceRateLimiter *InferenceRateLimiter

	SessionHandlers *SessionHandlers
	AuthMiddleware  *users.Middleware
	UserStore       *users.UserStore
	QuotaManager    *users.QuotaManager
	KBManager       *users.KnowledgeBaseManager
	LoRAManager     *inference.LoRAManager

	AgentManager        *agents.Manager
	AgentRunner         *agents.Runner
	ToolRegistry        *agents.ToolRegistry
	MCPHandler          http.Handler
	CapabilityBroker    *capabilities.Broker
	RunLog              *runs.Log
	ArtifactStore       *artifacts.Store
	BrowserHub          *computer.BrowserHub
	IntegrationRegistry *integrations.Registry

	P2PDiscovery   *p2p.Discovery
	P2PTransfer    *p2p.TransferManager
	P2PIdentity    *p2p.Identity
	P2PTrust       *p2p.TrustStore
	Metrics        *metrics.OffGridMetrics
	WebSocketHub   *websocket.Hub
	PowerManager   *power.PowerManager
	Degradation    *degradation.Manager
	AuditLogger    *audit.AuditLogger
	LDAPAuth       *users.LDAPAuthenticator
	MCPMarketplace *mcp.Marketplace
	LoadBalancer   *inference.LoadBalancer
	DistributedRAG *rag.DistributedRAG
	PluginManager  *tools.PluginManager
}

func validateDependencies(cfg HTTPConfig, deps Dependencies) error {
	if cfg.Runtime == nil {
		return fmt.Errorf("HTTP configuration is required")
	}
	if deps.Context == nil {
		return fmt.Errorf("runtime context is required")
	}
	if deps.Registry == nil || deps.Engine == nil || deps.ModelCache == nil || deps.Monitor == nil ||
		deps.EmbeddingEngine == nil || deps.RAGEngine == nil || deps.StatsTracker == nil ||
		deps.ResponseCache == nil || deps.InferenceLifecycle == nil ||
		deps.ProviderScheduler == nil || deps.ProviderRegistry == nil {
		return fmt.Errorf("model and resource dependencies are required")
	}
	if deps.RateLimiter == nil || deps.InferenceRateLimiter == nil || deps.AuthMiddleware == nil || deps.MCPHandler == nil {
		return fmt.Errorf("HTTP enforcement dependencies are required")
	}
	if deps.SessionHandlers == nil || deps.UserStore == nil || deps.Metrics == nil ||
		deps.QuotaManager == nil || deps.KBManager == nil || deps.LoRAManager == nil ||
		deps.AgentManager == nil || deps.ToolRegistry == nil || deps.CapabilityBroker == nil ||
		deps.BrowserHub == nil || deps.IntegrationRegistry == nil || deps.MCPMarketplace == nil ||
		deps.PluginManager == nil || deps.WebSocketHub == nil || deps.PowerManager == nil ||
		deps.Degradation == nil {
		return fmt.Errorf("session, identity and metrics dependencies are required")
	}
	return nil
}

// NewTransport assembles HTTP state without acquiring storage, loading models,
// starting services or claiming ownership of its borrowed dependencies. The
// legacy concrete constructor remains in use until app.New is wired.
func NewTransport(cfg HTTPConfig, deps Dependencies) (*Server, error) {
	if err := validateDependencies(cfg, deps); err != nil {
		return nil, err
	}
	runtimeCtx, runtimeCancel := context.WithCancel(deps.Context)
	server := &Server{
		config:               cfg.Runtime,
		registry:             deps.Registry,
		engine:               deps.Engine,
		embeddingEngine:      deps.EmbeddingEngine,
		ragEngine:            deps.RAGEngine,
		monitor:              deps.Monitor,
		systemResources:      deps.SystemResources,
		statsTracker:         deps.StatsTracker,
		cache:                deps.ResponseCache,
		startTime:            time.Now(),
		downloadProgress:     make(map[string]*DownloadProgress),
		downloadCancelFuncs:  make(map[string]context.CancelFunc),
		exportProgress:       make(map[string]*ExportProgress),
		modelCache:           deps.ModelCache,
		llamaProcess:         deps.LlamaProcess,
		inferenceLifecycle:   deps.InferenceLifecycle,
		providerScheduler:    deps.ProviderScheduler,
		providerRegistry:     deps.ProviderRegistry,
		rateLimiter:          deps.RateLimiter,
		inferenceRateLimiter: deps.InferenceRateLimiter,
		sessionHandlers:      deps.SessionHandlers,
		authMiddleware:       deps.AuthMiddleware,
		userStore:            deps.UserStore,
		quotaManager:         deps.QuotaManager,
		kbManager:            deps.KBManager,
		loraManager:          deps.LoRAManager,
		agentManager:         deps.AgentManager,
		agentRunner:          deps.AgentRunner,
		toolRegistry:         deps.ToolRegistry,
		mcpHandler:           deps.MCPHandler,
		p2pDiscovery:         deps.P2PDiscovery,
		p2pTransfer:          deps.P2PTransfer,
		p2pIdentity:          deps.P2PIdentity,
		p2pTrust:             deps.P2PTrust,
		offgridMetrics:       deps.Metrics,
		wsHub:                deps.WebSocketHub,
		powerManager:         deps.PowerManager,
		degradationMgr:       deps.Degradation,
		auditLogger:          deps.AuditLogger,
		ldapAuth:             deps.LDAPAuth,
		mcpMarketplace:       deps.MCPMarketplace,
		loadBalancer:         deps.LoadBalancer,
		distributedRAG:       deps.DistributedRAG,
		pluginManager:        deps.PluginManager,
		capabilityBroker:     deps.CapabilityBroker,
		runLog:               deps.RunLog,
		artifactStore:        deps.ArtifactStore,
		browserHub:           deps.BrowserHub,
		integrationRegistry:  deps.IntegrationRegistry,
		runtimeCtx:           runtimeCtx,
		runtimeCancel:        runtimeCancel,
		workspaceID:          deps.Workspace,
		version:              deps.Version,
	}
	server.modelCache.SetLoadAdmission(server.checkModelLoadCapacity)
	return server, nil
}
