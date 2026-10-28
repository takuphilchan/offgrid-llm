package server

import (
	"net/http"

	"github.com/takuphilchan/offgrid-llm/internal/users"
)

type routeGuards struct {
	adminOnly        func(http.HandlerFunc) http.HandlerFunc
	modelManagerOnly func(http.HandlerFunc) http.HandlerFunc
	ragManagerOnly   func(http.HandlerFunc) http.HandlerFunc
	chatOnly         func(http.HandlerFunc) http.HandlerFunc
	modelsOnly       func(http.HandlerFunc) http.HandlerFunc
	ragOnly          func(http.HandlerFunc) http.HandlerFunc
	statsOnly        func(http.HandlerFunc) http.HandlerFunc
	sessionsOnly     func(http.HandlerFunc) http.HandlerFunc
}

// buildRouter retains the frozen registration and middleware order.
func (s *Server) buildRouter() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/system", s.handleSystemIdentity)
	adminOnly := func(handler http.HandlerFunc) http.HandlerFunc {
		return s.requirePermissionWhenAuthEnabled(users.PermissionAdmin, handler)
	}
	modelManagerOnly := func(handler http.HandlerFunc) http.HandlerFunc {
		return s.requirePermissionWhenAuthEnabled(users.PermissionModelsManage, handler)
	}
	ragManagerOnly := func(handler http.HandlerFunc) http.HandlerFunc {
		return s.requirePermissionWhenAuthEnabled(users.PermissionRAGManage, handler)
	}
	chatOnly := func(handler http.HandlerFunc) http.HandlerFunc {
		return s.requirePermissionWhenAuthEnabled(users.PermissionChat, handler)
	}
	modelsOnly := func(handler http.HandlerFunc) http.HandlerFunc {
		return s.requirePermissionWhenAuthEnabled(users.PermissionModels, handler)
	}
	ragOnly := func(handler http.HandlerFunc) http.HandlerFunc {
		return s.requirePermissionWhenAuthEnabled(users.PermissionRAG, handler)
	}
	statsOnly := func(handler http.HandlerFunc) http.HandlerFunc {
		return s.requirePermissionWhenAuthEnabled(users.PermissionStats, handler)
	}
	sessionsOnly := func(handler http.HandlerFunc) http.HandlerFunc {
		return s.requirePermissionWhenAuthEnabled(users.PermissionSessions, handler)
	}
	guards := routeGuards{
		adminOnly:        adminOnly,
		modelManagerOnly: modelManagerOnly,
		ragManagerOnly:   ragManagerOnly,
		chatOnly:         chatOnly,
		modelsOnly:       modelsOnly,
		ragOnly:          ragOnly,
		statsOnly:        statsOnly,
		sessionsOnly:     sessionsOnly,
	}
	s.registerHealthRoutes(mux, guards)
	s.registerModelRoutes(mux, guards)
	s.registerInferenceRoutes(mux, guards)
	s.registerDiscoveryRoutes(mux, guards)
	s.registerLoadBalancerRoutes(mux, guards)
	s.registerDistributedRAGRoutes(mux, guards)
	s.registerTemplateRoutes(mux, guards)
	s.registerFileRoutes(mux, guards)
	s.registerKnowledgeRoutes(mux, guards)
	s.registerP2PRoutes(mux, guards)
	s.registerStatisticsRoutes(mux, guards)
	s.registerPluginRoutes(mux, guards)
	s.registerSessionRoutes(mux, guards)
	s.registerModelCacheRoutes(mux, guards)
	s.registerCacheRoutes(mux, guards)
	s.registerSystemRoutes(mux, guards)
	s.registerUserRoutes(mux, guards)
	s.registerLoRARoutes(mux, guards)
	s.registerAgentRoutes(mux, guards)
	s.registerAudioRoutes(mux, guards)
	s.registerWebSocketRoutes(mux, guards)
	s.registerLegacyUIRoutes(mux, guards)
	s.registerUIRoutes(mux, guards)

	// Build handler chain: logging -> auth -> routes
	var handler http.Handler = mux
	handler = s.authMiddleware.Wrap(handler) // Auth middleware
	handler = s.browserTransport(handler)    // Narrow token-authenticated companion channel, never agent administration.
	handler = s.executionTransport(handler)  // Exact delegated host routes only; pairing and workspace controls retain user authentication.
	handler = requestBodyLimitMiddleware(handler)
	handler = s.loggingMiddleware(handler) // Logging middleware (outermost)
	return handler
}

