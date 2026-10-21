package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
)

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Get current resource usage
	stats := s.monitor.GetStats()

	// Get model count
	s.syncRegistryLoadState()
	models := s.registry.ListModels()

	// Calculate uptime
	uptime := time.Since(s.startTime)
	lifecycleStatus := inference.LifecycleStatus{Capacity: s.config.InferenceSlots}
	if s.inferenceLifecycle != nil {
		lifecycleStatus = s.inferenceLifecycle.Status()
	}
	uptimeStr := formatDuration(uptime)

	// Build detailed health response
	health := map[string]interface{}{
		"status":         "healthy",
		"version":        s.version,
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
		"power": map[string]interface{}{
			"state":       string(s.powerManager.Status().State),
			"battery":     s.powerManager.Status().BatteryPercent,
			"mode":        s.getPowerMode(),
			"max_context": s.powerManager.GetMaxContext(),
		},
		"models": map[string]interface{}{
			"available": len(models),
			"loaded":    s.registry.CountLoadedModels(),
		},
		"inference": lifecycleStatus,
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
