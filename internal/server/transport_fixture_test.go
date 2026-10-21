package server

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/artifacts"
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
	"github.com/takuphilchan/offgrid-llm/internal/power"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"github.com/takuphilchan/offgrid-llm/internal/resource"
	"github.com/takuphilchan/offgrid-llm/internal/runs"
	"github.com/takuphilchan/offgrid-llm/internal/stats"
	"github.com/takuphilchan/offgrid-llm/internal/storage"
	"github.com/takuphilchan/offgrid-llm/internal/tools"
	"github.com/takuphilchan/offgrid-llm/internal/users"
	"github.com/takuphilchan/offgrid-llm/internal/websocket"
)

// newTestServer is an explicit, disposable HTTP fixture. It does not invoke
// production application composition or a second server constructor.
func newTestServer(t *testing.T) *Server {
	t.Helper()
	cfg := config.LoadConfig()
	root := t.TempDir()
	cfg.DataDir = filepath.Join(root, "data")
	cfg.ModelsDir = filepath.Join(root, "models")
	cfg.UseMockEngine = true
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	owner, err := storage.AcquireOwnership(cfg.DataDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := owner.Close(); err != nil {
			t.Errorf("close fixture workspace: %v", err)
		}
	})
	if err := storage.PrepareDataLayout(cfg.ModelsDir, cfg.DataDir); err != nil {
		t.Fatal(err)
	}
	workspace, err := owner.WorkspaceID()
	if err != nil {
		t.Fatal(err)
	}
	registry := models.NewRegistry(cfg.ModelsDir)
	engine := inference.NewMockEngine()
	embedding := inference.NewEmbeddingEngine(filepath.Join(cfg.ModelsDir, "..", "bin"))
	monitor := resource.NewMonitor(5 * time.Second)
	tracker := stats.NewTracker()
	responseCache := cache.NewResponseCache(1000, time.Hour)
	providerRegistry := inference.NewProviderRegistry()
	providerScheduler, err := inference.NewProviderScheduler(providerRegistry, cfg.InferenceSlots)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = providerScheduler.Close() })
	if err := providerScheduler.AttachEmbeddingEngine(embedding); err != nil {
		t.Fatal(err)
	}
	resources, _ := resource.DetectResources()
	modelCache := inference.NewConfiguredModelCache(cfg, resources)
	llamaProcess, err := providerScheduler.AttachLlamaCache(modelCache, engine)
	if err != nil {
		modelCache.StopMonitor()
		modelCache.UnloadAll()
		t.Fatal(err)
	}
	ragEngine := rag.NewEngine(embedding, cfg.DataDir)
	t.Cleanup(func() { _ = ragEngine.Close() })
	sessions := NewSessionHandlers(filepath.Join(cfg.DataDir, "sessions"))
	t.Cleanup(func() { _ = sessions.Close() })
	userStore := users.NewUserStore(cfg.DataDir)
	auth := users.NewMiddleware(userStore)
	auth.SetRequireAuth(cfg.RequireAuth)
	auth.SetGuestEnabled(cfg.GuestAccess)
	for _, path := range []string{"/health", "/ready", "/livez", "/readyz", "/metrics", "/openapi.yaml"} {
		auth.AddBypassPath(path)
	}
	sandbox := agents.NewLazySandbox("python:3.10-slim")
	t.Cleanup(func() { _ = sandbox.Cleanup() })
	manager := agents.NewManagerWithPersistence(nil, agents.NewSandboxedExecutor(sandbox), nil, cfg.DataDir)
	toolRegistry := agents.NewToolRegistry()
	broker := capabilities.NewBroker(capabilities.DefaultPolicy{})
	toolRegistry.SetCapabilityBroker(broker)
	runLog, err := runs.NewLog(filepath.Join(cfg.DataDir, "runs", "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	artifactStore, err := artifacts.NewStore(filepath.Join(cfg.DataDir, "artifacts"))
	if err != nil {
		t.Fatal(err)
	}
	deps := Dependencies{
		Context: context.Background(), Version: "test", Workspace: workspace,
		Registry: registry, Engine: engine, EmbeddingEngine: embedding, RAGEngine: ragEngine,
		Monitor: monitor, SystemResources: resources, StatsTracker: tracker, ResponseCache: responseCache,
		ModelCache: modelCache, LlamaProcess: llamaProcess,
		InferenceLifecycle: inference.NewLifecycleGate(cfg.InferenceSlots), ProviderScheduler: providerScheduler,
		ProviderRegistry: providerRegistry, RateLimiter: NewRateLimiter(600, time.Minute, 60),
		InferenceRateLimiter: NewInferenceRateLimiter(10, 20), SessionHandlers: sessions,
		AuthMiddleware: auth, UserStore: userStore, QuotaManager: users.NewQuotaManager(cfg.DataDir),
		KBManager: users.NewKnowledgeBaseManager(cfg.DataDir), LoRAManager: inference.NewLoRAManager(cfg.DataDir, engine),
		AgentManager: manager, ToolRegistry: toolRegistry, MCPHandler: toolRegistry.MCPHTTPHandler("test"),
		CapabilityBroker: broker, RunLog: runLog, ArtifactStore: artifactStore,
		BrowserHub: computer.NewBrowserHub(), IntegrationRegistry: integrations.NewDefaultRegistry(),
		Metrics: metrics.NewOffGridMetrics(), WebSocketHub: websocket.NewHub(),
		PowerManager: power.NewPowerManager(power.DefaultPolicy()), Degradation: degradation.NewManager(degradation.DefaultConfig()),
		MCPMarketplace: mcp.NewMarketplace(cfg.DataDir), PluginManager: tools.NewPluginManager(filepath.Join(cfg.DataDir, "plugins")),
	}
	s, err := NewTransport(HTTPConfig{Runtime: cfg}, deps)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Errorf("close fixture transport: %v", err)
		}
	})
	hooks := s.AgentHooks()
	runner := agents.NewRunner(manager, hooks.Tools, hooks.Call)
	runner.StreamCaller, runner.ContextWindow, runner.CountContext, runner.Observer = hooks.Stream, hooks.ContextWindow, hooks.CountContext, hooks.Observe
	if err := s.BindApplicationRuntime(runner); err != nil {
		t.Fatal(err)
	}
	runner.StartCoordinator(s.runtimeCtx)
	return s
}
