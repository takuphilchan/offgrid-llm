// Package inference provides inference-related functionality including load balancing
package inference

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// BackendType represents the type of inference backend
type BackendType string

const (
	BackendLlamaServer BackendType = "llama-server"
	BackendOllama      BackendType = "ollama"
	BackendLocalAI     BackendType = "localai"
	BackendVLLM        BackendType = "vllm"
	BackendOpenAI      BackendType = "openai"
	BackendCustom      BackendType = "custom"
)

// Backend represents an inference backend server
type Backend struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	URL         string      `json:"url"`               // Base URL (e.g., http://localhost:8080)
	Type        BackendType `json:"type"`              // Type of backend
	APIKey      string      `json:"api_key,omitempty"` // Optional API key
	Models      []string    `json:"models"`            // Models available on this backend
	Weight      int         `json:"weight"`            // Weight for weighted round-robin (1-100)
	MaxRequests int         `json:"max_requests"`      // Max concurrent requests (0 = unlimited)
	Enabled     bool        `json:"enabled"`

	// Health status
	Healthy       bool      `json:"healthy"`
	LastCheck     time.Time `json:"last_check"`
	LastLatencyMS int64     `json:"last_latency_ms"`
	ErrorCount    int       `json:"error_count"`

	// Runtime stats
	ActiveRequests int64   `json:"active_requests"`
	TotalRequests  int64   `json:"total_requests"`
	TotalErrors    int64   `json:"total_errors"`
	AvgLatencyMS   float64 `json:"avg_latency_ms"`
}

// BalancingStrategy defines how requests are distributed across backends
type BalancingStrategy string

const (
	StrategyRoundRobin         BalancingStrategy = "round_robin"
	StrategyWeightedRoundRobin BalancingStrategy = "weighted_round_robin"
	StrategyLeastConnections   BalancingStrategy = "least_connections"
	StrategyLatency            BalancingStrategy = "latency"
	StrategyRandom             BalancingStrategy = "random"
)

// LoadBalancer distributes requests across multiple inference backends
type LoadBalancer struct {
	backends        map[string]*Backend
	strategy        BalancingStrategy
	rrIndex         uint64 // Round-robin index
	healthCheckInt  time.Duration
	healthCheckPath string
	mu              sync.RWMutex
	stopCh          chan struct{}
	httpClient      *http.Client
}

// LoadBalancerConfig contains configuration for the load balancer
type LoadBalancerConfig struct {
	Backends           []Backend         `json:"backends"`
	Strategy           BalancingStrategy `json:"strategy"`
	HealthCheckSeconds int               `json:"health_check_seconds"`
	HealthCheckPath    string            `json:"health_check_path"`
}

// NewLoadBalancer creates a new load balancer
func NewLoadBalancer(config LoadBalancerConfig) *LoadBalancer {
	lb := &LoadBalancer{
		backends:        make(map[string]*Backend),
		strategy:        config.Strategy,
		healthCheckInt:  time.Duration(config.HealthCheckSeconds) * time.Second,
		healthCheckPath: config.HealthCheckPath,
		stopCh:          make(chan struct{}),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	if lb.strategy == "" {
		lb.strategy = StrategyRoundRobin
	}
	if lb.healthCheckInt <= 0 {
		lb.healthCheckInt = 30 * time.Second
	}
	if lb.healthCheckPath == "" {
		lb.healthCheckPath = "/health"
	}

	// Initialize backends
	for _, b := range config.Backends {
		backend := b           // Copy
		backend.Healthy = true // Optimistically healthy until checked
		lb.backends[b.ID] = &backend
	}

	// Start health check loop
	go lb.healthCheckLoop()

	return lb
}

// AddBackend adds a new backend to the load balancer
func (lb *LoadBalancer) AddBackend(backend Backend) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	backend.Healthy = true
	lb.backends[backend.ID] = &backend
	log.Printf("Added backend: %s (%s)", backend.ID, backend.URL)
}

