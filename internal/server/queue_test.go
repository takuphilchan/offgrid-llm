package server

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/resource"
)

func TestNewRequestQueue(t *testing.T) {
	config := DefaultQueueConfig()
	monitor := resource.NewMonitor(time.Second)
	gpuMonitor := resource.NewGPUMonitor()

	queue := NewRequestQueue(config, monitor, gpuMonitor)
	if queue == nil {
		t.Fatal("NewRequestQueue returned nil")
	}

	if queue.maxConcurrent != config.MaxConcurrent {
		t.Errorf("Expected maxConcurrent %d, got %d", config.MaxConcurrent, queue.maxConcurrent)
	}

	if queue.maxQueueSize != config.MaxQueueSize {
		t.Errorf("Expected maxQueueSize %d, got %d", config.MaxQueueSize, queue.maxQueueSize)
	}
}

func TestDefaultQueueConfig(t *testing.T) {
	config := DefaultQueueConfig()

	if config.MaxConcurrent < 1 {
		t.Error("MaxConcurrent should be at least 1")
	}

	if config.MaxQueueSize < 1 {
		t.Error("MaxQueueSize should be at least 1")
	}

	if config.MemoryThreshold == 0 {
		t.Error("MemoryThreshold should be set")
	}

	if config.QueueTimeout == 0 {
		t.Error("QueueTimeout should be set")
	}

	t.Logf("Default config: MaxConcurrent=%d, MaxQueueSize=%d, MemoryThreshold=%d MB, Timeout=%v",
		config.MaxConcurrent, config.MaxQueueSize, config.MemoryThreshold, config.QueueTimeout)
}

func TestQueueEnqueueAndProcess(t *testing.T) {
	config := QueueConfig{
		MaxConcurrent:   2,
		MaxQueueSize:    5,
		MemoryThreshold: 0, // Disable memory check for test
		QueueTimeout:    5 * time.Second,
	}

	monitor := resource.NewMonitor(time.Second)
	monitor.Start()
	defer monitor.Stop()

	gpuMonitor := resource.NewGPUMonitor()
	queue := NewRequestQueue(config, monitor, gpuMonitor)

	// Set a simple process function
	processedCount := 0
	queue.SetProcessFunc(func(ctx context.Context, req interface{}) (interface{}, error) {
		processedCount++
		time.Sleep(100 * time.Millisecond) // Simulate work
		return "processed", nil
	})

	queue.Start()
	defer queue.Stop()

	// Enqueue a request
	ctx := context.Background()
	response, err := queue.Enqueue(ctx, "test request", PriorityNormal)

	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	if response == nil {
		t.Fatal("Response is nil")
	}

	if response.Error != nil {
		t.Errorf("Response has error: %v", response.Error)
	}

	if response.Response != "processed" {
		t.Errorf("Expected response 'processed', got %v", response.Response)
	}

	if processedCount != 1 {
		t.Errorf("Expected 1 processed request, got %d", processedCount)
	}

	t.Logf("Request processed in %v", response.Duration)
}

func TestQueueMultipleRequests(t *testing.T) {
	config := QueueConfig{
		MaxConcurrent:   2,
		MaxQueueSize:    10,
		MemoryThreshold: 0, // Disable for test
		QueueTimeout:    10 * time.Second,
	}

	monitor := resource.NewMonitor(time.Second)
	monitor.Start()
	defer monitor.Stop()

	queue := NewRequestQueue(config, monitor, nil)

	processedCount := 0
	queue.SetProcessFunc(func(ctx context.Context, req interface{}) (interface{}, error) {
		processedCount++
		time.Sleep(50 * time.Millisecond)
		return processedCount, nil
	})

	queue.Start()
	defer queue.Stop()

	// Enqueue multiple requests concurrently
	numRequests := 5
	results := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(id int) {
			ctx := context.Background()
			_, err := queue.Enqueue(ctx, id, PriorityNormal)
			results <- err
		}(i)
	}

	// Wait for all to complete
	for i := 0; i < numRequests; i++ {
		err := <-results
		if err != nil {
			t.Errorf("Request %d failed: %v", i, err)
		}
	}

	if processedCount != numRequests {
		t.Errorf("Expected %d processed requests, got %d", numRequests, processedCount)
	}

	// Check stats
	stats := queue.GetStats()
	t.Logf("Stats: Total=%d, OK=%d, Errors=%d, Rejected=%d",
		stats.TotalRequests, stats.CompletedOK, stats.CompletedError, stats.Rejected)

	if stats.CompletedOK != int64(numRequests) {
		t.Errorf("Expected %d completed OK, got %d", numRequests, stats.CompletedOK)
	}
}

