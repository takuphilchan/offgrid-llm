package resource

import (
	"testing"
)

func TestNewGPUMonitor(t *testing.T) {
	monitor := NewGPUMonitor()
	if monitor == nil {
		t.Fatal("NewGPUMonitor returned nil")
	}

	// Should have attempted detection
	gpuType := monitor.GetGPUType()
	t.Logf("Detected GPU type: %s", gpuType)

	// Check availability
	isAvailable := monitor.IsAvailable()
	t.Logf("GPU available: %v", isAvailable)

	if isAvailable {
		// If GPU is available, we should have info
		totalVRAM := monitor.GetTotalVRAM()
		t.Logf("Total VRAM: %d MB", totalVRAM)

		freeVRAM := monitor.GetFreeVRAM()
		t.Logf("Free VRAM: %d MB", freeVRAM)

		if totalVRAM == 0 {
			t.Error("GPU available but total VRAM is 0")
		}

		// Get detailed GPU info
		gpus := monitor.GetGPUInfo()
		if len(gpus) == 0 {
			t.Error("GPU available but GetGPUInfo returned empty slice")
		}

		for i, gpu := range gpus {
			t.Logf("GPU %d:", i)
			t.Logf("  ID: %d", gpu.ID)
			t.Logf("  Name: %s", gpu.Name)
			t.Logf("  Memory Total: %d MB", gpu.MemoryTotal)
			t.Logf("  Memory Used: %d MB", gpu.MemoryUsed)
			t.Logf("  Memory Free: %d MB", gpu.MemoryFree)
			t.Logf("  Utilization: %.1f%%", gpu.Utilization)
			if gpu.Temperature > 0 {
				t.Logf("  Temperature: %d°C", gpu.Temperature)
			}

			// Validate data consistency (allow small rounding errors from nvidia-smi)
			calculatedTotal := gpu.MemoryUsed + gpu.MemoryFree
			diff := gpu.MemoryTotal - calculatedTotal
			if diff < 0 {
				diff = -diff
			}
			if diff > 100 { // Allow up to 100MB difference for rounding
				t.Errorf("Memory accounting inconsistent: %d != %d + %d (diff: %d MB)",
					gpu.MemoryTotal, gpu.MemoryUsed, gpu.MemoryFree, diff)
			}
		}
	} else {
		t.Log("No GPU detected (CPU-only mode) - this is normal on systems without NVIDIA/AMD GPUs")
	}
}

func TestHasEnoughVRAM(t *testing.T) {
	monitor := NewGPUMonitor()

	if !monitor.IsAvailable() {
		t.Skip("No GPU available, skipping VRAM test")
	}

	// Test with very small requirement (should pass)
	hasEnough, err := monitor.HasEnoughVRAM(1)
	if err != nil {
		t.Logf("1 MB test: %v", err)
	} else if hasEnough {
		t.Log("1 MB test: passed")
	}

	// Test with very large requirement (likely to fail)
	hasEnough, err = monitor.HasEnoughVRAM(1000000) // 1TB
	if err != nil {
		t.Logf("1TB test (expected to fail): %v", err)
	} else if !hasEnough {
		t.Log("1TB test: correctly rejected")
	}

	// Test with realistic model size (4GB)
	hasEnough, err = monitor.HasEnoughVRAM(4096)
	if err != nil {
		t.Logf("4GB model test: %v", err)
	} else if hasEnough {
		t.Log("4GB model test: passed - sufficient VRAM")
	}
}

func TestGetBestGPU(t *testing.T) {
	monitor := NewGPUMonitor()

	if !monitor.IsAvailable() {
		t.Skip("No GPU available, skipping best GPU test")
	}

	gpus := monitor.GetGPUInfo()
	if len(gpus) == 0 {
		t.Fatal("GPU available but no GPU info")
	}

	bestGPU, err := monitor.GetBestGPU()
	if err != nil {
		t.Fatalf("GetBestGPU failed: %v", err)
	}

	t.Logf("Best GPU: %s (ID: %d) with %d MB free",
		bestGPU.Name, bestGPU.ID, bestGPU.MemoryFree)

	// Verify it's actually the best
	for _, gpu := range gpus {
		if gpu.MemoryFree > bestGPU.MemoryFree {
			t.Errorf("GPU %d has more free memory (%d MB) than best GPU (%d MB)",
				gpu.ID, gpu.MemoryFree, bestGPU.MemoryFree)
		}
	}
}

func TestGPUInfoCaching(t *testing.T) {
	monitor := NewGPUMonitor()

	if !monitor.IsAvailable() {
		t.Skip("No GPU available, skipping cache test")
	}

	// First call
	info1 := monitor.GetGPUInfo()

	// Immediate second call (should use cache)
	info2 := monitor.GetGPUInfo()

	if len(info1) != len(info2) {
		t.Error("Cached GPU info has different length than fresh info")
	}
}

func BenchmarkGetGPUInfo(b *testing.B) {
	monitor := NewGPUMonitor()

	if !monitor.IsAvailable() {
		b.Skip("No GPU available")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = monitor.GetGPUInfo()
	}
}

func BenchmarkHasEnoughVRAM(b *testing.B) {
	monitor := NewGPUMonitor()

	if !monitor.IsAvailable() {
		b.Skip("No GPU available")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = monitor.HasEnoughVRAM(4096)
	}
}