func (s *Server) registerHealthRoutes(mux *http.ServeMux, guards routeGuards) {
	// Health check endpoints
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/ready", s.handleReady)
	mux.HandleFunc("/livez", s.handleLiveness)   // Kubernetes-style
	mux.HandleFunc("/readyz", s.handleReadiness) // Kubernetes-style
	mux.HandleFunc("/power", s.handlePower)      // Power/battery status
	mux.HandleFunc("/openapi.yaml", handleOpenAPISpec)
}

func (s *Server) registerModelRoutes(mux *http.ServeMux, guards routeGuards) {
	modelManagerOnly := guards.modelManagerOnly
	modelsOnly := guards.modelsOnly
	// API v1 routes (OpenAI-compatible)
	mux.HandleFunc("/v1/models", modelsOnly(s.rateLimiter.Middleware(s.handleListModels)))
	mux.HandleFunc(modelPackagePath, s.rateLimiter.Middleware(s.handleModelPackages))
	mux.HandleFunc(modelPackagePath+"/", s.rateLimiter.Middleware(s.handleModelPackages))
	mux.HandleFunc("/api/v2/models", s.rateLimiter.Middleware(s.handleModelsV2))
	mux.HandleFunc("/api/v2/models/", s.rateLimiter.Middleware(s.handleModelsV2))
	mux.HandleFunc("/v1/models/delete", modelManagerOnly(s.rateLimiter.Middleware(s.handleDeleteModel)))
	mux.HandleFunc("/v1/models/download", modelManagerOnly(s.rateLimiter.Middleware(s.handleDownloadModel)))
	mux.HandleFunc("/v1/models/download/progress", modelsOnly(s.handleDownloadProgress))
	mux.HandleFunc("/v1/models/download/cancel", modelManagerOnly(s.handleCancelDownload))
	mux.HandleFunc("/v1/models/verify", modelManagerOnly(s.handleVerifyModel))
	mux.HandleFunc("/v1/integrations", modelsOnly(s.handleIntegrations))
	mux.HandleFunc("/v1/integrations/", modelsOnly(s.handleIntegration))
}

func (s *Server) registerInferenceRoutes(mux *http.ServeMux, guards routeGuards) {
	mux.HandleFunc("/v1/inference/prepare", guards.chatOnly(s.inferenceRateLimiter.Middleware(s.handleInferencePrepare)))
	chatOnly := guards.chatOnly
	// Inference endpoints with strict rate limiting
	mux.HandleFunc("/v1/chat/completions", chatOnly(s.inferenceRateLimiter.Middleware(s.handleChatCompletions)))
	mux.HandleFunc("/v1/completions", chatOnly(s.inferenceRateLimiter.Middleware(s.handleCompletions)))
	mux.HandleFunc("/v1/embeddings", chatOnly(s.inferenceRateLimiter.Middleware(s.handleEmbeddings)))
	mux.HandleFunc("/v1/responses", chatOnly(s.inferenceRateLimiter.Middleware(s.handleResponses)))
}

