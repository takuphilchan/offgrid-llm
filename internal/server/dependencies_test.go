package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/config"
)

func transportDependenciesFromLegacy(s *Server) Dependencies {
	return Dependencies{
		Context:              context.Background(),
		Version:              s.version,
		Workspace:            s.workspaceID,
		Registry:             s.registry,
		Engine:               s.engine,
		EmbeddingEngine:      s.embeddingEngine,
		RAGEngine:            s.ragEngine,
		Monitor:              s.monitor,
		SystemResources:      s.systemResources,
		StatsTracker:         s.statsTracker,
		ResponseCache:        s.cache,
		ModelCache:           s.modelCache,
		LlamaProcess:         s.llamaProcess,
		InferenceLifecycle:   s.inferenceLifecycle,
		ProviderScheduler:    s.providerScheduler,
		ProviderRegistry:     s.providerRegistry,
		RateLimiter:          s.rateLimiter,
		InferenceRateLimiter: s.inferenceRateLimiter,
		SessionHandlers:      s.sessionHandlers,
		AuthMiddleware:       s.authMiddleware,
		UserStore:            s.userStore,
		QuotaManager:         s.quotaManager,
		KBManager:            s.kbManager,
		LoRAManager:          s.loraManager,
		AgentManager:         s.agentManager,
		ToolRegistry:         s.toolRegistry,
		MCPHandler:           s.mcpHandler,
		CapabilityBroker:     s.capabilityBroker,
		RunLog:               s.runLog,
		ArtifactStore:        s.artifactStore,
		BrowserHub:           s.browserHub,
		IntegrationRegistry:  s.integrationRegistry,
		P2PDiscovery:         s.p2pDiscovery,
		P2PTransfer:          s.p2pTransfer,
		P2PIdentity:          s.p2pIdentity,
		P2PTrust:             s.p2pTrust,
		Metrics:              s.offgridMetrics,
		WebSocketHub:         s.wsHub,
		PowerManager:         s.powerManager,
		Degradation:          s.degradationMgr,
		AuditLogger:          s.auditLogger,
		LDAPAuth:             s.ldapAuth,
		MCPMarketplace:       s.mcpMarketplace,
		LoadBalancer:         s.loadBalancer,
		DistributedRAG:       s.distributedRAG,
		PluginManager:        s.pluginManager,
	}
}

func TestTransportDependenciesValidateBeforeAssembly(t *testing.T) {
	if _, err := NewTransport(HTTPConfig{}, Dependencies{}); err == nil || !strings.Contains(err.Error(), "HTTP configuration") {
		t.Fatalf("missing config error = %v", err)
	}
	if _, err := NewTransport(HTTPConfig{Runtime: &config.Config{}}, Dependencies{}); err == nil || !strings.Contains(err.Error(), "runtime context") {
		t.Fatalf("missing context error = %v", err)
	}
	s := newTestServer(t)
	base := transportDependenciesFromLegacy(s)
	for _, tc := range []struct {
		name string
		edit func(*Dependencies)
		want string
	}{
		{name: "model", edit: func(d *Dependencies) { d.Registry = nil }, want: "model and resource"},
		{name: "enforcement", edit: func(d *Dependencies) { d.AuthMiddleware = nil }, want: "HTTP enforcement"},
		{name: "identity", edit: func(d *Dependencies) { d.UserStore = nil }, want: "session, identity"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deps := base
			tc.edit(&deps)
			if _, err := NewTransport(HTTPConfig{Runtime: s.config}, deps); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("dependency error = %v", err)
			}
		})
	}
	transport, err := NewTransport(HTTPConfig{Runtime: s.config}, base)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(transport.runtimeCancel)
	if transport.registry != s.registry {
		t.Fatal("transport did not borrow the expected dependencies")
	}
	w := httptest.NewRecorder()
	transport.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("borrowed transport health = %d: %s", w.Code, w.Body.String())
	}
}
