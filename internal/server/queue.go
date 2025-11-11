package server

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/resource"
)

// RequestPriority defines request priority levels
type RequestPriority int

const (
	PriorityLow RequestPriority = iota
	PriorityNormal
	PriorityHigh
)

// QueuedRequest represents a request waiting in the queue
type QueuedRequest struct {
	ID           string
	Request      interface{} // *api.ChatCompletionRequest or *api.CompletionRequest
	Priority     RequestPriority
	QueuedAt     time.Time
	ResponseChan chan *QueuedResponse
	Context      context.Context
}

// QueuedResponse represents the response to a queued request
type QueuedResponse struct {
	Response interface{}
	Error    error
	Duration time.Duration
}

// RequestQueue manages concurrent inference requests
type RequestQueue struct {
	mu              sync.RWMutex
	queue           []*QueuedRequest
	maxConcurrent   int
	currentRunning  int
	maxQueueSize    int
	processingChan  chan *QueuedRequest
	stopChan        chan struct{}
	resourceMonitor *resource.Monitor
	gpuMonitor      *resource.GPUMonitor
	memoryThreshold uint64 // MB - reject requests if below this
	queueTimeout    time.Duration
	stats           QueueStats
	processFunc     ProcessFunc
}

// QueueStats tracks queue statistics
type QueueStats struct {
	TotalRequests   int64         `json:"total_requests"`
	CompletedOK     int64         `json:"completed_ok"`
	CompletedError  int64         `json:"completed_error"`
	Rejected        int64         `json:"rejected"`
	Timeouts        int64         `json:"timeouts"`
	CurrentQueue    int           `json:"current_queue"`
	CurrentRunning  int           `json:"current_running"`
	AvgWaitTime     time.Duration `json:"avg_wait_time"`
	AvgProcessTime  time.Duration `json:"avg_process_time"`
	MaxQueueDepth   int           `json:"max_queue_depth"`
	LastRequestTime time.Time     `json:"last_request_time"`
}

// ProcessFunc is the function signature for processing requests
type ProcessFunc func(context.Context, interface{}) (interface{}, error)

// QueueConfig contains queue configuration
type QueueConfig struct {
	MaxConcurrent   int
	MaxQueueSize    int
	MemoryThreshold uint64 // MB
	QueueTimeout    time.Duration
}

// DefaultQueueConfig returns sensible defaults
func DefaultQueueConfig() QueueConfig {
	return QueueConfig{
		MaxConcurrent:   2,   // Safe for edge devices
		MaxQueueSize:    10,  // Prevent unbounded growth
		MemoryThreshold: 512, // Require at least 512MB free
		QueueTimeout:    5 * time.Minute,
	}
}

// NewRequestQueue creates a new request queue
func NewRequestQueue(config QueueConfig, monitor *resource.Monitor, gpuMonitor *resource.GPUMonitor) *RequestQueue {
	if config.MaxConcurrent < 1 {
		config.MaxConcurrent = 1
	}
	if config.MaxQueueSize < 1 {
		config.MaxQueueSize = 10
	}

	return &RequestQueue{
		queue:           make([]*QueuedRequest, 0),
		maxConcurrent:   config.MaxConcurrent,
		maxQueueSize:    config.MaxQueueSize,
		processingChan:  make(chan *QueuedRequest, config.MaxConcurrent),
		stopChan:        make(chan struct{}),
		resourceMonitor: monitor,
		gpuMonitor:      gpuMonitor,
		memoryThreshold: config.MemoryThreshold,
		queueTimeout:    config.QueueTimeout,
	}
}

// SetProcessFunc sets the function used to process requests
func (q *RequestQueue) SetProcessFunc(fn ProcessFunc) {
	q.processFunc = fn
}

// Start starts the queue workers
func (q *RequestQueue) Start() {
	// Start worker goroutines
	for i := 0; i < q.maxConcurrent; i++ {
		go q.worker(i)
	}

	// Start queue dispatcher
	go q.dispatcher()
}

// Stop stops the queue
func (q *RequestQueue) Stop() {
	close(q.stopChan)
}

