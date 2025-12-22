package server

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// Maximum number of rate limit buckets to prevent memory exhaustion under DDoS
const maxRateLimitBuckets = 10000

// RateLimiter implements a token bucket rate limiter per IP/endpoint
type RateLimiter struct {
	mu       sync.RWMutex
	buckets  map[string]*TokenBucket
	rate     int           // requests per interval
	interval time.Duration // time window
	burst    int           // max burst size
}

// TokenBucket represents a token bucket for rate limiting
type TokenBucket struct {
	tokens     int
	lastRefill time.Time
	mu         sync.Mutex
}

// NewRateLimiter creates a new rate limiter
// rate: number of requests allowed per interval
// interval: time window (e.g., 1 minute)
// burst: maximum burst size (can exceed rate briefly)
func NewRateLimiter(rate int, interval time.Duration, burst int) *RateLimiter {
	rl := &RateLimiter{
		buckets:  make(map[string]*TokenBucket),
		rate:     rate,
		interval: interval,
		burst:    burst,
	}

	// Start cleanup goroutine to remove old buckets
	go rl.cleanup()

	return rl
}

// Allow checks if a request from the given key should be allowed
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.RLock()
	bucket, exists := rl.buckets[key]
	bucketCount := len(rl.buckets)
	rl.mu.RUnlock()

	if !exists {
		rl.mu.Lock()
		// Double-check after acquiring write lock
		bucket, exists = rl.buckets[key]
		if !exists {
			// Check if we've hit the maximum bucket count (DDoS protection)
			if bucketCount >= maxRateLimitBuckets {
				// Find and evict the oldest bucket
				var oldestKey string
				var oldestTime time.Time
				first := true
				for k, b := range rl.buckets {
					if first || b.lastRefill.Before(oldestTime) {
						oldestKey = k
						oldestTime = b.lastRefill
						first = false
					}
				}
				if oldestKey != "" {
					delete(rl.buckets, oldestKey)
				}
			}
			bucket = &TokenBucket{
				tokens:     rl.burst,
				lastRefill: time.Now(),
			}
			rl.buckets[key] = bucket
		}
		rl.mu.Unlock()
	}

	return bucket.take(rl.rate, rl.interval)
}

// take attempts to take a token from the bucket
func (tb *TokenBucket) take(rate int, interval time.Duration) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.lastRefill)

	// Refill tokens based on elapsed time
	tokensToAdd := int(elapsed.Seconds() / interval.Seconds() * float64(rate))
	if tokensToAdd > 0 {
		tb.tokens += tokensToAdd
		if tb.tokens > rate {
			tb.tokens = rate
		}
		tb.lastRefill = now
	}

	// Try to take a token
	if tb.tokens > 0 {
		tb.tokens--
		return true
	}

	return false
}

// cleanup periodically removes old buckets
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for key, bucket := range rl.buckets {
			bucket.mu.Lock()
			if now.Sub(bucket.lastRefill) > 10*time.Minute {
				delete(rl.buckets, key)
			}
			bucket.mu.Unlock()
		}
		rl.mu.Unlock()
	}
}

// Middleware returns an HTTP middleware that applies rate limiting
func (rl *RateLimiter) Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Create key from IP + endpoint
		key := r.RemoteAddr + ":" + r.URL.Path

		if !rl.Allow(key) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]string{
					"message": "Rate limit exceeded. Please slow down your requests.",
					"type":    "rate_limit_error",
				},
			})
			return
		}

		next(w, r)
	}
}

// InferenceRateLimiter is a specialized rate limiter for inference endpoints
// It's more restrictive to prevent system overload
type InferenceRateLimiter struct {
	mu              sync.RWMutex
	activeRequests  map[string]int // IP -> count of active requests
	maxConcurrent   int            // max concurrent requests per IP
	globalSemaphore chan struct{}  // global concurrency limit
}

// NewInferenceRateLimiter creates a rate limiter for inference endpoints
func NewInferenceRateLimiter(maxConcurrentPerIP int, maxGlobalConcurrent int) *InferenceRateLimiter {
	return &InferenceRateLimiter{
		activeRequests:  make(map[string]int),
		maxConcurrent:   maxConcurrentPerIP,
		globalSemaphore: make(chan struct{}, maxGlobalConcurrent),
	}
}

// Acquire attempts to acquire a slot for inference
func (irl *InferenceRateLimiter) Acquire(ip string) bool {
	// Check global limit first (non-blocking)
	select {
	case irl.globalSemaphore <- struct{}{}:
		// Got global slot
	default:
		// Global limit reached
		return false
	}

	// Check per-IP limit
	irl.mu.Lock()
	defer irl.mu.Unlock()

	count := irl.activeRequests[ip]
	if count >= irl.maxConcurrent {
		// Release global slot since we can't proceed
		<-irl.globalSemaphore
		return false
	}

	irl.activeRequests[ip]++
	return true
}

// Release releases an inference slot
func (irl *InferenceRateLimiter) Release(ip string) {
	irl.mu.Lock()
	defer irl.mu.Unlock()

	if count := irl.activeRequests[ip]; count > 0 {
		irl.activeRequests[ip]--
		if irl.activeRequests[ip] == 0 {
			delete(irl.activeRequests, ip)
		}
	}

	// Release global slot
	<-irl.globalSemaphore
}

// Middleware returns an HTTP middleware for inference endpoints
func (irl *InferenceRateLimiter) Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr

		if !irl.Acquire(ip) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]string{
					"message": "Too many concurrent requests. Please wait for current requests to complete.",
					"type":    "concurrency_limit_error",
				},
			})
			return
		}

		defer irl.Release(ip)
		next(w, r)
	}
}