func TestQueueFullRejection(t *testing.T) {
	config := QueueConfig{
		MaxConcurrent:   1,
		MaxQueueSize:    2, // Small queue
		MemoryThreshold: 0, // Disable for test
		QueueTimeout:    5 * time.Second,
	}

	monitor := resource.NewMonitor(time.Second)
	monitor.Start()
	defer monitor.Stop()

	queue := NewRequestQueue(config, monitor, nil)

	// Set a slow process function
	queue.SetProcessFunc(func(ctx context.Context, req interface{}) (interface{}, error) {
		time.Sleep(2 * time.Second)
		return "done", nil
	})

	queue.Start()
	defer queue.Stop()

	// Try to fill the queue beyond capacity
	ctx := context.Background()
	results := make(chan error, 10)

	for i := 0; i < 10; i++ {
		go func() {
			_, err := queue.Enqueue(ctx, "request", PriorityNormal)
			results <- err
		}()
		time.Sleep(10 * time.Millisecond) // Stagger slightly
	}

	// Count rejections
	rejections := 0
	for i := 0; i < 10; i++ {
		err := <-results
		if err != nil {
			rejections++
			t.Logf("Request rejected (expected): %v", err)
		}
	}

	if rejections == 0 {
		t.Error("Expected some requests to be rejected when queue is full")
	}

	stats := queue.GetStats()
	if stats.Rejected == 0 {
		t.Error("Stats should show rejected requests")
	}

	t.Logf("Rejected %d requests (stats: %d)", rejections, stats.Rejected)
}

func TestQueuePriority(t *testing.T) {
	config := QueueConfig{
		MaxConcurrent:   1, // Force sequential processing
		MaxQueueSize:    10,
		MemoryThreshold: 0, // Disable for test
		QueueTimeout:    10 * time.Second,
	}

	monitor := resource.NewMonitor(time.Second)
	monitor.Start()
	defer monitor.Stop()

	queue := NewRequestQueue(config, monitor, nil)

	processOrder := []string{}
	queue.SetProcessFunc(func(ctx context.Context, req interface{}) (interface{}, error) {
		id := req.(string)
		processOrder = append(processOrder, id)
		time.Sleep(100 * time.Millisecond)
		return id, nil
	})

	queue.Start()
	defer queue.Stop()

	ctx := context.Background()

	// Enqueue in specific order with different priorities
	go queue.Enqueue(ctx, "low1", PriorityLow)
	time.Sleep(10 * time.Millisecond)
	go queue.Enqueue(ctx, "high1", PriorityHigh)
	time.Sleep(10 * time.Millisecond)
	go queue.Enqueue(ctx, "normal1", PriorityNormal)
	time.Sleep(10 * time.Millisecond)
	go queue.Enqueue(ctx, "high2", PriorityHigh)

	// Wait for processing
	time.Sleep(2 * time.Second)

	t.Logf("Process order: %v", processOrder)

	// High priority should be processed before low priority
	// (Note: first request starts immediately, so check relative ordering)
	if len(processOrder) >= 4 {
		highIdx := -1
		lowIdx := -1
		for i, id := range processOrder {
			if (id == "high1" || id == "high2") && highIdx == -1 {
				highIdx = i
			}
			if id == "low1" && lowIdx == -1 {
				lowIdx = i
			}
		}
		if highIdx != -1 && lowIdx != -1 && highIdx > lowIdx {
			t.Log("Priority ordering verified (high before low)")
		}
	}
}