// RemoveBackend removes a backend from the load balancer
func (lb *LoadBalancer) RemoveBackend(backendID string) error {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if _, exists := lb.backends[backendID]; !exists {
		return fmt.Errorf("backend %s not found", backendID)
	}
	delete(lb.backends, backendID)
	log.Printf("Removed backend: %s", backendID)
	return nil
}

// EnableBackend enables a backend
func (lb *LoadBalancer) EnableBackend(backendID string, enabled bool) error {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	backend, exists := lb.backends[backendID]
	if !exists {
		return fmt.Errorf("backend %s not found", backendID)
	}
	backend.Enabled = enabled
	return nil
}

// GetBackend selects a backend based on the configured strategy
func (lb *LoadBalancer) GetBackend(modelID string) (*Backend, error) {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	// Filter healthy, enabled backends that have the model
	var available []*Backend
	for _, b := range lb.backends {
		if !b.Enabled || !b.Healthy {
			continue
		}
		// Check if backend has the model (empty models list means all models)
		if len(b.Models) > 0 {
			hasModel := false
			for _, m := range b.Models {
				if m == modelID || m == "*" {
					hasModel = true
					break
				}
			}
			if !hasModel {
				continue
			}
		}
		// Check max requests
		if b.MaxRequests > 0 && b.ActiveRequests >= int64(b.MaxRequests) {
			continue
		}
		available = append(available, b)
	}

	if len(available) == 0 {
		return nil, fmt.Errorf("no healthy backends available for model %s", modelID)
	}

	// Select based on strategy
	switch lb.strategy {
	case StrategyRoundRobin:
		return lb.selectRoundRobin(available), nil
	case StrategyWeightedRoundRobin:
		return lb.selectWeightedRoundRobin(available), nil
	case StrategyLeastConnections:
		return lb.selectLeastConnections(available), nil
	case StrategyLatency:
		return lb.selectLowestLatency(available), nil
	default:
		return lb.selectRoundRobin(available), nil
	}
}

// selectRoundRobin selects backends in round-robin order
func (lb *LoadBalancer) selectRoundRobin(backends []*Backend) *Backend {
	idx := atomic.AddUint64(&lb.rrIndex, 1) - 1
	return backends[idx%uint64(len(backends))]
}

// selectWeightedRoundRobin selects backends based on their weight
func (lb *LoadBalancer) selectWeightedRoundRobin(backends []*Backend) *Backend {
	// Calculate total weight
	totalWeight := 0
	for _, b := range backends {
		weight := b.Weight
		if weight <= 0 {
			weight = 1
		}
		totalWeight += weight
	}

	// Select based on weight
	idx := int(atomic.AddUint64(&lb.rrIndex, 1)) % totalWeight
	cumulative := 0
	for _, b := range backends {
		weight := b.Weight
		if weight <= 0 {
			weight = 1
		}
		cumulative += weight
		if idx < cumulative {
			return b
		}
	}
	return backends[0]
}

// selectLeastConnections selects the backend with fewest active connections
func (lb *LoadBalancer) selectLeastConnections(backends []*Backend) *Backend {
	var best *Backend
	minConnections := int64(-1)
	for _, b := range backends {
		if minConnections < 0 || b.ActiveRequests < minConnections {
			minConnections = b.ActiveRequests
			best = b
		}
	}
	return best
}

// selectLowestLatency selects the backend with lowest latency
func (lb *LoadBalancer) selectLowestLatency(backends []*Backend) *Backend {
	// Sort by latency
	sorted := make([]*Backend, len(backends))
	copy(sorted, backends)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].AvgLatencyMS < sorted[j].AvgLatencyMS
	})
	return sorted[0]
}

// MarkRequestStart marks the start of a request to a backend
func (lb *LoadBalancer) MarkRequestStart(backendID string) {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	if backend, exists := lb.backends[backendID]; exists {
		atomic.AddInt64(&backend.ActiveRequests, 1)
		atomic.AddInt64(&backend.TotalRequests, 1)
	}
}

