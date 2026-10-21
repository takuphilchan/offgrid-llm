package inference

import "time"

// EmbeddingTimingSnapshot covers completed scheduler-admitted embedding loads
// and generations. It does not claim measured device memory or free capacity.
type EmbeddingTimingSnapshot struct {
	Samples    uint64
	Load       time.Duration
	Queue      time.Duration
	Generation time.Duration
	Cancel     time.Duration
	Unload     time.Duration
}

func (s *ProviderScheduler) recordEmbeddingTiming(timing ProviderTiming) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.embeddingSamples++
	s.embeddingTimings.Load += timing.Load
	s.embeddingTimings.Queue += timing.Queue
	s.embeddingTimings.Generation += timing.Generation
	s.embeddingTimings.Cancel += timing.Cancel
	s.embeddingTimings.Unload += timing.Unload
	s.mu.Unlock()
}

func (s *ProviderScheduler) EmbeddingTimingSnapshot() EmbeddingTimingSnapshot {
	if s == nil {
		return EmbeddingTimingSnapshot{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return EmbeddingTimingSnapshot{
		Samples:    s.embeddingSamples,
		Load:       s.embeddingTimings.Load,
		Queue:      s.embeddingTimings.Queue,
		Generation: s.embeddingTimings.Generation,
		Cancel:     s.embeddingTimings.Cancel,
		Unload:     s.embeddingTimings.Unload,
	}
}
