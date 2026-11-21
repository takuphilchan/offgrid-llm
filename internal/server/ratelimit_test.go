package server

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestRateLimiterCloseJoinsCleanupAndPreservesDrain(t *testing.T) {
	rl := NewRateLimiter(1, time.Hour, 1)
	var closing sync.WaitGroup
	for range 16 {
		closing.Go(func() {
			if err := rl.Close(); err != nil {
				t.Error(err)
			}
		})
	}
	closing.Wait()
	select {
	case <-rl.stopped:
	default:
		t.Fatal("Close returned before the cleanup worker stopped")
	}
	if !rl.Allow("draining") || rl.Allow("draining") {
		t.Fatal("closing cleanup changed enforcement for draining handlers")
	}
}

func TestRateLimiterConcurrentEvictionUsesSynchronizedRefill(t *testing.T) {
	rl := NewRateLimiter(100, time.Nanosecond, 100)
	t.Cleanup(func() { _ = rl.Close() })
	// Share one frequently refilled bucket to exercise eviction reading its
	// timestamp while another request refills it.
	bucket := &TokenBucket{tokens: 100, lastRefill: time.Now()}
	for i := range maxRateLimitBuckets {
		rl.buckets[fmt.Sprint(i)] = bucket
	}
	start, done := make(chan struct{}), make(chan struct{})
	var workers sync.WaitGroup
	workers.Go(func() {
		<-start
		for {
			select {
			case <-done:
				return
			default:
				bucket.take(100, time.Nanosecond, 100)
			}
		}
	})
	for i := range 32 {
		workers.Go(func() { <-start; rl.Allow(fmt.Sprintf("new-%d", i)) })
	}
	close(start)
	// Each eviction completes before the refill worker is released.
	for i := range 32 {
		rl.Allow(fmt.Sprintf("followup-%d", i))
	}
	close(done)
	workers.Wait()
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	if len(rl.buckets) != maxRateLimitBuckets {
		t.Fatalf("bucket bound violated: %d", len(rl.buckets))
	}
}

func TestRateLimiterEvictsOldestEmptyKeyAtCapacity(t *testing.T) {
	rl := NewRateLimiter(1, time.Hour, 1)
	t.Cleanup(func() { _ = rl.Close() })
	now := time.Now()
	rl.buckets[""] = &TokenBucket{tokens: 1, lastRefill: now.Add(-time.Hour)}
	for i := 1; i < maxRateLimitBuckets; i++ {
		rl.buckets[fmt.Sprint(i)] = &TokenBucket{tokens: 1, lastRefill: now}
	}
	if !rl.Allow("new") {
		t.Fatal("new bucket did not admit its first request")
	}
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	if _, exists := rl.buckets[""]; exists || len(rl.buckets) != maxRateLimitBuckets {
		t.Fatalf("oldest empty key was not evicted within the bucket bound: exists=%v count=%d", exists, len(rl.buckets))
	}
}

func TestClientIPIgnoresSourcePort(t *testing.T) {
	tests := map[string]string{
		"192.0.2.10:12345":  "192.0.2.10",
		"192.0.2.10:54321":  "192.0.2.10",
		"[2001:db8::1]:443": "2001:db8::1",
		"192.0.2.10":        "192.0.2.10",
	}
	for remoteAddr, want := range tests {
		if got := clientIP(remoteAddr); got != want {
			t.Errorf("clientIP(%q) = %q, want %q", remoteAddr, got, want)
		}
	}
}
