package inference

// LegacyTimingSnapshot is aggregate timing for completed, scheduler-admitted
// llama requests. It is diagnostic data, not a device-memory qualification.
type LegacyTimingSnapshot struct {
	Samples    uint64
	Load       int64 // Nanoseconds, preserving sub-millisecond fixture precision.
	Queue      int64
	Generation int64
	Cancel     int64
	Unload     int64
}

// RecordLegacyTiming is called once when a legacy generation lease releases.
// Failed and cancelled generations are included because they occupied the
// same scheduler resources until their engine call returned.
func (s *ProviderScheduler) RecordLegacyTiming(timing ProviderTiming) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.legacySamples++
	s.legacyTimings.Load += timing.Load
	s.legacyTimings.Queue += timing.Queue
	s.legacyTimings.Generation += timing.Generation
	s.legacyTimings.Cancel += timing.Cancel
	s.legacyTimings.Unload += timing.Unload
	s.mu.Unlock()
}

func (s *ProviderScheduler) LegacyTimingSnapshot() LegacyTimingSnapshot {
	if s == nil {
		return LegacyTimingSnapshot{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return LegacyTimingSnapshot{
		Samples:    s.legacySamples,
		Load:       int64(s.legacyTimings.Load),
		Queue:      int64(s.legacyTimings.Queue),
		Generation: int64(s.legacyTimings.Generation),
		Cancel:     int64(s.legacyTimings.Cancel),
		Unload:     int64(s.legacyTimings.Unload),
	}
}
