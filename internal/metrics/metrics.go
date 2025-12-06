package metrics

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// MetricType represents the type of a metric
type MetricType string

const (
	MetricTypeCounter   MetricType = "counter"
	MetricTypeGauge     MetricType = "gauge"
	MetricTypeHistogram MetricType = "histogram"
	MetricTypeSummary   MetricType = "summary"
)

// Metric represents a Prometheus metric
type Metric interface {
	Name() string
	Help() string
	Type() MetricType
	Collect() []string
}

// Counter is a monotonically increasing counter
type Counter struct {
	name   string
	help   string
	mu     sync.RWMutex
	values map[string]float64 // labels hash -> value
	labels []string
}

// NewCounter creates a new counter
func NewCounter(name, help string, labels ...string) *Counter {
	return &Counter{
		name:   name,
		help:   help,
		values: make(map[string]float64),
		labels: labels,
	}
}

func (c *Counter) Name() string     { return c.name }
func (c *Counter) Help() string     { return c.help }
func (c *Counter) Type() MetricType { return MetricTypeCounter }

// Inc increments the counter by 1
func (c *Counter) Inc(labelValues ...string) {
	c.Add(1, labelValues...)
}

// Add adds a value to the counter
func (c *Counter) Add(val float64, labelValues ...string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := labelsToKey(labelValues)
	c.values[key] += val
}

// Collect returns metric lines for Prometheus
func (c *Counter) Collect() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	lines := []string{
		fmt.Sprintf("# HELP %s %s", c.name, c.help),
		fmt.Sprintf("# TYPE %s counter", c.name),
	}

	for key, val := range c.values {
		labels := keyToLabels(c.labels, key)
		if labels == "" {
			lines = append(lines, fmt.Sprintf("%s %g", c.name, val))
		} else {
			lines = append(lines, fmt.Sprintf("%s{%s} %g", c.name, labels, val))
		}
	}

	return lines
}

// Gauge represents a value that can go up and down
type Gauge struct {
	name   string
	help   string
	mu     sync.RWMutex
	values map[string]float64
	labels []string
}

// NewGauge creates a new gauge
func NewGauge(name, help string, labels ...string) *Gauge {
	return &Gauge{
		name:   name,
		help:   help,
		values: make(map[string]float64),
		labels: labels,
	}
}

func (g *Gauge) Name() string     { return g.name }
func (g *Gauge) Help() string     { return g.help }
func (g *Gauge) Type() MetricType { return MetricTypeGauge }

// Set sets the gauge value
func (g *Gauge) Set(val float64, labelValues ...string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	key := labelsToKey(labelValues)
	g.values[key] = val
}

// Inc increments the gauge by 1
func (g *Gauge) Inc(labelValues ...string) {
	g.Add(1, labelValues...)
}

// Dec decrements the gauge by 1
func (g *Gauge) Dec(labelValues ...string) {
	g.Add(-1, labelValues...)
}

// Add adds a value to the gauge
func (g *Gauge) Add(val float64, labelValues ...string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	key := labelsToKey(labelValues)
	g.values[key] += val
}

// Collect returns metric lines for Prometheus
func (g *Gauge) Collect() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	lines := []string{
		fmt.Sprintf("# HELP %s %s", g.name, g.help),
		fmt.Sprintf("# TYPE %s gauge", g.name),
	}

	for key, val := range g.values {
		labels := keyToLabels(g.labels, key)
		if labels == "" {
			lines = append(lines, fmt.Sprintf("%s %g", g.name, val))
		} else {
			lines = append(lines, fmt.Sprintf("%s{%s} %g", g.name, labels, val))
		}
	}

	return lines
}

// Histogram tracks observations in buckets
type Histogram struct {
	name    string
	help    string
	mu      sync.RWMutex
	buckets []float64
	values  map[string]*histogramData
	labels  []string
}

type histogramData struct {
	counts map[float64]uint64
	sum    float64
	count  uint64
}

// NewHistogram creates a new histogram
func NewHistogram(name, help string, buckets []float64, labels ...string) *Histogram {
	// Sort buckets
	sorted := make([]float64, len(buckets))
	copy(sorted, buckets)
	sort.Float64s(sorted)

	return &Histogram{
		name:    name,
		help:    help,
		buckets: sorted,
		values:  make(map[string]*histogramData),
		labels:  labels,
	}
}

