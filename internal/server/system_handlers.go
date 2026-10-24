package server

import (
	"encoding/json"
	"log"
	"net/http"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/metrics"
	"github.com/takuphilchan/offgrid-llm/internal/platform"
	"github.com/takuphilchan/offgrid-llm/internal/power"
)

// handlePower returns current power/battery status
func (s *Server) handlePower(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	status := s.powerManager.Status()

	response := map[string]interface{}{
		"state":              string(status.State),
		"battery_level":      string(status.BatteryLevel),
		"battery_percent":    status.BatteryPercent,
		"is_charging":        status.IsCharging,
		"time_remaining":     status.TimeRemaining,
		"power_save":         status.PowerSaveActive,
		"max_context":        s.powerManager.GetMaxContext(),
		"max_concurrent":     s.powerManager.GetMaxConcurrent(),
		"embeddings_enabled": s.powerManager.ShouldEnableEmbeddings(),
		"mode":               s.getPowerMode(),
	}

	json.NewEncoder(w).Encode(response)
}

// getPowerMode returns a human-readable power mode string
func (s *Server) getPowerMode() string {
	status := s.powerManager.Status()

	if status.State == power.PowerUnknown {
		return "unknown"
	}
	if status.PowerSaveActive {
		return "saver"
	}
	return "full"
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
	lifecycleStatus := inference.LifecycleStatus{Capacity: s.config.InferenceSlots}
	if s.inferenceLifecycle != nil {
		lifecycleStatus = s.inferenceLifecycle.Status()
	}

	response := map[string]interface{}{
		"server": map[string]interface{}{
			"uptime":         uptime.String(),
			"uptime_seconds": int64(uptime.Seconds()),
			"start_time":     s.startTime.Format(time.RFC3339),
			"version":        s.version,
			"current_model":  s.currentModelID,
		},
		"inference": map[string]interface{}{
			"models":    allStats,
			"aggregate": s.getAggregateStats(),
			"lifecycle": lifecycleStatus,
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
		"version":         s.version,
		"inference_slots": s.config.InferenceSlots,
		"features": map[string]bool{
			"users":   s.config.MultiUserMode,
			"metrics": true, // Always available but can be hidden
			"agent":   true,
			"lora":    true,
		},
	})
}
