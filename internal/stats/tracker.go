package stats

import (
	"sync"
	"time"
)

// InferenceStats tracks statistics for model inference
type InferenceStats struct {
	ModelID           string    `json:"model_id"`
	TotalRequests     int64     `json:"total_requests"`
	TotalTokens       int64     `json:"total_tokens"`
	TotalDurationMs   int64     `json:"total_duration_ms"`
	AverageResponseMs float64   `json:"average_response_ms"`
	LastUsed          time.Time `json:"last_used"`
}

// Tracker manages inference statistics
type Tracker struct {
	mu    sync.RWMutex
	stats map[string]*InferenceStats
}

// NewTracker creates a new statistics tracker
func NewTracker() *Tracker {
	return &Tracker{
		stats: make(map[string]*InferenceStats),
	}
}

// RecordInference records an inference request
func (t *Tracker) RecordInference(modelID string, tokens int64, durationMs int64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, exists := t.stats[modelID]; !exists {
		t.stats[modelID] = &InferenceStats{
			ModelID: modelID,
		}
	}

	s := t.stats[modelID]
	s.TotalRequests++
	s.TotalTokens += tokens
	s.TotalDurationMs += durationMs
	s.AverageResponseMs = float64(s.TotalDurationMs) / float64(s.TotalRequests)
	s.LastUsed = time.Now()
}

// GetStats returns statistics for a specific model
func (t *Tracker) GetStats(modelID string) *InferenceStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if s, exists := t.stats[modelID]; exists {
		// Return a copy
		statsCopy := *s
		return &statsCopy
	}
	return nil
}

// GetAllStats returns statistics for all models
func (t *Tracker) GetAllStats() map[string]*InferenceStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make(map[string]*InferenceStats)
	for k, v := range t.stats {
		statsCopy := *v
		result[k] = &statsCopy
	}
	return result
}

// Reset clears all statistics
func (t *Tracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stats = make(map[string]*InferenceStats)
}

// ResetModel clears statistics for a specific model
func (t *Tracker) ResetModel(modelID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.stats, modelID)
}