// DefaultBuckets returns default histogram buckets
func DefaultBuckets() []float64 {
	return []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}
}

// LatencyBuckets returns buckets suitable for latency measurements
func LatencyBuckets() []float64 {
	return []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60}
}

// TokenBuckets returns buckets suitable for token counts
func TokenBuckets() []float64 {
	return []float64{10, 50, 100, 250, 500, 1000, 2500, 5000, 10000, 25000}
}

func (h *Histogram) Name() string     { return h.name }
func (h *Histogram) Help() string     { return h.help }
func (h *Histogram) Type() MetricType { return MetricTypeHistogram }

// Observe records an observation
func (h *Histogram) Observe(val float64, labelValues ...string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	key := labelsToKey(labelValues)
	data, ok := h.values[key]
	if !ok {
		data = &histogramData{
			counts: make(map[float64]uint64),
		}
		h.values[key] = data
	}

	data.sum += val
	data.count++

	for _, bucket := range h.buckets {
		if val <= bucket {
			data.counts[bucket]++
		}
	}
}

// Collect returns metric lines for Prometheus
func (h *Histogram) Collect() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	lines := []string{
		fmt.Sprintf("# HELP %s %s", h.name, h.help),
		fmt.Sprintf("# TYPE %s histogram", h.name),
	}

	for key, data := range h.values {
		baseLabels := keyToLabels(h.labels, key)

		// Cumulative bucket counts
		var cumulative uint64
		for _, bucket := range h.buckets {
			cumulative += data.counts[bucket]
			if baseLabels == "" {
				lines = append(lines, fmt.Sprintf("%s_bucket{le=\"%g\"} %d", h.name, bucket, cumulative))
			} else {
				lines = append(lines, fmt.Sprintf("%s_bucket{%s,le=\"%g\"} %d", h.name, baseLabels, bucket, cumulative))
			}
		}

		// +Inf bucket
		if baseLabels == "" {
			lines = append(lines, fmt.Sprintf("%s_bucket{le=\"+Inf\"} %d", h.name, data.count))
			lines = append(lines, fmt.Sprintf("%s_sum %g", h.name, data.sum))
			lines = append(lines, fmt.Sprintf("%s_count %d", h.name, data.count))
		} else {
			lines = append(lines, fmt.Sprintf("%s_bucket{%s,le=\"+Inf\"} %d", h.name, baseLabels, data.count))
			lines = append(lines, fmt.Sprintf("%s_sum{%s} %g", h.name, baseLabels, data.sum))
			lines = append(lines, fmt.Sprintf("%s_count{%s} %d", h.name, baseLabels, data.count))
		}
	}

	return lines
}

// Registry holds all registered metrics
type Registry struct {
	mu      sync.RWMutex
	metrics map[string]Metric
}

// NewRegistry creates a new registry
func NewRegistry() *Registry {
	return &Registry{
		metrics: make(map[string]Metric),
	}
}

// Register registers a metric
func (r *Registry) Register(m Metric) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.metrics[m.Name()] = m
}

// Unregister unregisters a metric
func (r *Registry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.metrics, name)
}

// Collect collects all metrics
func (r *Registry) Collect() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var lines []string
	for _, m := range r.metrics {
		lines = append(lines, m.Collect()...)
	}

	return strings.Join(lines, "\n") + "\n"
}

// Handler returns an HTTP handler for the metrics endpoint
func (r *Registry) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		w.Write([]byte(r.Collect()))
	})
}

// DefaultRegistry is the default metrics registry
var DefaultRegistry = NewRegistry()

// Helper functions

func labelsToKey(labelValues []string) string {
	return strings.Join(labelValues, "\x00")
}

func keyToLabels(labelNames []string, key string) string {
	if key == "" {
		return ""
	}

	values := strings.Split(key, "\x00")
	pairs := make([]string, 0, len(labelNames))
	for i, name := range labelNames {
		if i < len(values) {
			pairs = append(pairs, fmt.Sprintf("%s=\"%s\"", name, escapeLabel(values[i])))
		}
	}
	return strings.Join(pairs, ",")
}

func escapeLabel(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	return s
}

// OffGrid-specific metrics

