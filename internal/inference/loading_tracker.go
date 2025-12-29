// Package inference provides model loading progress tracking for better UX.
// Enables real-time loading progress feedback and predictive pre-warming.
package inference

import (
	"sync"
	"time"
)

// LoadingPhase represents the current phase of model loading
type LoadingPhase string

const (
	PhaseIdle      LoadingPhase = "idle"
	PhaseUnloading LoadingPhase = "unloading"
	PhaseStarting  LoadingPhase = "starting"
	PhaseLoading   LoadingPhase = "loading"
	PhaseWarmup    LoadingPhase = "warmup"
	PhaseReady     LoadingPhase = "ready"
	PhaseFailed    LoadingPhase = "failed"
)

// LoadingProgress tracks the current state of model loading
type LoadingProgress struct {
	ModelID     string       `json:"model_id"`
	Phase       LoadingPhase `json:"phase"`
	Progress    int          `json:"progress"` // 0-100 percentage
	Message     string       `json:"message"`  // Human-readable status
	StartedAt   time.Time    `json:"started_at"`
	ElapsedMS   int64        `json:"elapsed_ms"`
	EstimatedMS int64        `json:"estimated_ms"` // Estimated total time
	Error       string       `json:"error,omitempty"`
	IsWarm      bool         `json:"is_warm"` // Was model in page cache
	SizeMB      int64        `json:"size_mb"` // Model size
}

// ModelHistory tracks recent model usage for predictive warming
type ModelHistory struct {
	ModelID    string    `json:"model_id"`
	UsedAt     time.Time `json:"used_at"`
	UsageCount int       `json:"usage_count"`
	AvgLoadMS  int64     `json:"avg_load_ms"`
}

// LoadingTracker provides real-time model loading progress and predictive warming
type LoadingTracker struct {
	current     *LoadingProgress
	history     map[string]*ModelHistory
	loadTimes   map[string][]int64 // Historical load times per model
	subscribers []chan LoadingProgress
	mu          sync.RWMutex
}

// NewLoadingTracker creates a new loading progress tracker
func NewLoadingTracker() *LoadingTracker {
	return &LoadingTracker{
		current:   &LoadingProgress{Phase: PhaseIdle},
		history:   make(map[string]*ModelHistory),
		loadTimes: make(map[string][]int64),
	}
}

// StartLoading begins tracking a model load operation
func (lt *LoadingTracker) StartLoading(modelID string, sizeMB int64, isWarm bool) {
	lt.mu.Lock()
	defer lt.mu.Unlock()

	// Estimate load time based on history and model size
	estimatedMS := lt.estimateLoadTime(modelID, sizeMB, isWarm)

	lt.current = &LoadingProgress{
		ModelID:     modelID,
		Phase:       PhaseStarting,
		Progress:    0,
		Message:     "Starting model server...",
		StartedAt:   time.Now(),
		ElapsedMS:   0,
		EstimatedMS: estimatedMS,
		IsWarm:      isWarm,
		SizeMB:      sizeMB,
	}

	lt.notify()
}

// UpdatePhase updates the current loading phase
func (lt *LoadingTracker) UpdatePhase(phase LoadingPhase, progress int, message string) {
	lt.mu.Lock()
	defer lt.mu.Unlock()

	if lt.current == nil {
		return
	}

	lt.current.Phase = phase
	lt.current.Progress = progress
	lt.current.Message = message
	lt.current.ElapsedMS = time.Since(lt.current.StartedAt).Milliseconds()

	lt.notify()
}

// Complete marks the loading as complete and records timing
func (lt *LoadingTracker) Complete(modelID string, success bool, errorMsg string) {
	lt.mu.Lock()
	defer lt.mu.Unlock()

	if lt.current == nil || lt.current.ModelID != modelID {
		return
	}

	loadTime := time.Since(lt.current.StartedAt).Milliseconds()

	if success {
		lt.current.Phase = PhaseReady
		lt.current.Progress = 100
		lt.current.Message = "Model ready"

		// Record load time for future estimates
		lt.recordLoadTime(modelID, loadTime)

		// Update usage history
		lt.updateHistory(modelID, loadTime)
	} else {
		lt.current.Phase = PhaseFailed
		lt.current.Error = errorMsg
		lt.current.Message = "Loading failed"
	}

	lt.current.ElapsedMS = loadTime
	lt.notify()
}