// Enqueue adds a request to the queue
func (q *RequestQueue) Enqueue(ctx context.Context, request interface{}, priority RequestPriority) (*QueuedResponse, error) {
	q.mu.Lock()

	// Check queue size limit
	if len(q.queue) >= q.maxQueueSize {
		q.stats.Rejected++
		q.mu.Unlock()
		return nil, fmt.Errorf("queue full: %d/%d requests", len(q.queue), q.maxQueueSize)
	}

	// Check available memory
	if q.resourceMonitor != nil {
		stats := q.resourceMonitor.GetStats()
		availableMB := stats.MemoryTotalMB - stats.MemoryUsedMB
		if availableMB < q.memoryThreshold {
			q.stats.Rejected++
			q.mu.Unlock()
			return nil, fmt.Errorf("insufficient memory: %d MB available, %d MB required", availableMB, q.memoryThreshold)
		}
	}

	// Create queued request
	qr := &QueuedRequest{
		ID:           fmt.Sprintf("req-%d-%d", time.Now().UnixNano(), len(q.queue)),
		Request:      request,
		Priority:     priority,
		QueuedAt:     time.Now(),
		ResponseChan: make(chan *QueuedResponse, 1),
		Context:      ctx,
	}

	// Add to queue
	q.queue = append(q.queue, qr)
	q.stats.TotalRequests++
	q.stats.CurrentQueue = len(q.queue)
	q.stats.LastRequestTime = time.Now()

	if len(q.queue) > q.stats.MaxQueueDepth {
		q.stats.MaxQueueDepth = len(q.queue)
	}

	q.mu.Unlock()

	// Wait for response with timeout
	select {
	case response := <-qr.ResponseChan:
		return response, nil
	case <-time.After(q.queueTimeout):
		q.mu.Lock()
		q.stats.Timeouts++
		q.mu.Unlock()
		return nil, errors.New("request timed out in queue")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// dispatcher continuously moves requests from queue to workers
func (q *RequestQueue) dispatcher() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			q.dispatch()
		case <-q.stopChan:
			return
		}
	}
}

// dispatch moves next priority request to processing channel
func (q *RequestQueue) dispatch() {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Check if we can dispatch
	if q.currentRunning >= q.maxConcurrent || len(q.queue) == 0 {
		return
	}

	// Find highest priority request
	bestIdx := 0
	for i, req := range q.queue {
		if req.Priority > q.queue[bestIdx].Priority {
			bestIdx = i
		}
	}

	// Remove from queue and send to processing
	request := q.queue[bestIdx]
	q.queue = append(q.queue[:bestIdx], q.queue[bestIdx+1:]...)
	q.stats.CurrentQueue = len(q.queue)
	q.currentRunning++

	// Send to worker (non-blocking)
	select {
	case q.processingChan <- request:
	default:
		// Should not happen, but handle gracefully
		q.currentRunning--
		request.ResponseChan <- &QueuedResponse{
			Error: errors.New("failed to dispatch to worker"),
		}
	}
}

// worker processes requests from the processing channel
func (q *RequestQueue) worker(id int) {
	for {
		select {
		case request := <-q.processingChan:
			q.processRequest(request)
		case <-q.stopChan:
			return
		}
	}
}

// processRequest executes a single request
func (q *RequestQueue) processRequest(request *QueuedRequest) {
	defer func() {
		q.mu.Lock()
		q.currentRunning--
		q.mu.Unlock()
	}()

	startTime := time.Now()
	waitTime := startTime.Sub(request.QueuedAt)

	// Update wait time stats
	q.mu.Lock()
	if q.stats.AvgWaitTime == 0 {
		q.stats.AvgWaitTime = waitTime
	} else {
		q.stats.AvgWaitTime = (q.stats.AvgWaitTime + waitTime) / 2
	}
	q.mu.Unlock()

	// Process the request
	var response interface{}
	var err error

	if q.processFunc != nil {
		response, err = q.processFunc(request.Context, request.Request)
	} else {
		err = errors.New("no process function configured")
	}

	duration := time.Since(startTime)

	// Update process time stats
	q.mu.Lock()
	if err != nil {
		q.stats.CompletedError++
	} else {
		q.stats.CompletedOK++
	}

	if q.stats.AvgProcessTime == 0 {
		q.stats.AvgProcessTime = duration
	} else {
		q.stats.AvgProcessTime = (q.stats.AvgProcessTime + duration) / 2
	}
	q.mu.Unlock()

	// Send response
	request.ResponseChan <- &QueuedResponse{
		Response: response,
		Error:    err,
		Duration: duration,
	}
}

// GetStats returns current queue statistics
func (q *RequestQueue) GetStats() QueueStats {
	q.mu.RLock()
	defer q.mu.RUnlock()

	stats := q.stats
	stats.CurrentQueue = len(q.queue)
	stats.CurrentRunning = q.currentRunning
	return stats
}

// UpdateConcurrency changes max concurrent requests (dynamic adjustment)
func (q *RequestQueue) UpdateConcurrency(maxConcurrent int) {
	if maxConcurrent < 1 {
		maxConcurrent = 1
	}

	q.mu.Lock()
	oldMax := q.maxConcurrent
	q.maxConcurrent = maxConcurrent
	q.mu.Unlock()

	// Start additional workers if needed
	if maxConcurrent > oldMax {
		for i := oldMax; i < maxConcurrent; i++ {
			go q.worker(i)
		}
	}
}

// Clear removes all pending requests from the queue
func (q *RequestQueue) Clear() int {
	q.mu.Lock()
	defer q.mu.Unlock()

	count := len(q.queue)
	for _, req := range q.queue {
		req.ResponseChan <- &QueuedResponse{
			Error: errors.New("queue cleared"),
		}
	}

	q.queue = make([]*QueuedRequest, 0)
	q.stats.CurrentQueue = 0
	return count
}