// MarkRequestEnd marks the end of a request to a backend
func (lb *LoadBalancer) MarkRequestEnd(backendID string, latencyMS int64, success bool) {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	if backend, exists := lb.backends[backendID]; exists {
		atomic.AddInt64(&backend.ActiveRequests, -1)
		if !success {
			atomic.AddInt64(&backend.TotalErrors, 1)
		}
		// Update average latency (exponential moving average)
		if backend.AvgLatencyMS == 0 {
			backend.AvgLatencyMS = float64(latencyMS)
		} else {
			backend.AvgLatencyMS = backend.AvgLatencyMS*0.9 + float64(latencyMS)*0.1
		}
	}
}

// healthCheckLoop periodically checks backend health
func (lb *LoadBalancer) healthCheckLoop() {
	ticker := time.NewTicker(lb.healthCheckInt)
	defer ticker.Stop()

	// Initial check
	lb.checkAllBackends()

	for {
		select {
		case <-lb.stopCh:
			return
		case <-ticker.C:
			lb.checkAllBackends()
		}
	}
}

// checkAllBackends checks health of all backends
func (lb *LoadBalancer) checkAllBackends() {
	lb.mu.RLock()
	backends := make([]*Backend, 0, len(lb.backends))
	for _, b := range lb.backends {
		backends = append(backends, b)
	}
	lb.mu.RUnlock()

	var wg sync.WaitGroup
	for _, b := range backends {
		wg.Add(1)
		go func(backend *Backend) {
			defer wg.Done()
			lb.checkBackendHealth(backend)
		}(b)
	}
	wg.Wait()
}

// checkBackendHealth checks if a backend is healthy
func (lb *LoadBalancer) checkBackendHealth(backend *Backend) {
	startTime := time.Now()

	healthURL := backend.URL + lb.healthCheckPath
	req, err := http.NewRequest("GET", healthURL, nil)
	if err != nil {
		lb.markUnhealthy(backend, err)
		return
	}

	if backend.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+backend.APIKey)
	}

	resp, err := lb.httpClient.Do(req)
	if err != nil {
		lb.markUnhealthy(backend, err)
		return
	}
	defer resp.Body.Close()

	latencyMS := time.Since(startTime).Milliseconds()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		lb.mu.Lock()
		backend.Healthy = true
		backend.LastCheck = time.Now()
		backend.LastLatencyMS = latencyMS
		backend.ErrorCount = 0
		lb.mu.Unlock()
	} else {
		lb.markUnhealthy(backend, fmt.Errorf("health check returned status %d", resp.StatusCode))
	}
}

// markUnhealthy marks a backend as unhealthy
func (lb *LoadBalancer) markUnhealthy(backend *Backend, err error) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	backend.ErrorCount++
	backend.LastCheck = time.Now()

	// Mark unhealthy after 3 consecutive errors
	if backend.ErrorCount >= 3 {
		if backend.Healthy {
			log.Printf("Backend %s marked unhealthy: %v", backend.ID, err)
		}
		backend.Healthy = false
	}
}

// GetStats returns load balancer statistics
func (lb *LoadBalancer) GetStats() map[string]interface{} {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	backends := make([]map[string]interface{}, 0, len(lb.backends))
	healthyCount := 0
	totalRequests := int64(0)
	totalErrors := int64(0)

	for _, b := range lb.backends {
		backends = append(backends, map[string]interface{}{
			"id":              b.ID,
			"name":            b.Name,
			"url":             b.URL,
			"type":            b.Type,
			"healthy":         b.Healthy,
			"enabled":         b.Enabled,
			"weight":          b.Weight,
			"models":          b.Models,
			"active_requests": b.ActiveRequests,
			"total_requests":  b.TotalRequests,
			"total_errors":    b.TotalErrors,
			"avg_latency_ms":  b.AvgLatencyMS,
			"last_check":      b.LastCheck,
			"error_count":     b.ErrorCount,
		})
		if b.Healthy && b.Enabled {
			healthyCount++
		}
		totalRequests += b.TotalRequests
		totalErrors += b.TotalErrors
	}

	return map[string]interface{}{
		"strategy":         lb.strategy,
		"total_backends":   len(lb.backends),
		"healthy_backends": healthyCount,
		"total_requests":   totalRequests,
		"total_errors":     totalErrors,
		"backends":         backends,
	}
}