// OffGridMetrics contains all OffGrid LLM metrics
type OffGridMetrics struct {
	// Request metrics
	RequestsTotal    *Counter
	RequestDuration  *Histogram
	RequestsInFlight *Gauge

	// LLM metrics
	TokensInputTotal   *Counter
	TokensOutputTotal  *Counter
	TokensPerRequest   *Histogram
	GenerationDuration *Histogram
	TokensPerSecond    *Histogram

	// Model metrics
	ModelLoaded   *Gauge
	ModelLoadTime *Histogram

	// RAG metrics
	RAGQueriesTotal    *Counter
	RAGQueryDuration   *Histogram
	RAGDocumentsTotal  *Gauge
	RAGChunksTotal     *Gauge
	RAGEmbeddingsTotal *Counter

	// Session metrics
	ActiveSessions  *Gauge
	SessionsCreated *Counter

	// Error metrics
	ErrorsTotal *Counter

	// Resource metrics
	MemoryUsage *Gauge
	CPUUsage    *Gauge
	DiskUsage   *Gauge

	// WebSocket metrics
	WebSocketConnections *Gauge
	WebSocketMessages    *Counter

	// User metrics
	ActiveUsers   *Gauge
	TotalUsers    *Gauge
	QuotaExceeded *Counter
}