// GetProgress returns the current loading progress
func (lt *LoadingTracker) GetProgress() *LoadingProgress {
	lt.mu.RLock()
	defer lt.mu.RUnlock()

	if lt.current == nil {
		return &LoadingProgress{Phase: PhaseIdle}
	}

	// Update elapsed time
	progress := *lt.current
	progress.ElapsedMS = time.Since(lt.current.StartedAt).Milliseconds()

	return &progress
}

// GetRecentModels returns recently used models for predictive warming
func (lt *LoadingTracker) GetRecentModels(limit int) []ModelHistory {
	lt.mu.RLock()
	defer lt.mu.RUnlock()

	recent := make([]ModelHistory, 0, len(lt.history))
	for _, h := range lt.history {
		recent = append(recent, *h)
	}

	// Sort by recency and usage
	// (In production, use proper sorting)
	if len(recent) > limit {
		recent = recent[:limit]
	}

	return recent
}

// ShouldPrewarm returns models that should be pre-warmed based on usage patterns
func (lt *LoadingTracker) ShouldPrewarm(currentModel string) []string {
	lt.mu.RLock()
	defer lt.mu.RUnlock()

	candidates := make([]string, 0)
	now := time.Now()

	for modelID, h := range lt.history {
		if modelID == currentModel {
			continue
		}
		// Pre-warm if used recently (within 24 hours) and frequently (3+ times)
		if now.Sub(h.UsedAt) < 24*time.Hour && h.UsageCount >= 3 {
			candidates = append(candidates, modelID)
		}
	}

	return candidates
}

// estimateLoadTime estimates how long a model will take to load
func (lt *LoadingTracker) estimateLoadTime(modelID string, sizeMB int64, isWarm bool) int64 {
	// Check historical data
	if times, ok := lt.loadTimes[modelID]; ok && len(times) > 0 {
		var sum int64
		for _, t := range times {
			sum += t
		}
		return sum / int64(len(times))
	}

	// Estimate based on size and warm status
	// Warm models load ~5x faster due to page cache
	if isWarm {
		return sizeMB * 5 // ~5ms per MB when warm
	}
	return sizeMB * 25 // ~25ms per MB when cold (disk read)
}

// recordLoadTime saves a load time for future estimates
func (lt *LoadingTracker) recordLoadTime(modelID string, loadTimeMS int64) {
	times := lt.loadTimes[modelID]
	times = append(times, loadTimeMS)

	// Keep last 5 load times
	if len(times) > 5 {
		times = times[len(times)-5:]
	}
	lt.loadTimes[modelID] = times
}

// updateHistory updates the usage history for a model
func (lt *LoadingTracker) updateHistory(modelID string, loadTimeMS int64) {
	h, exists := lt.history[modelID]
	if !exists {
		h = &ModelHistory{
			ModelID:    modelID,
			UsageCount: 0,
			AvgLoadMS:  loadTimeMS,
		}
		lt.history[modelID] = h
	}

	h.UsedAt = time.Now()
	h.UsageCount++
	// Running average
	h.AvgLoadMS = (h.AvgLoadMS*int64(h.UsageCount-1) + loadTimeMS) / int64(h.UsageCount)
}

// Subscribe returns a channel for real-time progress updates
func (lt *LoadingTracker) Subscribe() <-chan LoadingProgress {
	lt.mu.Lock()
	defer lt.mu.Unlock()

	ch := make(chan LoadingProgress, 10)
	lt.subscribers = append(lt.subscribers, ch)
	return ch
}

// Unsubscribe removes a subscriber
func (lt *LoadingTracker) Unsubscribe(ch <-chan LoadingProgress) {
	lt.mu.Lock()
	defer lt.mu.Unlock()

	for i, sub := range lt.subscribers {
		if sub == ch {
			lt.subscribers = append(lt.subscribers[:i], lt.subscribers[i+1:]...)
			close(sub)
			break
		}
	}
}

// notify sends progress to all subscribers
func (lt *LoadingTracker) notify() {
	if lt.current == nil {
		return
	}

	progress := *lt.current
	progress.ElapsedMS = time.Since(lt.current.StartedAt).Milliseconds()

	for _, ch := range lt.subscribers {
		select {
		case ch <- progress:
		default:
			// Skip if channel is full
		}
	}
}

// Reset clears the current loading state
func (lt *LoadingTracker) Reset() {
	lt.mu.Lock()
	defer lt.mu.Unlock()

	lt.current = &LoadingProgress{Phase: PhaseIdle}
	lt.notify()
}
