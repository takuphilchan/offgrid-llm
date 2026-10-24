package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/artifacts"
	"github.com/takuphilchan/offgrid-llm/internal/audit"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/computer"
	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/degradation"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/integrations"
	"github.com/takuphilchan/offgrid-llm/internal/mcp"
	"github.com/takuphilchan/offgrid-llm/internal/metrics"
	"github.com/takuphilchan/offgrid-llm/internal/p2p"
	"github.com/takuphilchan/offgrid-llm/internal/power"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"github.com/takuphilchan/offgrid-llm/internal/runs"
	"github.com/takuphilchan/offgrid-llm/internal/server"
	"github.com/takuphilchan/offgrid-llm/internal/tools"
	"github.com/takuphilchan/offgrid-llm/internal/users"
	"github.com/takuphilchan/offgrid-llm/internal/websocket"
)

// New constructs one complete application graph. On any failure acquired
// resources are released in reverse order; no partially initialized transport
// is returned or started.
func New(cfg *config.Config) (_ *Runtime, err error) {
	c, err := newCore(cfg)
	if err != nil {
		return nil, err
	}
	var transport *server.Server
	defer func() {
		if err != nil {
			if transport != nil {
				err = errors.Join(err, transport.Close())
			}
			err = c.owned.rollback(err)
		}
	}()

	// HTTP limits and the RAG resolver preserve the legacy acquisition order.
	rateLimiter := server.NewRateLimiter(600, time.Minute, 60)
	inferenceRateLimiter := server.NewInferenceRateLimiter(10, 20)
	ragEngine := rag.NewEngine(c.embedding, cfg.DataDir)
	c.owned.add("RAG engine", ragEngine.Close)
	ragEngine.SetModelResolver(func(modelID string) (string, error) {
		model, lookupErr := c.registry.GetModel(modelID)
		if lookupErr != nil {
			return "", lookupErr
		}
		if model.Type != "embedding" {
			return "", fmt.Errorf("model %q is %q, not an embedding model", modelID, model.Type)
		}
		return model.Path, nil
	})
	sessionHandlers := server.NewSessionHandlers(filepath.Join(cfg.DataDir, "sessions"))
	c.owned.add("session turns", sessionHandlers.Close)
	if err := sessionHandlers.RecoverTurns(); err != nil {
		return nil, fmt.Errorf("recover conversations: %w", err)
	}
	if err := c.initProviderScheduler(cfg); err != nil {
		return nil, err
	}
	userStore := users.NewUserStore(cfg.DataDir)
	quotaManager := users.NewQuotaManager(cfg.DataDir)
	kbManager := users.NewKnowledgeBaseManager(cfg.DataDir)
	loraManager := inference.NewLoRAManager(cfg.DataDir, c.engine)
	var sandbox agents.Sandbox = agents.NewLazySandbox("python:3.10-slim")
	c.owned.add("agent sandbox", sandbox.Cleanup)
	executor := agents.NewSandboxedExecutor(sandbox)
	agentManager := agents.NewManagerWithPersistence(nil, executor, nil, cfg.DataDir)
	toolRegistry := agents.NewToolRegistry()
	if loadErr := toolRegistry.LoadUserTools(filepath.Join(cfg.DataDir, "tools.json")); loadErr != nil {
		log.Printf("Warning: Failed to load user tools: %v", loadErr)
	}
	capabilityBroker := capabilities.NewBroker(capabilities.DefaultPolicy{})
	toolRegistry.SetCapabilityBroker(capabilityBroker)
	runLog, logErr := runs.NewLog(filepath.Join(cfg.DataDir, "runs", "events.jsonl"))
	if logErr != nil {
		log.Printf("Warning: Failed to initialize durable run log: %v", logErr)
	}
	artifactStore, artifactErr := artifacts.NewStore(filepath.Join(cfg.DataDir, "artifacts"))
	if artifactErr != nil {
		log.Printf("Warning: Failed to initialize artifact store: %v", artifactErr)
	}
	integrationRegistry := integrations.NewDefaultRegistry()
	offgridMetrics := metrics.NewOffGridMetrics()
	wsHub := websocket.NewHub()
	runtimeCtx, runtimeCancel := context.WithCancel(context.Background())
	c.owned.add("runtime context", func() error { runtimeCancel(); return nil })
	go wsHub.Run(runtimeCtx)
	powerManager := power.NewPowerManager(power.DefaultPolicy())
	powerManager.Start()
	c.owned.add("power manager", func() error { powerManager.Stop(); return nil })
	powerStatus := powerManager.Status()
	if powerStatus.State == power.PowerBattery {
		log.Printf("Power: Battery %d%% (informational; model context unchanged)", powerStatus.BatteryPercent)
	} else if powerStatus.State == power.PowerAC {
		log.Printf("Power: AC connected - Full performance")
	}
	powerManager.OnChange(func(status power.PowerStatus) {
		if status.State == power.PowerBattery {
			log.Printf("Switched to battery (%d%%); model context unchanged", status.BatteryPercent)
		} else if status.State == power.PowerAC {
			log.Printf("AC power connected - full performance available")
		}
	})
	degradationMgr := degradation.NewManager(degradation.DefaultConfig())
	degradationMgr.Start()
	c.owned.add("degradation manager", func() error { degradationMgr.Stop(); return nil })
	degradationMgr.OnLevelChange(func(level degradation.DegradationLevel) {
		log.Printf("Resource pressure changed: %s (max concurrent: %d; model context unchanged)", level.String(), degradationMgr.MaxConcurrentRequests())
	})
	var auditLogger *audit.AuditLogger
	if cfg.EnableAuditLog {
		auditDir := cfg.AuditLogDir
		if auditDir == "" {
			auditDir = filepath.Join(cfg.DataDir, "audit")
		}
		auditLogger, err = audit.NewAuditLogger(audit.AuditConfig{LogDir: auditDir, MaxFileSize: 50, RetentionDays: 365})
		if err != nil {
			log.Printf("Warning: Failed to initialize audit logger: %v", err)
			err = nil
		} else {
			log.Printf("Audit logging enabled: %s", auditDir)
		}
	}
	authMiddleware := users.NewMiddleware(userStore)
	authMiddleware.SetRequireAuth(cfg.RequireAuth)
	authMiddleware.SetGuestEnabled(cfg.GuestAccess)
	for _, path := range []string{"/health", "/ready", "/livez", "/readyz", "/metrics", "/openapi.yaml"} {
		authMiddleware.AddBypassPath(path)
	}
	var ldapAuth *users.LDAPAuthenticator
	if cfg.LDAPEnabled && cfg.LDAPServer != "" {
		ldapProvider := users.NewLDAPAuthProvider(users.LDAPConfig{
			Server: cfg.LDAPServer, Port: cfg.LDAPPort, UseTLS: cfg.LDAPUseTLS, StartTLS: cfg.LDAPStartTLS,
			BindDN: cfg.LDAPBindDN, BindPassword: cfg.LDAPBindPassword, BaseDN: cfg.LDAPBaseDN,
			UserFilter: cfg.LDAPUserFilter, AdminGroups: cfg.LDAPAdminGroups, UserGroups: cfg.LDAPUserGroups,
			DefaultRole: users.RoleUser, InsecureSkipVerify: cfg.LDAPSkipTLSVerify,
		})
		if testErr := ldapProvider.TestConnection(); testErr != nil {
			log.Printf("Warning: LDAP connection test failed: %v", testErr)
		} else {
			log.Printf("LDAP authentication enabled: %s:%d", cfg.LDAPServer, cfg.LDAPPort)
			ldapAuth = users.NewLDAPAuthenticator(ldapProvider, userStore, cfg.LDAPAutoCreate)
		}
	}
	mcpMarketplace := mcp.NewMarketplace(cfg.DataDir)
	var loadBalancer *inference.LoadBalancer
	if len(cfg.LoadBalancerBackends) > 0 {
		backends := make([]inference.Backend, 0, len(cfg.LoadBalancerBackends))
		for _, b := range cfg.LoadBalancerBackends {
			backends = append(backends, inference.Backend{ID: b.ID, Name: b.Name, URL: b.URL, Type: inference.BackendType(b.Type), APIKey: b.APIKey, Models: b.Models, Weight: b.Weight, Enabled: true})
		}
		loadBalancer = inference.NewLoadBalancer(inference.LoadBalancerConfig{Backends: backends, Strategy: inference.BalancingStrategy(cfg.LoadBalancerStrategy), HealthCheckSeconds: 30, HealthCheckPath: "/health"})
		c.owned.add("load balancer", func() error { loadBalancer.Stop(); return nil })
		log.Printf("Load balancer enabled with %d backends", len(backends))
	}
	var distributedRAG *rag.DistributedRAG
	if len(cfg.DistributedRAGNodes) > 0 {
		distributedRAG = rag.NewDistributedRAG(rag.DistributedRAGConfig{LocalNodeID: cfg.NodeID, SearchPath: "/v1/rag/search", HealthPath: "/v1/rag/stats", TimeoutSeconds: 10, HealthCheckSeconds: 30})
		for _, n := range cfg.DistributedRAGNodes {
			distributedRAG.AddNode(rag.DistributedNode{ID: n.ID, URL: n.URL, Name: n.Name})
		}
		log.Printf("Distributed RAG enabled with %d remote nodes", len(cfg.DistributedRAGNodes))
	}
	pluginManager := tools.NewPluginManager(filepath.Join(cfg.DataDir, "plugins"))
	if scanErr := pluginManager.ScanPlugins(); scanErr != nil {
		log.Printf("Warning: failed to scan plugins: %v", scanErr)
	} else if count := len(pluginManager.ListPlugins()); count > 0 {
		log.Printf("Loaded %d plugins", count)
	}
	var p2pDiscovery *p2p.Discovery
	var p2pTransfer *p2p.TransferManager
	var p2pIdentity *p2p.Identity
	var p2pTrust *p2p.TrustStore
	if cfg.EnableP2P {
		identity, identityErr := p2p.LoadOrCreateIdentity(filepath.Join(cfg.DataDir, "p2p", "identity.key"))
		trustStore, trustErr := p2p.NewTrustStore(filepath.Join(cfg.DataDir, "p2p", "trusted-peers.json"))
		if identityErr != nil || trustErr != nil {
			log.Printf("Warning: secure P2P initialization failed (identity=%v trust=%v)", identityErr, trustErr)
		} else {
			p2pDiscovery = p2p.NewDiscovery(cfg.P2PPort, cfg.DiscoveryPort)
			p2pDiscovery.SetIdentity(identity, trustStore)
			p2pTransfer = p2p.NewTrustedTransferManager(cfg.P2PPort, cfg.ModelsDir, identity, trustStore)
			p2pIdentity, p2pTrust = identity, trustStore
			c.owned.add("P2P discovery", func() error { p2pDiscovery.Stop(); return nil })
		}
	}
	if err := c.initModelCache(cfg); err != nil {
		return nil, err
	}
	// Shutdown is the reverse of this ordering. Cancel background work, then
	// close provider admission, then let that scheduler retire the legacy cache.
	// During earlier construction failures these owners retain their original
	// rollback entries; promotion occurs immediately after cache acquisition.
	if err := c.owned.promote("provider scheduler"); err != nil {
		return nil, err
	}
	if err := c.owned.promote("runtime context"); err != nil {
		return nil, err
	}
	version := runtimeVersion()
	browserHub := computer.NewBrowserHub()
	deps := server.Dependencies{
		Context: runtimeCtx, Version: version, Workspace: c.workspace,
		Registry: c.registry, Engine: c.engine, EmbeddingEngine: c.embedding, RAGEngine: ragEngine,
		Monitor: c.monitor, SystemResources: c.resources, StatsTracker: c.stats, ResponseCache: c.cache,
		ModelCache: c.modelCache, LlamaProcess: c.llamaProcess,
		InferenceLifecycle: inference.NewLifecycleGate(cfg.InferenceSlots),
		ProviderScheduler:  c.providerScheduler, ProviderRegistry: c.providerRegistry,
		RateLimiter: rateLimiter, InferenceRateLimiter: inferenceRateLimiter,
		SessionHandlers: sessionHandlers, AuthMiddleware: authMiddleware, UserStore: userStore,
		QuotaManager: quotaManager, KBManager: kbManager, LoRAManager: loraManager,
		AgentManager: agentManager, ToolRegistry: toolRegistry, MCPHandler: toolRegistry.MCPHTTPHandler(version),
		CapabilityBroker: capabilityBroker, RunLog: runLog, ArtifactStore: artifactStore,
		BrowserHub: browserHub, IntegrationRegistry: integrationRegistry,
		P2PDiscovery: p2pDiscovery, P2PTransfer: p2pTransfer, P2PIdentity: p2pIdentity, P2PTrust: p2pTrust,
		Metrics: offgridMetrics, WebSocketHub: wsHub, PowerManager: powerManager, Degradation: degradationMgr,
		AuditLogger: auditLogger, LDAPAuth: ldapAuth, MCPMarketplace: mcpMarketplace,
		LoadBalancer: loadBalancer, DistributedRAG: distributedRAG, PluginManager: pluginManager,
	}
	transport, err = server.NewTransport(server.HTTPConfig{Runtime: cfg}, deps)
	if err != nil {
		return nil, err
	}
	hooks := transport.AgentHooks()
	runner := agents.NewRunner(agentManager, hooks.Tools, hooks.Call)
	runner.StreamCaller = hooks.Stream
	runner.ContextWindow = hooks.ContextWindow
	runner.CountContext = hooks.CountContext
	runner.Observer = hooks.Observe
	if err := transport.BindApplicationRuntime(runner); err != nil {
		return nil, err
	}
	runner.StartCoordinator(runtimeCtx)
	return &Runtime{transport: transport, owned: c.owned}, nil
}

func runtimeVersion() string {
	if version := strings.TrimSpace(os.Getenv("OFFGRID_VERSION")); version != "" {
		return version
	}
	if raw, err := os.ReadFile("VERSION"); err == nil {
		if version := strings.TrimSpace(string(raw)); version != "" {
			return version
		}
	}
	return "dev"
}
