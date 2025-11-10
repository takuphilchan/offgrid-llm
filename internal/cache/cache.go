package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// CacheEntry represents a cached response
type CacheEntry struct {
	Response  string    `json:"response"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Hits      int       `json:"hits"`
}

// ResponseCache implements an LRU cache for model responses
type ResponseCache struct {
	mu         sync.RWMutex
	entries    map[string]*CacheEntry
	maxEntries int
	ttl        time.Duration
	hits       int64
	misses     int64
	enabled    bool
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

// generateKey creates a cache key from model and prompt
func generateKey(model, prompt string, params map[string]interface{}) string {
	// Create a deterministic key from model, prompt, and parameters
	data := struct {
		Model  string                 `json:"model"`
		Prompt string                 `json:"prompt"`
		Params map[string]interface{} `json:"params"`
	}{
		Model:  model,
		Prompt: prompt,
		Params: params,
	}

	jsonData, _ := json.Marshal(data)
	hash := sha256.Sum256(jsonData)
	return hex.EncodeToString(hash[:])
}

// Get retrieves a cached response
func (c *ResponseCache) Get(model, prompt string, params map[string]interface{}) (string, bool) {
	if !c.enabled {
		return "", false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	key := generateKey(model, prompt, params)
	entry, exists := c.entries[key]

	if !exists {
		c.misses++
		return "", false
	}

	// Check if expired
	if time.Now().After(entry.ExpiresAt) {
		c.misses++
		return "", false
	}

	// Update hit count
	entry.Hits++
	c.hits++

	return entry.Response, true
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
	var minHits int

	first := true
	for key, entry := range c.entries {
		if first {
			oldestKey = key
			oldestTime = entry.CreatedAt
			minHits = entry.Hits
			first = false
			continue
		}

		// Prioritize eviction by: expired > least hits > oldest
		if time.Now().After(entry.ExpiresAt) {
			oldestKey = key
			break
		} else if entry.Hits < minHits {
			oldestKey = key
			minHits = entry.Hits
			oldestTime = entry.CreatedAt
		} else if entry.Hits == minHits && entry.CreatedAt.Before(oldestTime) {
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
	c.hits = 0
	c.misses = 0
}

// Stats returns cache statistics
func (c *ResponseCache) Stats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	hitRate := 0.0
	total := c.hits + c.misses
	if total > 0 {
		hitRate = float64(c.hits) / float64(total) * 100
	}

	return map[string]interface{}{
		"enabled":     c.enabled,
		"entries":     len(c.entries),
		"max_entries": c.maxEntries,
		"ttl_seconds": c.ttl.Seconds(),
		"hits":        c.hits,
		"misses":      c.misses,
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
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			removed := c.CleanExpired()
			if removed > 0 {
				fmt.Printf("Cache cleanup: removed %d expired entries\n", removed)
			}
		}
	}()
}
