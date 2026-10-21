package inference

import (
	"errors"
	"math"
	"sort"
)

// ProviderDeviceMemory is last-observed resident usage, not current free
// capacity. A zero limit means no safe device budget was established.
type ProviderDeviceMemory struct {
	Backend             string
	Device              string
	MeasuredBytes       int64
	LimitBytes          int64
	ResidentCount       int
	UnmeasuredResidents int
}

type ProviderMemorySnapshot struct {
	Devices                      []ProviderDeviceMemory
	PendingTransitions           int   // Loads and tracked retirements not yet settled.
	LegacyUnmeasuredResidents    int   // Cache-owned llama device allocations remain unmeasured.
	EmbeddingUnmeasuredResidents int   // A loaded embedding runtime has no qualified device-memory sample.
	LegacyResidentRSSBytes       int64 // Measured host-process RSS, not GPU memory or admission capacity.
	LegacyRSSMeasured            int
	LegacyRSSUnmeasured          int
	Unresolved                   bool // A retired process or provider may still hold resources.
}

// LegacyProcessMemory separates host-process RSS from device capacity. A
// missing or exited process is counted as unmeasured, never as zero usage.
type LegacyProcessMemory struct {
	Residents     int
	RSSBytes      int64
	RSSMeasured   int
	RSSUnmeasured int
}

// accountResidentMemoryLocked refuses overlapping residency without measured
// device evidence. The current scheduler retires other models before loading;
// this check keeps that invariant explicit if concurrency is later enabled.
// The caller has already validated the provider's own resource report.
func (s *ProviderScheduler) accountResidentMemoryLocked(key string, report ProviderResources) error {
	usage := report.MemoryBytes
	limit := report.MemoryLimitBytes
	peers := 0
	for otherKey, other := range s.residentMemory {
		if otherKey == key || other.Backend != report.Backend || other.Device != report.Device {
			continue
		}
		peers++
		if !other.MemoryMeasured || !report.MemoryMeasured || other.MemoryLimitBytes == 0 || report.MemoryLimitBytes == 0 {
			return errors.New("concurrent provider residency has unmeasured device capacity")
		}
		if usage > math.MaxInt64-other.MemoryBytes {
			return errors.New("provider resident memory accounting overflow")
		}
		usage += other.MemoryBytes
		if other.MemoryLimitBytes < limit {
			limit = other.MemoryLimitBytes
		}
	}
	if peers > 0 && usage > limit {
		return errors.New("provider resident memory exceeds device limit")
	}
	if s.residentMemory == nil {
		s.residentMemory = make(map[string]ProviderResources)
	}
	s.residentMemory[key] = report
	return nil
}

// invalidateResidentMemory discards a sample taken before a failed generation.
// It never invents a new device identity or changes a worker retired during
// the call. A later admission must obtain a fresh validated resource report.
func (s *ProviderScheduler) invalidateResidentMemory(key string, runtime ProviderRuntime) (ProviderResources, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.resident[key] != runtime {
		return ProviderResources{}, false
	}
	prior, ok := s.residentMemory[key]
	if !ok {
		return ProviderResources{}, false
	}
	unknown := ProviderResources{Backend: prior.Backend, Device: prior.Device}
	s.residentMemory[key] = unknown
	return unknown, true
}

// MemorySnapshot reports only last-observed usage of known native residents.
// It excludes llama cache allocations and never turns unknown CPU/driver
// allocations into a false zero-memory claim.
func (s *ProviderScheduler) MemorySnapshot() ProviderMemorySnapshot {
	s.mu.Lock()
	observer := s.legacyResident
	memoryObserver := s.legacyMemory
	embeddingOwner := s.embeddingOwner
	legacyActive := s.legacyRuns
	pending := len(s.loading) + s.retiring
	snapshot := ProviderMemorySnapshot{PendingTransitions: pending, Unresolved: s.unloadErr != nil || s.closeErr != nil || pending != 0}
	if s.closed {
		select {
		case <-s.closeDone:
		default:
			snapshot.Unresolved = true
		}
	}
	devices := make(map[string]*ProviderDeviceMemory)
	for _, report := range s.residentMemory {
		key := report.Backend + "\x00" + report.Device
		device := devices[key]
		if device == nil {
			device = &ProviderDeviceMemory{Backend: report.Backend, Device: report.Device}
			devices[key] = device
		}
		device.ResidentCount++
		if report.MemoryMeasured {
			if device.MeasuredBytes > math.MaxInt64-report.MemoryBytes {
				snapshot.Unresolved = true
			} else {
				device.MeasuredBytes += report.MemoryBytes
			}
			if report.MemoryLimitBytes > 0 && (device.LimitBytes == 0 || report.MemoryLimitBytes < device.LimitBytes) {
				device.LimitBytes = report.MemoryLimitBytes
			}
		} else {
			device.UnmeasuredResidents++
		}
	}
	for _, device := range devices {
		snapshot.Devices = append(snapshot.Devices, *device)
	}
	sort.Slice(snapshot.Devices, func(i, j int) bool {
		if snapshot.Devices[i].Backend != snapshot.Devices[j].Backend {
			return snapshot.Devices[i].Backend < snapshot.Devices[j].Backend
		}
		return snapshot.Devices[i].Device < snapshot.Devices[j].Device
	})
	s.mu.Unlock()
	// Do not hold the scheduler lock while observing ModelCache: its monitor
	// acquires a scheduler lease before mutating the cache.
	if memoryObserver != nil {
		legacy := memoryObserver()
		snapshot.LegacyUnmeasuredResidents = legacy.Residents
		snapshot.LegacyResidentRSSBytes = legacy.RSSBytes
		snapshot.LegacyRSSMeasured = legacy.RSSMeasured
		snapshot.LegacyRSSUnmeasured = legacy.RSSUnmeasured
	} else if observer != nil {
		if count := observer(); count > 0 {
			snapshot.LegacyUnmeasuredResidents = count
		}
	}
	if embeddingOwner != nil && embeddingOwner.resident.Load() {
		snapshot.EmbeddingUnmeasuredResidents = 1
	}
	// An admitted legacy load may have started a child that has not yet been
	// published by its owner. Never portray that transition as settled zero
	// residency merely because the cache/embedding observer sees no child yet.
	if legacyActive > 0 && snapshot.LegacyUnmeasuredResidents == 0 && snapshot.EmbeddingUnmeasuredResidents == 0 {
		snapshot.Unresolved = true
	}
	return snapshot
}