func (s *Server) registerDiscoveryRoutes(mux *http.ServeMux, guards routeGuards) {
	adminOnly := guards.adminOnly
	modelManagerOnly := guards.modelManagerOnly
	chatOnly := guards.chatOnly
	modelsOnly := guards.modelsOnly
	// Model search and discovery (OffGrid-specific)
	mux.HandleFunc("/v1/search", modelsOnly(s.handleModelSearch))
	mux.HandleFunc("/v1/search/files", modelsOnly(s.handleModelSearchFiles))
	mux.HandleFunc("/v1/catalog", modelsOnly(s.handleModelCatalog))
	mux.HandleFunc("/v1/benchmark", chatOnly(s.handleBenchmark))
	mux.HandleFunc("/v1/quantize", modelManagerOnly(s.handleQuantize))
	mux.HandleFunc("/v1/quantize/types", modelsOnly(s.handleQuantizeTypes))
	mux.HandleFunc("/v1/models/hotswap", modelManagerOnly(s.handleHotSwap))
	mux.HandleFunc("/v1/models/hotswap/status", modelsOnly(s.handleHotSwapStatus))
	mux.HandleFunc("/v1/models/hotswap/prepare", modelManagerOnly(s.handleHotSwapPrepare))
	mux.HandleFunc("/v1/terminal/exec", adminOnly(s.handleTerminalExec))
	mux.HandleFunc("/v1/terminal/exec/stream", adminOnly(s.handleTerminalExecStream))
}

func (s *Server) registerLoadBalancerRoutes(mux *http.ServeMux, guards routeGuards) {
	adminOnly := guards.adminOnly
	// Load balancer endpoints
	mux.HandleFunc("/v1/loadbalancer", adminOnly(s.handleLoadBalancer))
	mux.HandleFunc("/v1/loadbalancer/backends", adminOnly(s.handleLoadBalancerBackends))
	mux.HandleFunc("/v1/loadbalancer/backends/", adminOnly(s.handleLoadBalancerBackend))
}

func (s *Server) registerDistributedRAGRoutes(mux *http.ServeMux, guards routeGuards) {
	ragManagerOnly := guards.ragManagerOnly
	ragOnly := guards.ragOnly
	// Distributed RAG endpoints
	mux.HandleFunc("/v1/rag/distributed", ragOnly(s.handleDistributedRAG))
	mux.HandleFunc("/v1/rag/distributed/nodes", ragManagerOnly(s.handleDistributedRAGNodes))
	mux.HandleFunc("/v1/rag/distributed/search", ragOnly(s.handleDistributedRAGSearch))
}

func (s *Server) registerTemplateRoutes(mux *http.ServeMux, guards routeGuards) {
	adminOnly := guards.adminOnly
	// Templates endpoints
	mux.HandleFunc("/v1/templates", adminOnly(s.handleTemplates))
	mux.HandleFunc("/v1/templates/", adminOnly(s.handleTemplateDetails))
}

func (s *Server) registerFileRoutes(mux *http.ServeMux, guards routeGuards) {
	adminOnly := guards.adminOnly
	// USB import/export
	mux.HandleFunc("/v1/usb/scan", adminOnly(s.handleUSBScan))
	mux.HandleFunc("/v1/usb/import", adminOnly(s.handleUSBImport))
	mux.HandleFunc("/v1/usb/export", adminOnly(s.handleUSBExport))
	mux.HandleFunc("/v1/usb/export/progress", s.handleExportProgress)
	mux.HandleFunc("/v1/filesystem/browse", adminOnly(s.handleFilesystemBrowse))
	mux.HandleFunc("/v1/filesystem/common-paths", adminOnly(s.handleCommonPaths))
}

func (s *Server) registerKnowledgeRoutes(mux *http.ServeMux, guards routeGuards) {
	ragManagerOnly := guards.ragManagerOnly
	ragOnly := guards.ragOnly
	// RAG (Retrieval Augmented Generation) endpoints
	mux.HandleFunc("/v1/rag/status", ragOnly(s.handleRAGStatus))
	mux.HandleFunc("/v1/rag/enable", ragManagerOnly(s.handleRAGEnable))
	mux.HandleFunc("/v1/rag/disable", ragManagerOnly(s.handleRAGDisable))
	mux.HandleFunc("/v1/documents", ragOnly(s.handleDocumentsList))
	mux.HandleFunc("/v1/documents/ingest", ragManagerOnly(s.handleDocumentIngest))
	mux.HandleFunc("/v1/documents/ingest-url", ragManagerOnly(s.handleDocumentIngestURL))
	mux.HandleFunc("/v1/documents/delete", ragManagerOnly(s.handleDocumentDelete))
	mux.HandleFunc("/v1/documents/source", ragOnly(s.handleDocumentSource))
	mux.HandleFunc("/v1/documents/search", ragOnly(s.handleDocumentSearch))
	mux.HandleFunc("/v1/documents/reindex", ragManagerOnly(s.handleDocumentReindex))
	mux.HandleFunc("/v1/rag/evaluate", ragManagerOnly(s.handleRAGEvaluate))
}

