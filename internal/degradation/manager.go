// Package degradation provides graceful degradation under resource pressure.
// For edge devices with limited resources, this ensures the system remains
// responsive even when approaching resource limits.
package degradation

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// DegradationLevel represents the current system degradation state
type DegradationLevel int32

const (
	LevelNormal    DegradationLevel = 0 // Full functionality
	LevelReduced   DegradationLevel = 1 // Some features disabled
	LevelMinimal   DegradationLevel = 2 // Essential features only
	LevelEmergency DegradationLevel = 3 // Survival mode
)

func (l DegradationLevel) String() string {
	switch l {
	case LevelNormal:
		return "normal"
	case LevelReduced:
		return "reduced"
	case LevelMinimal:
		return "minimal"
	case LevelEmergency:
		return "emergency"
	default:
		return "unknown"
	}
}

// ResourceMetrics contains current resource usage
type ResourceMetrics struct {
	MemoryUsedMB   int64   `json:"memory_used_mb"`
	MemoryTotalMB  int64   `json:"memory_total_mb"`
	MemoryPercent  float64 `json:"memory_percent"`
	CPUPercent     float64 `json:"cpu_percent"`
	DiskUsedMB     int64   `json:"disk_used_mb"`
	DiskFreeMB     int64   `json:"disk_free_mb"`
	DiskPercent    float64 `json:"disk_percent"`
	ActiveRequests int64   `json:"active_requests"`
	QueuedRequests int64   `json:"queued_requests"`
	Goroutines     int     `json:"goroutines"`
}

// DegradationConfig configures degradation thresholds
type DegradationConfig struct {
	// Memory thresholds (percent)
	MemoryReducedThreshold   float64 `json:"memory_reduced_threshold"`
	MemoryMinimalThreshold   float64 `json:"memory_minimal_threshold"`
	MemoryEmergencyThreshold float64 `json:"memory_emergency_threshold"`

	// CPU thresholds (percent)
	CPUReducedThreshold float64 `json:"cpu_reduced_threshold"`
	CPUMinimalThreshold float64 `json:"cpu_minimal_threshold"`

	// Request queue thresholds
	QueueReducedThreshold   int `json:"queue_reduced_threshold"`
	QueueMinimalThreshold   int `json:"queue_minimal_threshold"`
	QueueEmergencyThreshold int `json:"queue_emergency_threshold"`

	// Recovery settings
	RecoveryDelay time.Duration `json:"recovery_delay"`
	CheckInterval time.Duration `json:"check_interval"`
}

// DefaultConfig returns sensible defaults for edge devices
func DefaultConfig() DegradationConfig {
	return DegradationConfig{
		MemoryReducedThreshold:   70.0,
		MemoryMinimalThreshold:   85.0,
		MemoryEmergencyThreshold: 95.0,

		CPUReducedThreshold: 80.0,
		CPUMinimalThreshold: 95.0,

		QueueReducedThreshold:   10,
		QueueMinimalThreshold:   25,
		QueueEmergencyThreshold: 50,

		RecoveryDelay: 30 * time.Second,
		CheckInterval: 5 * time.Second,
	}
}

// Manager manages graceful degradation
type Manager struct {
	mu              sync.RWMutex
	config          DegradationConfig
	currentLevel    int32 // atomic
	metrics         ResourceMetrics
	lastLevelChange time.Time
	callbacks       []func(DegradationLevel)
	ctx             context.Context
	cancel          context.CancelFunc
	activeRequests  int64 // atomic
	queuedRequests  int64 // atomic
}

// NewManager creates a new degradation manager
func NewManager(config DegradationConfig) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		config:          config,
		lastLevelChange: time.Now(),
		ctx:             ctx,
		cancel:          cancel,
	}
}

// Start starts the degradation monitor
func (m *Manager) Start() {
	go m.monitorLoop()
}

// Stop stops the degradation monitor
func (m *Manager) Stop() {
	m.cancel()
}

// CurrentLevel returns the current degradation level
func (m *Manager) CurrentLevel() DegradationLevel {
	return DegradationLevel(atomic.LoadInt32(&m.currentLevel))
}

