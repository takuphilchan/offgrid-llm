package degradation

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/shirou/gopsutil/v3/mem"
)

func TestPressureAdmissionAllowsBoundedWorkAndRecoversAfterRelease(t *testing.T) {
	for _, level := range []DegradationLevel{LevelNormal, LevelReduced, LevelMinimal, LevelEmergency} {
		m := NewManager(DefaultConfig())
		atomic.StoreInt32(&m.currentLevel, int32(level))
		var admitted atomic.Int64
		var workers sync.WaitGroup
		for i := 0; i < 100; i++ {
			workers.Add(1)
			go func() {
				defer workers.Done()
				if m.RequestStart() {
					admitted.Add(1)
				}
			}()
		}
		workers.Wait()
		if admitted.Load() != int64(m.MaxConcurrentRequests()) {
			t.Fatalf("level %v admitted %d, expected %d", level, admitted.Load(), m.MaxConcurrentRequests())
		}
		m.RequestEnd()
		if !m.RequestStart() || m.RequestStart() {
			t.Fatalf("level %v did not release exactly one slot", level)
		}
		for i := int64(0); i < admitted.Load(); i++ {
			m.RequestEnd()
		}
		if atomic.LoadInt64(&m.activeRequests) != 0 {
			t.Fatal("request accounting leaked")
		}
		m.Stop()
	}
}

func TestPressureReductionDoesNotCancelExistingWork(t *testing.T) {
	m := NewManager(DefaultConfig())
	defer m.Stop()
	for i := 0; i < 3; i++ {
		if !m.RequestStart() {
			t.Fatal("initial admission failed")
		}
	}
	atomic.StoreInt32(&m.currentLevel, int32(LevelEmergency))
	if m.RequestStart() {
		t.Fatal("admitted above reduced capacity")
	}
	for i := 0; i < 3; i++ {
		m.RequestEnd()
	}
	if !m.RequestStart() {
		t.Fatal("idle emergency slot remained unavailable")
	}
	m.RequestEnd()
}

func TestMemoryPressureUsesOSCapacityNotGoHeapOccupancy(t *testing.T) {
	m := NewManager(DefaultConfig())
	defer m.Stop()
	for _, test := range []struct {
		available uint64
		level     DegradationLevel
	}{{80, LevelNormal}, {25, LevelReduced}, {10, LevelMinimal}, {2, LevelEmergency}} {
		m.memorySample = func() (*mem.VirtualMemoryStat, error) {
			return &mem.VirtualMemoryStat{Total: 100 << 20, Available: test.available << 20}, nil
		}
		got := m.collectMetrics()
		if !got.MemoryMeasured || m.calculateLevel(got) != test.level {
			t.Fatalf("available=%d metrics=%+v level=%v", test.available, got, m.calculateLevel(got))
		}
	}
	m.metrics = ResourceMetrics{MemoryPercent: 96, MemoryTotalMB: 100, MemoryUsedMB: 96}
	m.memorySample = func() (*mem.VirtualMemoryStat, error) { return nil, errors.New("unavailable") }
	got := m.collectMetrics()
	if got.MemoryMeasured || m.calculateLevel(got) != LevelEmergency {
		t.Fatalf("missing sample falsely recovered: %+v", got)
	}
}

func TestPressureDoesNotRewriteContext(t *testing.T) {
	m := NewManager(DefaultConfig())
	defer m.Stop()
	for _, level := range []DegradationLevel{LevelNormal, LevelReduced, LevelMinimal, LevelEmergency} {
		atomic.StoreInt32(&m.currentLevel, int32(level))
		if m.MaxContextSize() != 1048576 {
			t.Fatalf("level %v shrank context", level)
		}
	}
	if m.MaxConcurrentRequests() != 1 {
		t.Fatal("emergency concurrency guard lost")
	}
}