func (s *Server) registerP2PRoutes(mux *http.ServeMux, guards routeGuards) {
	adminOnly := guards.adminOnly
	modelManagerOnly := guards.modelManagerOnly
	modelsOnly := guards.modelsOnly
	// P2P endpoints
	mux.HandleFunc("/v1/p2p/peers", modelsOnly(s.handleP2PPeers))
	mux.HandleFunc("/v1/p2p/download", modelManagerOnly(s.handleP2PDownload))
	mux.HandleFunc("/v1/p2p/status", modelsOnly(s.handleP2PStatus))
	mux.HandleFunc("/v1/p2p/trust", adminOnly(s.handleP2PTrust))
}

func (s *Server) registerStatisticsRoutes(mux *http.ServeMux, guards routeGuards) {
	statsOnly := guards.statsOnly
	// Statistics endpoint
	mux.HandleFunc("/stats", statsOnly(s.handleStats))
	mux.HandleFunc("/v1/stats", statsOnly(s.handleStatsV1))
	mux.HandleFunc("/v1/system/info", statsOnly(s.handleSystemInfo))
}

func (s *Server) registerPluginRoutes(mux *http.ServeMux, guards routeGuards) {
	adminOnly := guards.adminOnly
	// Plugin endpoints
	mux.HandleFunc("/v1/plugins", adminOnly(s.handlePlugins))
	mux.HandleFunc("/v1/plugins/", adminOnly(s.handlePluginAction))
}

func (s *Server) registerSessionRoutes(mux *http.ServeMux, guards routeGuards) {
	sessionsOnly := guards.sessionsOnly
	// Sessions endpoints
	mux.HandleFunc("/v1/sessions", sessionsOnly(s.sessionHandlers.HandleSessions))
	mux.HandleFunc("/v1/sessions/", sessionsOnly(s.sessionHandlers.HandleSessions))
}

func (s *Server) registerModelCacheRoutes(mux *http.ServeMux, guards routeGuards) {
	modelManagerOnly := guards.modelManagerOnly
	modelsOnly := guards.modelsOnly
	// Model cache and loading progress
	mux.HandleFunc("/v1/cache/stats", modelsOnly(s.handleCacheStats))
	mux.HandleFunc("/v1/loading/progress", modelsOnly(s.handleLoadingProgress))
	mux.HandleFunc("/v1/loading/progress/stream", modelsOnly(s.handleLoadingProgressStream))
	mux.HandleFunc("/v1/loading/prewarm", modelManagerOnly(s.handleFastPrewarm))
}

func (s *Server) registerCacheRoutes(mux *http.ServeMux, guards routeGuards) {
	modelManagerOnly := guards.modelManagerOnly
	modelsOnly := guards.modelsOnly
	// Cache management endpoints
	mux.HandleFunc("/cache/stats", modelsOnly(s.handleCacheStats))
	mux.HandleFunc("/cache/clear", modelManagerOnly(s.handleCacheClear))
}

func (s *Server) registerSystemRoutes(mux *http.ServeMux, guards routeGuards) {
	statsOnly := guards.statsOnly
	// Prometheus metrics endpoint
	mux.HandleFunc("/metrics", s.handleMetrics)
	mux.HandleFunc("/v1/system/stats", statsOnly(s.handleSystemStats))
	mux.HandleFunc("/v1/system/config", s.handleSystemConfig) // UI feature flags
}