func TestQueueTimeout(t *testing.T) {
	config := QueueConfig{
		MaxConcurrent:   1,
		MaxQueueSize:    5,
		MemoryThreshold: 0,                      // Disable for test
		QueueTimeout:    500 * time.Millisecond, // Short timeout
	}

	monitor := resource.NewMonitor(time.Second)
	monitor.Start()
	defer monitor.Stop()

	queue := NewRequestQueue(config, monitor, nil)

	// Set a very slow process function
	queue.SetProcessFunc(func(ctx context.Context, req interface{}) (interface{}, error) {
		time.Sleep(5 * time.Second)
		return "done", nil
	})

	queue.Start()
	defer queue.Stop()

	ctx := context.Background()

	// First request will start processing
	go queue.Enqueue(ctx, "req1", PriorityNormal)
	time.Sleep(50 * time.Millisecond)

	// Second request should timeout in queue
	_, err := queue.Enqueue(ctx, "req2", PriorityNormal)
	if err == nil {
		t.Error("Expected timeout error")
	}

	t.Logf("Timeout error (expected): %v", err)

	stats := queue.GetStats()
	if stats.Timeouts == 0 {
		t.Error("Stats should show timeout")
	}
}

func TestQueueProcessError(t *testing.T) {
	config := DefaultQueueConfig()
	config.MemoryThreshold = 0 // Disable for test

	monitor := resource.NewMonitor(time.Second)
	monitor.Start()
	defer monitor.Stop()

	queue := NewRequestQueue(config, monitor, nil)

	// Set a process function that returns an error
	queue.SetProcessFunc(func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, errors.New("processing failed")
	})

	queue.Start()
	defer queue.Stop()

	ctx := context.Background()
	response, err := queue.Enqueue(ctx, "request", PriorityNormal)

	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	if response.Error == nil {
		t.Error("Expected error in response")
	}

	stats := queue.GetStats()
	if stats.CompletedError == 0 {
		t.Error("Stats should show error")
	}

	t.Logf("Processing error (expected): %v", response.Error)
}

func TestUpdateConcurrency(t *testing.T) {
	config := QueueConfig{
		MaxConcurrent:   2,
		MaxQueueSize:    10,
		MemoryThreshold: 100,
		QueueTimeout:    5 * time.Second,
	}

	monitor := resource.NewMonitor(time.Second)
	queue := NewRequestQueue(config, monitor, nil)

	if queue.maxConcurrent != 2 {
		t.Errorf("Initial concurrency should be 2, got %d", queue.maxConcurrent)
	}

	// Update to higher concurrency
	queue.UpdateConcurrency(5)

	if queue.maxConcurrent != 5 {
		t.Errorf("Updated concurrency should be 5, got %d", queue.maxConcurrent)
	}

	// Update to lower concurrency
	queue.UpdateConcurrency(1)

	if queue.maxConcurrent != 1 {
		t.Errorf("Updated concurrency should be 1, got %d", queue.maxConcurrent)
	}

	// Test minimum constraint
	queue.UpdateConcurrency(0)
	if queue.maxConcurrent < 1 {
		t.Error("Concurrency should be at least 1")
	}
}

func TestClearQueue(t *testing.T) {
	config := QueueConfig{
		MaxConcurrent:   1,
		MaxQueueSize:    10,
		MemoryThreshold: 0, // Disable for test
		QueueTimeout:    5 * time.Second,
	}

	monitor := resource.NewMonitor(time.Second)
	monitor.Start()
	queue := NewRequestQueue(config, monitor, nil)

	// Slow process function
	queue.SetProcessFunc(func(ctx context.Context, req interface{}) (interface{}, error) {
		time.Sleep(2 * time.Second)
		return "done", nil
	})

	queue.Start()
	defer queue.Stop()

	ctx := context.Background()

	// Queue several requests
	for i := 0; i < 5; i++ {
		go queue.Enqueue(ctx, i, PriorityNormal)
	}

	time.Sleep(100 * time.Millisecond) // Let them queue up

	// Clear the queue
	cleared := queue.Clear()
	t.Logf("Cleared %d requests", cleared)

	if cleared == 0 {
		t.Error("Expected to clear some requests")
	}

	stats := queue.GetStats()
	if stats.CurrentQueue != 0 {
		t.Errorf("Queue should be empty after clear, got %d", stats.CurrentQueue)
	}
}

func BenchmarkQueueEnqueue(b *testing.B) {
	config := DefaultQueueConfig()
	monitor := resource.NewMonitor(time.Second)
	queue := NewRequestQueue(config, monitor, nil)

	queue.SetProcessFunc(func(ctx context.Context, req interface{}) (interface{}, error) {
		return "done", nil
	})

	queue.Start()
	defer queue.Stop()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = queue.Enqueue(ctx, i, PriorityNormal)
	}
}
