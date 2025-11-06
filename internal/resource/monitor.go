package resource

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

// Monitor tracks system resource usage
type Monitor struct {
	mu             sync.RWMutex
	stats          Stats
	updateInterval time.Duration
	stopChan       chan struct{}
}

// Stats represents system resource statistics
type Stats struct {
	CPUUsagePercent    float64   `json:"cpu_usage_percent"`
	MemoryUsedMB       uint64    `json:"memory_used_mb"`
	MemoryTotalMB      uint64    `json:"memory_total_mb"`
	MemoryUsagePercent float64   `json:"memory_usage_percent"`
	DiskUsedGB         uint64    `json:"disk_used_gb"`
	DiskTotalGB        uint64    `json:"disk_total_gb"`
	DiskUsagePercent   float64   `json:"disk_usage_percent"`
	NumGoroutines      int       `json:"num_goroutines"`
	LastUpdated        time.Time `json:"last_updated"`
}

// NewMonitor creates a new resource monitor
func NewMonitor(updateInterval time.Duration) *Monitor {
	return &Monitor{
		updateInterval: updateInterval,
		stopChan:       make(chan struct{}),
	}
}

// Start begins monitoring system resources
func (m *Monitor) Start() {
	go m.monitorLoop()
}

// Stop stops the resource monitor
func (m *Monitor) Stop() {
	close(m.stopChan)
}

// GetStats returns the current resource statistics
func (m *Monitor) GetStats() Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}

// monitorLoop continuously updates resource statistics
func (m *Monitor) monitorLoop() {
	ticker := time.NewTicker(m.updateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.updateStats()
		case <-m.stopChan:
			return
		}
	}
}

// updateStats updates the current resource statistics
func (m *Monitor) updateStats() {
	m.mu.Lock()
	defer m.mu.Unlock()

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	m.stats = Stats{
		CPUUsagePercent:    m.getCPUUsage(),
		MemoryUsedMB:       memStats.Alloc / 1024 / 1024,
		MemoryTotalMB:      memStats.Sys / 1024 / 1024,
		MemoryUsagePercent: float64(memStats.Alloc) / float64(memStats.Sys) * 100,
		DiskUsedGB:         0, // TODO: Implement disk usage
		DiskTotalGB:        0, // TODO: Implement disk usage
		DiskUsagePercent:   0, // TODO: Implement disk usage
		NumGoroutines:      runtime.NumGoroutine(),
		LastUpdated:        time.Now(),
	}
}

// getCPUUsage returns CPU usage percentage
// This is a simplified version - in production, use a library like gopsutil
func (m *Monitor) getCPUUsage() float64 {
	// TODO: Implement actual CPU usage tracking
	// For now, return number of CPUs as a placeholder
	return float64(runtime.NumCPU())
}

// CheckAvailableMemory checks if there's enough memory for a model
func (m *Monitor) CheckAvailableMemory(requiredMB uint64) (bool, error) {
	stats := m.GetStats()
	availableMB := stats.MemoryTotalMB - stats.MemoryUsedMB

	if availableMB < requiredMB {
		return false, fmt.Errorf("insufficient memory: need %d MB, have %d MB available",
			requiredMB, availableMB)
	}

	return true, nil
}

// GetMemoryInfo returns detailed memory information
func GetMemoryInfo() MemoryInfo {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return MemoryInfo{
		AllocMB:      m.Alloc / 1024 / 1024,
		TotalAllocMB: m.TotalAlloc / 1024 / 1024,
		SysMB:        m.Sys / 1024 / 1024,
		NumGC:        m.NumGC,
	}
}

// MemoryInfo contains detailed memory information
type MemoryInfo struct {
	AllocMB      uint64 `json:"alloc_mb"`
	TotalAllocMB uint64 `json:"total_alloc_mb"`
	SysMB        uint64 `json:"sys_mb"`
	NumGC        uint32 `json:"num_gc"`
}

// EstimateModelMemory estimates memory requirements for a model
func EstimateModelMemory(modelSizeBytes int64, quantization string) uint64 {
	// Base memory is the model size
	baseMB := uint64(modelSizeBytes / 1024 / 1024)

	// Add overhead based on quantization
	// These are rough estimates
	var overhead float64
	switch quantization {
	case "Q4_0", "Q4_1":
		overhead = 1.2 // 20% overhead
	case "Q5_0", "Q5_1":
		overhead = 1.3
	case "Q8_0":
		overhead = 1.4
	case "F16":
		overhead = 1.5
	case "F32":
		overhead = 1.8
	default:
		overhead = 1.3 // Default to moderate overhead
	}

	return uint64(float64(baseMB) * overhead)
}