// SetStrategy changes the load balancing strategy
func (lb *LoadBalancer) SetStrategy(strategy BalancingStrategy) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.strategy = strategy
	log.Printf("Load balancer strategy changed to: %s", strategy)
}

// Stop stops the load balancer
func (lb *LoadBalancer) Stop() {
	close(lb.stopCh)
}

// ProxyRequest proxies a request to a backend
func (lb *LoadBalancer) ProxyRequest(modelID string, method, path string, body io.Reader, headers http.Header) (*http.Response, *Backend, error) {
	backend, err := lb.GetBackend(modelID)
	if err != nil {
		return nil, nil, err
	}

	lb.MarkRequestStart(backend.ID)
	startTime := time.Now()

	// Build target URL
	targetURL, err := url.Parse(backend.URL)
	if err != nil {
		lb.MarkRequestEnd(backend.ID, 0, false)
		return nil, backend, err
	}
	targetURL.Path = path

	req, err := http.NewRequest(method, targetURL.String(), body)
	if err != nil {
		lb.MarkRequestEnd(backend.ID, 0, false)
		return nil, backend, err
	}

	// Copy headers
	for k, v := range headers {
		req.Header[k] = v
	}

	// Add auth if needed
	if backend.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+backend.APIKey)
	}

	resp, err := lb.httpClient.Do(req)
	latencyMS := time.Since(startTime).Milliseconds()

	if err != nil {
		lb.MarkRequestEnd(backend.ID, latencyMS, false)
		return nil, backend, err
	}

	success := resp.StatusCode >= 200 && resp.StatusCode < 500
	lb.MarkRequestEnd(backend.ID, latencyMS, success)

	return resp, backend, nil
}

// ListBackends returns all configured backends
func (lb *LoadBalancer) ListBackends() []Backend {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	backends := make([]Backend, 0, len(lb.backends))
	for _, b := range lb.backends {
		backends = append(backends, *b)
	}
	return backends
}

// UpdateBackend updates a backend configuration
func (lb *LoadBalancer) UpdateBackend(backendID string, updates map[string]interface{}) error {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	backend, exists := lb.backends[backendID]
	if !exists {
		return fmt.Errorf("backend %s not found", backendID)
	}

	if name, ok := updates["name"].(string); ok {
		backend.Name = name
	}
	if url, ok := updates["url"].(string); ok {
		backend.URL = url
	}
	if weight, ok := updates["weight"].(int); ok {
		backend.Weight = weight
	}
	if maxReq, ok := updates["max_requests"].(int); ok {
		backend.MaxRequests = maxReq
	}
	if enabled, ok := updates["enabled"].(bool); ok {
		backend.Enabled = enabled
	}
	if apiKey, ok := updates["api_key"].(string); ok {
		backend.APIKey = apiKey
	}
	if models, ok := updates["models"].([]string); ok {
		backend.Models = models
	}

	return nil
}

// Serialize serializes the load balancer config for persistence
func (lb *LoadBalancer) Serialize() ([]byte, error) {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	config := LoadBalancerConfig{
		Strategy:           lb.strategy,
		HealthCheckSeconds: int(lb.healthCheckInt.Seconds()),
		HealthCheckPath:    lb.healthCheckPath,
	}

	for _, b := range lb.backends {
		config.Backends = append(config.Backends, *b)
	}

	return json.MarshalIndent(config, "", "  ")
}