// Metrics returns the current resource metrics
func (m *Manager) Metrics() ResourceMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.metrics
}

// OnLevelChange registers a callback for level changes
func (m *Manager) OnLevelChange(callback func(DegradationLevel)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callbacks = append(m.callbacks, callback)
}

// RequestStart records a request start
func (m *Manager) RequestStart() bool {
	level := m.CurrentLevel()

	// In emergency mode, reject new requests
	if level == LevelEmergency {
		return false
	}

	atomic.AddInt64(&m.activeRequests, 1)
	return true
}

// RequestEnd records a request end
func (m *Manager) RequestEnd() {
	atomic.AddInt64(&m.activeRequests, -1)
}

// QueueRequest records a queued request
func (m *Manager) QueueRequest() {
	atomic.AddInt64(&m.queuedRequests, 1)
}

// DequeueRequest records a dequeued request
func (m *Manager) DequeueRequest() {
	atomic.AddInt64(&m.queuedRequests, -1)
}

// ShouldLimit returns true if new requests should be limited
func (m *Manager) ShouldLimit() bool {
	return m.CurrentLevel() >= LevelReduced
}

// ShouldCache returns true if caching should be used aggressively
func (m *Manager) ShouldCache() bool {
	return m.CurrentLevel() >= LevelReduced
}

// ShouldSkipEmbeddings returns true if embeddings should be skipped
func (m *Manager) ShouldSkipEmbeddings() bool {
	return m.CurrentLevel() >= LevelMinimal
}

// ShouldSkipRAG returns true if RAG should be disabled
func (m *Manager) ShouldSkipRAG() bool {
	return m.CurrentLevel() >= LevelMinimal
}

// MaxConcurrentRequests returns the max concurrent requests for current level
func (m *Manager) MaxConcurrentRequests() int {
	switch m.CurrentLevel() {
	case LevelNormal:
		return 10
	case LevelReduced:
		return 5
	case LevelMinimal:
		return 2
	case LevelEmergency:
		return 1
	default:
		return 10
	}
}

// MaxContextSize returns the max context size for current level
func (m *Manager) MaxContextSize() int {
	switch m.CurrentLevel() {
	case LevelNormal:
		return 8192
	case LevelReduced:
		return 4096
	case LevelMinimal:
		return 2048
	case LevelEmergency:
		return 512
	default:
		return 8192
	}
}

// MaxResponseTokens returns the max response tokens for current level
func (m *Manager) MaxResponseTokens() int {
	switch m.CurrentLevel() {
	case LevelNormal:
		return 2048
	case LevelReduced:
		return 1024
	case LevelMinimal:
		return 512
	case LevelEmergency:
		return 256
	default:
		return 2048
	}
}

// monitorLoop continuously monitors resources
func (m *Manager) monitorLoop() {
	ticker := time.NewTicker(m.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkResources()
		}
	}
}

// checkResources checks current resources and updates degradation level
func (m *Manager) checkResources() {
	metrics := m.collectMetrics()

	m.mu.Lock()
	m.metrics = metrics
	m.mu.Unlock()

	// Calculate target level based on metrics
	targetLevel := m.calculateLevel(metrics)

	// Apply recovery delay (don't recover too quickly)
	currentLevel := m.CurrentLevel()
	if targetLevel < currentLevel {
		if time.Since(m.lastLevelChange) < m.config.RecoveryDelay {
			return // Don't recover yet
		}
	}

	// Update level if changed
	if targetLevel != currentLevel {
		atomic.StoreInt32(&m.currentLevel, int32(targetLevel))
		m.lastLevelChange = time.Now()
		m.notifyCallbacks(targetLevel)
	}
}

// collectMetrics collects current resource metrics
func (m *Manager) collectMetrics() ResourceMetrics {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	metrics := ResourceMetrics{
		MemoryUsedMB:   int64(memStats.Alloc / 1024 / 1024),
		MemoryTotalMB:  int64(memStats.Sys / 1024 / 1024),
		ActiveRequests: atomic.LoadInt64(&m.activeRequests),
		QueuedRequests: atomic.LoadInt64(&m.queuedRequests),
		Goroutines:     runtime.NumGoroutine(),
	}

	if metrics.MemoryTotalMB > 0 {
		metrics.MemoryPercent = float64(metrics.MemoryUsedMB) / float64(metrics.MemoryTotalMB) * 100
	}

	return metrics
}