// NewOffGridMetrics creates a new set of OffGrid metrics
func NewOffGridMetrics() *OffGridMetrics {
	m := &OffGridMetrics{
		// Request metrics
		RequestsTotal: NewCounter(
			"offgrid_requests_total",
			"Total number of requests",
			"method", "endpoint", "status",
		),
		RequestDuration: NewHistogram(
			"offgrid_request_duration_seconds",
			"Request duration in seconds",
			LatencyBuckets(),
			"method", "endpoint",
		),
		RequestsInFlight: NewGauge(
			"offgrid_requests_in_flight",
			"Number of requests currently being processed",
		),

		// LLM metrics
		TokensInputTotal: NewCounter(
			"offgrid_tokens_input_total",
			"Total number of input tokens processed",
			"model",
		),
		TokensOutputTotal: NewCounter(
			"offgrid_tokens_output_total",
			"Total number of output tokens generated",
			"model",
		),
		TokensPerRequest: NewHistogram(
			"offgrid_tokens_per_request",
			"Number of tokens per request",
			TokenBuckets(),
			"type", // input or output
		),
		GenerationDuration: NewHistogram(
			"offgrid_generation_duration_seconds",
			"LLM generation duration in seconds",
			LatencyBuckets(),
			"model",
		),
		TokensPerSecond: NewHistogram(
			"offgrid_tokens_per_second",
			"Token generation rate (tokens/second)",
			[]float64{1, 5, 10, 25, 50, 100, 200, 500},
			"model",
		),

		// Model metrics
		ModelLoaded: NewGauge(
			"offgrid_model_loaded",
			"Whether a model is currently loaded",
			"model",
		),
		ModelLoadTime: NewHistogram(
			"offgrid_model_load_seconds",
			"Time to load a model in seconds",
			[]float64{0.5, 1, 2, 5, 10, 30, 60, 120},
			"model",
		),

		// RAG metrics
		RAGQueriesTotal: NewCounter(
			"offgrid_rag_queries_total",
			"Total number of RAG queries",
		),
		RAGQueryDuration: NewHistogram(
			"offgrid_rag_query_duration_seconds",
			"RAG query duration in seconds",
			LatencyBuckets(),
		),
		RAGDocumentsTotal: NewGauge(
			"offgrid_rag_documents_total",
			"Total number of documents in RAG",
		),
		RAGChunksTotal: NewGauge(
			"offgrid_rag_chunks_total",
			"Total number of chunks in RAG",
		),
		RAGEmbeddingsTotal: NewCounter(
			"offgrid_rag_embeddings_total",
			"Total number of embeddings generated",
		),

		// Session metrics
		ActiveSessions: NewGauge(
			"offgrid_active_sessions",
			"Number of active sessions",
		),
		SessionsCreated: NewCounter(
			"offgrid_sessions_created_total",
			"Total number of sessions created",
		),

		// Error metrics
		ErrorsTotal: NewCounter(
			"offgrid_errors_total",
			"Total number of errors",
			"type", "endpoint",
		),

		// Resource metrics
		MemoryUsage: NewGauge(
			"offgrid_memory_bytes",
			"Memory usage in bytes",
			"type", // heap, stack, etc.
		),
		CPUUsage: NewGauge(
			"offgrid_cpu_usage_percent",
			"CPU usage percentage",
		),
		DiskUsage: NewGauge(
			"offgrid_disk_bytes",
			"Disk usage in bytes",
			"path",
		),

		// WebSocket metrics
		WebSocketConnections: NewGauge(
			"offgrid_websocket_connections",
			"Number of active WebSocket connections",
		),
		WebSocketMessages: NewCounter(
			"offgrid_websocket_messages_total",
			"Total number of WebSocket messages",
			"direction", // sent or received
		),

		// User metrics
		ActiveUsers: NewGauge(
			"offgrid_active_users",
			"Number of active users",
		),
		TotalUsers: NewGauge(
			"offgrid_total_users",
			"Total number of registered users",
		),
		QuotaExceeded: NewCounter(
			"offgrid_quota_exceeded_total",
			"Total number of quota exceeded events",
			"user_id", "quota_type",
		),
	}

	// Register all metrics
	DefaultRegistry.Register(m.RequestsTotal)
	DefaultRegistry.Register(m.RequestDuration)
	DefaultRegistry.Register(m.RequestsInFlight)
	DefaultRegistry.Register(m.TokensInputTotal)
	DefaultRegistry.Register(m.TokensOutputTotal)
	DefaultRegistry.Register(m.TokensPerRequest)
	DefaultRegistry.Register(m.GenerationDuration)
	DefaultRegistry.Register(m.TokensPerSecond)
	DefaultRegistry.Register(m.ModelLoaded)
	DefaultRegistry.Register(m.ModelLoadTime)
	DefaultRegistry.Register(m.RAGQueriesTotal)
	DefaultRegistry.Register(m.RAGQueryDuration)
	DefaultRegistry.Register(m.RAGDocumentsTotal)
	DefaultRegistry.Register(m.RAGChunksTotal)
	DefaultRegistry.Register(m.RAGEmbeddingsTotal)
	DefaultRegistry.Register(m.ActiveSessions)
	DefaultRegistry.Register(m.SessionsCreated)
	DefaultRegistry.Register(m.ErrorsTotal)
	DefaultRegistry.Register(m.MemoryUsage)
	DefaultRegistry.Register(m.CPUUsage)
	DefaultRegistry.Register(m.DiskUsage)
	DefaultRegistry.Register(m.WebSocketConnections)
	DefaultRegistry.Register(m.WebSocketMessages)
	DefaultRegistry.Register(m.ActiveUsers)
	DefaultRegistry.Register(m.TotalUsers)
	DefaultRegistry.Register(m.QuotaExceeded)

	// Initialize gauges with base values so they appear in /metrics output
	m.RequestsInFlight.Set(0)
	m.ModelLoaded.Set(0)
	m.RAGDocumentsTotal.Set(0)
	m.RAGChunksTotal.Set(0)
	m.ActiveSessions.Set(0)
	m.MemoryUsage.Set(0)
	m.CPUUsage.Set(0)
	m.DiskUsage.Set(0)
	m.WebSocketConnections.Set(0)
	m.ActiveUsers.Set(0)
	m.TotalUsers.Set(0)

	return m
}

// Timer helps measure duration
type Timer struct {
	start time.Time
}

// NewTimer creates a new timer
func NewTimer() *Timer {
	return &Timer{start: time.Now()}
}

// ObserveDuration records the duration to a histogram
func (t *Timer) ObserveDuration(h *Histogram, labelValues ...string) {
	h.Observe(time.Since(t.start).Seconds(), labelValues...)
}

// Seconds returns elapsed seconds
func (t *Timer) Seconds() float64 {
	return time.Since(t.start).Seconds()
}

// MetricsMiddleware returns HTTP middleware for metrics collection
func MetricsMiddleware(m *OffGridMetrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			timer := NewTimer()
			m.RequestsInFlight.Inc()

			// Wrap response writer to capture status
			wrapped := &responseWriter{ResponseWriter: w, statusCode: 200}

			next.ServeHTTP(wrapped, r)

			m.RequestsInFlight.Dec()
			m.RequestsTotal.Inc(r.Method, r.URL.Path, fmt.Sprintf("%d", wrapped.statusCode))
			timer.ObserveDuration(m.RequestDuration, r.Method, r.URL.Path)
		})
	}
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Global metrics instance
var Metrics = NewOffGridMetrics()
