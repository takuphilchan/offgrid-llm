package inference

import (
	"os"
	"os/exec"
	"testing"
)

func TestModelCacheReportsHostRSSWithoutClaimingDeviceMemory(t *testing.T) {
	cache := NewModelCache(2, 0, t.TempDir())
	cache.StopMonitor()
	current, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	scheduler, err := NewProviderScheduler(NewProviderRegistry(), 1)
	if err != nil {
		t.Fatal(err)
	}
	// The scheduler's startup attachment must observe cache-owned processes
	// without changing the established GGUF engine's execution path.
	if _, err := scheduler.AttachLlamaCache(cache, NewMockEngine()); err != nil {
		t.Fatal(err)
	}
	cache.mu.Lock()
	cache.instances["measured"] = &ModelInstance{ModelID: "measured", Cmd: &exec.Cmd{Process: current}, ProcessCreateMS: observedProcessCreateTime(current.Pid)}
	cache.instances["unknown"] = &ModelInstance{ModelID: "unknown"}
	cache.mu.Unlock()
	report := scheduler.MemorySnapshot()
	if report.LegacyUnmeasuredResidents != 2 || report.LegacyRSSMeasured != 1 || report.LegacyRSSUnmeasured != 1 || report.LegacyResidentRSSBytes <= 0 || len(report.Devices) != 0 {
		t.Fatalf("host RSS was reported as free or absent: %+v", report)
	}
	cache.mu.Lock()
	cache.instances["measured"].ProcessCreateMS++ // A reused PID is not the owned child.
	cache.mu.Unlock()
	stale := scheduler.MemorySnapshot()
	if stale.LegacyResidentRSSBytes != 0 || stale.LegacyRSSMeasured != 0 || stale.LegacyRSSUnmeasured != 2 {
		t.Fatalf("stale process identity contributed host RSS: %+v", stale)
	}
	cache.mu.Lock()
	clear(cache.instances) // Never let a fixture cache retire the test process.
	cache.mu.Unlock()
	if err := scheduler.Close(); err != nil {
		t.Fatal(err)
	}
}