// calculateLevel calculates target degradation level based on metrics
func (m *Manager) calculateLevel(metrics ResourceMetrics) DegradationLevel {
	level := LevelNormal

	// Check memory
	if metrics.MemoryPercent >= m.config.MemoryEmergencyThreshold {
		return LevelEmergency
	}
	if metrics.MemoryPercent >= m.config.MemoryMinimalThreshold {
		if level < LevelMinimal {
			level = LevelMinimal
		}
	} else if metrics.MemoryPercent >= m.config.MemoryReducedThreshold {
		if level < LevelReduced {
			level = LevelReduced
		}
	}

	// Check queue
	queueLen := int(metrics.QueuedRequests)
	if queueLen >= m.config.QueueEmergencyThreshold {
		return LevelEmergency
	}
	if queueLen >= m.config.QueueMinimalThreshold {
		if level < LevelMinimal {
			level = LevelMinimal
		}
	} else if queueLen >= m.config.QueueReducedThreshold {
		if level < LevelReduced {
			level = LevelReduced
		}
	}

	return level
}

// notifyCallbacks notifies all registered callbacks
func (m *Manager) notifyCallbacks(level DegradationLevel) {
	m.mu.RLock()
	callbacks := make([]func(DegradationLevel), len(m.callbacks))
	copy(callbacks, m.callbacks)
	m.mu.RUnlock()

	for _, cb := range callbacks {
		go cb(level)
	}
}

// Status returns a human-readable status
func (m *Manager) Status() map[string]interface{} {
	level := m.CurrentLevel()
	metrics := m.Metrics()

	return map[string]interface{}{
		"level":            level.String(),
		"level_code":       int(level),
		"memory_used_mb":   metrics.MemoryUsedMB,
		"memory_total_mb":  metrics.MemoryTotalMB,
		"memory_percent":   metrics.MemoryPercent,
		"active_requests":  metrics.ActiveRequests,
		"queued_requests":  metrics.QueuedRequests,
		"goroutines":       metrics.Goroutines,
		"max_concurrent":   m.MaxConcurrentRequests(),
		"max_context":      m.MaxContextSize(),
		"max_tokens":       m.MaxResponseTokens(),
		"cache_aggressive": m.ShouldCache(),
		"rag_enabled":      !m.ShouldSkipRAG(),
	}
}

// RateLimiter provides adaptive rate limiting based on degradation level
type RateLimiter struct {
	manager  *Manager
	mu       sync.Mutex
	tokens   float64
	lastTime time.Time
}

// NewRateLimiter creates a rate limiter that adapts to degradation level
func NewRateLimiter(manager *Manager) *RateLimiter {
	return &RateLimiter{
		manager:  manager,
		tokens:   10,
		lastTime: time.Now(),
	}
}

// Allow returns true if a request should be allowed
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(rl.lastTime).Seconds()
	rl.lastTime = now

	// Refill rate depends on degradation level
	refillRate := rl.getRefillRate()
	maxTokens := rl.getMaxTokens()

	rl.tokens += elapsed * refillRate
	if rl.tokens > maxTokens {
		rl.tokens = maxTokens
	}

	if rl.tokens >= 1 {
		rl.tokens--
		return true
	}

	return false
}

func (rl *RateLimiter) getRefillRate() float64 {
	switch rl.manager.CurrentLevel() {
	case LevelNormal:
		return 10.0 // 10 tokens/sec
	case LevelReduced:
		return 5.0
	case LevelMinimal:
		return 2.0
	case LevelEmergency:
		return 0.5
	default:
		return 10.0
	}
}

func (rl *RateLimiter) getMaxTokens() float64 {
	switch rl.manager.CurrentLevel() {
	case LevelNormal:
		return 20.0
	case LevelReduced:
		return 10.0
	case LevelMinimal:
		return 5.0
	case LevelEmergency:
		return 2.0
	default:
		return 20.0
	}
}