func (s *Server) registerUserRoutes(mux *http.ServeMux, guards routeGuards) {
	// User management endpoints
	mux.HandleFunc("/v1/users/me", s.handleCurrentUser)
	mux.HandleFunc("/v1/users", s.requirePermissionWhenAuthEnabled(users.PermissionAdmin, s.handleUsers))
	mux.HandleFunc("/v1/users/", s.requirePermissionWhenAuthEnabled(users.PermissionAdmin, s.handleUsers))
	mux.HandleFunc("/v1/auth/login", s.handleLogin)
	mux.HandleFunc("/v1/auth/logout", s.handleLogout)
	mux.HandleFunc("/v1/auth/ldap/test", s.requirePermissionWhenAuthEnabled(users.PermissionAdmin, s.handleLDAPTest))
	mux.HandleFunc("/v1/auth/ldap/sync", s.requirePermissionWhenAuthEnabled(users.PermissionAdmin, s.handleLDAPSync))
	mux.HandleFunc("/v1/quota", s.handleQuota)
	mux.HandleFunc("/v1/quota/", s.requirePermissionWhenAuthEnabled(users.PermissionAdmin, s.handleQuotaManage))
}

func (s *Server) registerLoRARoutes(mux *http.ServeMux, guards routeGuards) {
	modelManagerOnly := guards.modelManagerOnly
	modelsOnly := guards.modelsOnly
	// LoRA adapter endpoints
	mux.HandleFunc("/v1/lora", modelsOnly(s.handleLoRAList))
	mux.HandleFunc("/v1/lora/", modelManagerOnly(s.handleLoRA))
}

func (s *Server) registerAgentRoutes(mux *http.ServeMux, guards routeGuards) {
	adminOnly := guards.adminOnly
	chatOnly := guards.chatOnly
	// Agent endpoints
	mux.HandleFunc("/mcp", adminOnly(s.mcpHandler.ServeHTTP))
	mux.HandleFunc("/v1/agents/run", adminOnly(s.handleAgentRun))
	mux.HandleFunc("/v1/agents/tasks", adminOnly(s.handleAgentTasks))
	mux.HandleFunc("/v1/agents/tasks/", adminOnly(s.handleAgentTaskAction))
	mux.HandleFunc("/api/v2/jobs/", chatOnly(s.profileJobAccess(s.handleJobRead)))
	mux.HandleFunc("/api/v2/jobs", chatOnly(s.handleJobSubmit))
	mux.HandleFunc("/api/v2/execution/workspaces", chatOnly(s.handleExecutionWorkspaces))
	mux.HandleFunc("/api/v2/execution/pairing", chatOnly(s.handleExecutionPairing))
	mux.HandleFunc("/api/v2/execution/revoke", chatOnly(s.handleExecutionRevoke))
	mux.HandleFunc("/api/v2/agent-profiles", chatOnly(s.handleAgentProfiles))
	mux.HandleFunc("/api/v2/agent-profiles/", chatOnly(s.handleAgentProfiles))
	mux.HandleFunc("/api/v2/agent-skills", chatOnly(s.handleAgentSkills))
	mux.HandleFunc("/api/v2/agent-skills/", chatOnly(s.handleAgentSkills))
	mux.HandleFunc("/api/v2/agent-memory/", chatOnly(s.handleAgentMemory))
	mux.HandleFunc("/api/v2/agent-schedules", chatOnly(s.handleAgentSchedules))
	mux.HandleFunc("/api/v2/agent-schedules/", chatOnly(s.handleAgentSchedules))
	mux.HandleFunc("/v1/agents/workflows", adminOnly(s.handleAgentWorkflows))
	mux.HandleFunc("/v1/agents/orchestrate", adminOnly(s.handleAgentOrchestrate))
	mux.HandleFunc("/v1/runs", adminOnly(s.handleRuns))
	mux.HandleFunc("/v1/runs/", adminOnly(s.handleRuns))
	mux.HandleFunc("/v1/artifacts/", adminOnly(s.handleArtifacts))
	mux.HandleFunc("/v1/computer/status", adminOnly(s.handleComputerStatus))
	mux.HandleFunc("/v1/computer/session", adminOnly(s.handleComputerUpgradeRequired))
	mux.HandleFunc("/v1/computer/action", adminOnly(s.handleComputerUpgradeRequired))
	mux.HandleFunc("/v1/computer/stop", adminOnly(s.handleComputerUpgradeRequired))
	mux.HandleFunc("/v1/computer/reset", adminOnly(s.handleComputerUpgradeRequired))
	mux.HandleFunc("/api/v2/computer/capabilities", adminOnly(s.handleComputerCapabilities))
	mux.HandleFunc("/api/v2/computer/status", adminOnly(s.handleComputerStatus))
	mux.HandleFunc("/api/v2/computer/stop", adminOnly(s.handleComputerStop))
	mux.HandleFunc("/api/v2/computer/pairing", adminOnly(s.handleBrowserPairing))
	mux.HandleFunc("/api/v2/computer/sessions", adminOnly(s.handleBrowserSessions))
	mux.HandleFunc("/api/v2/computer/sessions/stop", adminOnly(s.handleComputerSessionStop))
	mux.HandleFunc("/api/v2/computer/model-check", adminOnly(s.handleComputerModelCheck))
	mux.HandleFunc("/v1/agents/tools", adminOnly(s.handleAgentTools))
	mux.HandleFunc("/v1/capabilities", adminOnly(s.handleCapabilities))
	mux.HandleFunc("/v1/agents/mcp", adminOnly(s.handleAgentMCP))
	mux.HandleFunc("/v1/agents/mcp/test", adminOnly(s.handleAgentMCPTest))
	mux.HandleFunc("/v1/agents/mcp/marketplace", adminOnly(s.handleMCPMarketplace))
	mux.HandleFunc("/v1/agents/mcp/marketplace/", adminOnly(s.handleMCPMarketplaceAction))
}

