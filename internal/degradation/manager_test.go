package degradation

import (
	"errors"
	"sync/atomic"
	"testing"

	"github.com/shirou/gopsutil/v3/mem"
)

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
