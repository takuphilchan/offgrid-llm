package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// CacheEntry represents a cached response
type CacheEntry struct {
	Response  string    `json:"response"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Hits      int64     `json:"hits"` // Changed to int64 for atomic operations
}

// ResponseCache implements an LRU cache for model responses
type ResponseCache struct {
	mu         sync.RWMutex
	entries    map[string]*CacheEntry
	maxEntries int
	ttl        time.Duration
	hits       int64 // Use atomic operations for thread-safe counters
	misses     int64
	enabled    bool
	stopChan   chan struct{} // For stopping cleanup goroutine
}

// NewResponseCache creates a new response cache
func NewResponseCache(maxEntries int, ttl time.Duration) *ResponseCache {
	return &ResponseCache{
		entries:    make(map[string]*CacheEntry),
		maxEntries: maxEntries,
		ttl:        ttl,
		enabled:    true,
	}
}

// generateKey creates a cache key from model and prompt using efficient string concatenation
// This is much faster than JSON marshaling for key generation
func generateKey(model, prompt string, params map[string]interface{}) string {
	// Use a strings.Builder for efficient string concatenation
	var sb strings.Builder
	// Pre-allocate approximate size: model + prompt + params overhead
	sb.Grow(len(model) + len(prompt) + 256)

	sb.WriteString(model)
	sb.WriteByte('|')
	sb.WriteString(prompt)
	sb.WriteByte('|')

	// Sort params keys for deterministic ordering (params are usually small)
	if len(params) > 0 {
		keys := make([]string, 0, len(params))
		for k := range params {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for i, k := range keys {
			if i > 0 {
				sb.WriteByte(',')
			}
			sb.WriteString(k)
			sb.WriteByte(':')
			sb.WriteString(fmt.Sprint(params[k]))
		}
	}

	hash := sha256.Sum256([]byte(sb.String()))
	return hex.EncodeToString(hash[:])
}

// Get retrieves a cached response
func (c *ResponseCache) Get(model, prompt string, params map[string]interface{}) (string, bool) {
	if !c.enabled {
		return "", false
	}

	c.mu.RLock()
	key := generateKey(model, prompt, params)
	entry, exists := c.entries[key]

	if !exists {
		c.mu.RUnlock()
		atomic.AddInt64(&c.misses, 1)
		return "", false
	}

	// Check if expired
	if time.Now().After(entry.ExpiresAt) {
		c.mu.RUnlock()
		atomic.AddInt64(&c.misses, 1)
		return "", false
	}

	// Get response while holding read lock
	response := entry.Response
	c.mu.RUnlock()

	// Update hit counts atomically (thread-safe without write lock)
	atomic.AddInt64(&entry.Hits, 1)
	atomic.AddInt64(&c.hits, 1)

	return response, true
}

// Set stores a response in the cache
func (c *ResponseCache) Set(model, prompt, response string, params map[string]interface{}) {
	if !c.enabled {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	key := generateKey(model, prompt, params)

	// Check if we need to evict entries (LRU)
	if len(c.entries) >= c.maxEntries {
		c.evictOldest()
	}

	now := time.Now()
	c.entries[key] = &CacheEntry{
		Response:  response,
		CreatedAt: now,
		ExpiresAt: now.Add(c.ttl),
		Hits:      0,
	}
}

// evictOldest removes the oldest or least-used entry
func (c *ResponseCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time
	var minHits int64

	first := true
	for key, entry := range c.entries {
		hits := atomic.LoadInt64(&entry.Hits)
		if first {
			oldestKey = key
			oldestTime = entry.CreatedAt
			minHits = hits
			first = false
			continue
		}

		// Prioritize eviction by: expired > least hits > oldest
		if time.Now().After(entry.ExpiresAt) {
			oldestKey = key
			break
		} else if hits < minHits {
			oldestKey = key
			minHits = hits
			oldestTime = entry.CreatedAt
		} else if hits == minHits && entry.CreatedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.CreatedAt
		}
	}

	delete(c.entries, oldestKey)
}

// Clear removes all entries from the cache
func (c *ResponseCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*CacheEntry)
	atomic.StoreInt64(&c.hits, 0)
	atomic.StoreInt64(&c.misses, 0)
}

// Stats returns cache statistics
func (c *ResponseCache) Stats() map[string]interface{} {
	c.mu.RLock()
	entryCount := len(c.entries)
	maxEntries := c.maxEntries
	ttl := c.ttl
	enabled := c.enabled
	c.mu.RUnlock()

	// Read counters atomically
	hits := atomic.LoadInt64(&c.hits)
	misses := atomic.LoadInt64(&c.misses)

	hitRate := 0.0
	total := hits + misses
	if total > 0 {
		hitRate = float64(hits) / float64(total) * 100
	}

	return map[string]interface{}{
		"enabled":     enabled,
		"entries":     entryCount,
		"max_entries": maxEntries,
		"ttl_seconds": ttl.Seconds(),
		"hits":        hits,
		"misses":      misses,
		"hit_rate":    fmt.Sprintf("%.2f%%", hitRate),
	}
}

// Enable turns caching on
func (c *ResponseCache) Enable() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.enabled = true
}

// Disable turns caching off
func (c *ResponseCache) Disable() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.enabled = false
}

// CleanExpired removes expired entries
func (c *ResponseCache) CleanExpired() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	count := 0

	for key, entry := range c.entries {
		if now.After(entry.ExpiresAt) {
			delete(c.entries, key)
			count++
		}
	}

	return count
}

// StartCleanupRoutine runs periodic cleanup of expired entries
func (c *ResponseCache) StartCleanupRoutine(interval time.Duration) {
	c.stopChan = make(chan struct{})
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				removed := c.CleanExpired()
				if removed > 0 {
					fmt.Printf("Cache cleanup: removed %d expired entries\n", removed)
				}
			case <-c.stopChan:
				return
			}
		}
	}()
}

// StopCleanupRoutine stops the cleanup goroutine
func (c *ResponseCache) StopCleanupRoutine() {
	if c.stopChan != nil {
		close(c.stopChan)
		c.stopChan = nil
	}
}