func (s *Server) registerAudioRoutes(mux *http.ServeMux, guards routeGuards) {
	adminOnly := guards.adminOnly
	chatOnly := guards.chatOnly
	// Audio endpoints (OpenAI-compatible TTS/ASR)
	mux.HandleFunc("/v1/audio/transcriptions", chatOnly(s.handleAudioTranscriptions))
	mux.HandleFunc("/v1/audio/speech", chatOnly(s.handleAudioSpeech))
	mux.HandleFunc("/v1/audio/voices", chatOnly(s.handleAudioVoices))
	mux.HandleFunc("/v1/audio/whisper-models", chatOnly(s.handleAudioWhisperModels))
	mux.HandleFunc("/v1/audio/models", chatOnly(s.handleAudioModels))
	mux.HandleFunc("/v1/audio/status", chatOnly(s.handleAudioStatus))
	mux.HandleFunc("/v1/audio/download", adminOnly(s.handleAudioDownload))
	mux.HandleFunc("/v1/audio/setup/whisper", adminOnly(s.handleAudioSetupWhisper))
	mux.HandleFunc("/v1/audio/setup/piper", adminOnly(s.handleAudioSetupPiper))
}

func (s *Server) registerWebSocketRoutes(mux *http.ServeMux, guards routeGuards) {
	chatOnly := guards.chatOnly
	// WebSocket endpoint
	mux.HandleFunc("/v1/ws", chatOnly(s.rateLimiter.Middleware(s.handleWebSocket)))
}

func (s *Server) registerLegacyUIRoutes(mux *http.ServeMux, guards routeGuards) {
	modelManagerOnly := guards.modelManagerOnly
	modelsOnly := guards.modelsOnly
	// Simplified UI endpoints (no /v1 prefix for easier frontend access)
	mux.HandleFunc("/models", modelsOnly(s.handleListModels))
	mux.HandleFunc("/models/refresh", modelManagerOnly(s.handleRefreshModels))
	mux.HandleFunc("/catalog", modelsOnly(s.handleModelCatalog))
}

func (s *Server) registerUIRoutes(mux *http.ServeMux, guards routeGuards) {
	// Web UI - serve HTML/CSS/JS
	uiPath := resolveUIRoot()

	// Serve static files
	fs := http.FileServer(http.Dir(uiPath))
	mux.Handle("/ui/", http.StripPrefix("/ui/", fs))

	// Root endpoint
	mux.HandleFunc("/", s.handleRoot)
}
