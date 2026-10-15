package inference

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestBinaryManager_GetLlamaServer_Local(t *testing.T) {
	t.Setenv("OFFGRID_ENABLE_GPU", "false")
	// Create temp bin dir
	tmpDir, err := os.MkdirTemp("", "offgrid_bin_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	bm := NewBinaryManager(tmpDir)

	// Create dummy binary
	binaryName := "llama-server"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binaryPath := filepath.Join(tmpDir, binaryName)

	if err := os.WriteFile(binaryPath, []byte("dummy"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OFFGRID_LLAMA_SERVER_PATH", binaryPath)

	// Test finding it
	foundPath, err := bm.GetLlamaServer()
	if err != nil {
		t.Fatalf("Failed to find local binary: %v", err)
	}

	if foundPath != binaryPath {
		t.Errorf("Expected path %s, got %s", binaryPath, foundPath)
	}
}

func TestBinaryManagerRejectsOldLocalBinary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "llama-server")
	if err := os.WriteFile(path, []byte("old binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if llamaServerSupportsRequiredFlags(path) {
		t.Fatal("non-executable or old llama-server was accepted")
	}
}

func TestBinaryManager_GetDownloadURL(t *testing.T) {
	bm := NewBinaryManager("/tmp")
	url, err := bm.getDownloadURL()
	if err != nil {
		// Might fail on unsupported platforms, which is fine
		t.Logf("getDownloadURL returned error (might be expected): %v", err)
	} else {
		t.Logf("Download URL: %s", url)
		if url == "" {
			t.Error("URL should not be empty")
		}
	}
}

func TestBinaryManagerHonorsExplicitPaths(t *testing.T) {
	customBin := t.TempDir()
	t.Setenv("OFFGRID_BIN_DIR", customBin)
	manager := NewBinaryManager(t.TempDir())
	if manager.binDir != customBin {
		t.Fatalf("binDir = %q, want %q", manager.binDir, customBin)
	}
	t.Setenv("OFFGRID_LLAMA_SERVER_PATH", filepath.Join(customBin, "missing-llama-server"))
	if _, err := manager.GetLlamaServer(); err == nil {
		t.Fatal("missing explicit llama-server path silently fell back to another binary")
	}
}

func TestGPUDeviceListRequiresActualDevice(t *testing.T) {
	for _, test := range []struct {
		text, kind string
		want       bool
	}{
		{"CUDA driver failure\nAvailable devices:\n  (none)\n", "nvidia", false},
		{"Available devices:\n  CUDA0: NVIDIA fixture (8192 MiB, 7000 MiB free)\n", "nvidia", true},
		{"Available devices:\n  CPU: processor\n", "nvidia", false},
		{"CUDA0: NVIDIA fixture\n", "nvidia", false},
		{"Available devices:\n  CUDA0: NVIDIA fixture\n", "amd", false},
		{"Available devices:\n  HIP1: AMD fixture\n", "amd", true},
		{"Available devices:\n  Vulkan0: fixture\n", "amd", true},
	} {
		if got := llamaDeviceListHasGPU(test.text, test.kind); got != test.want {
			t.Fatalf("%q/%s: %v", test.text, test.kind, got)
		}
	}
}

func TestCachedCPURuntimeCannotShadowInstalledGPU(t *testing.T) {
	manager := &BinaryManager{version: "b10516", binDir: t.TempDir(), hasNVIDIA: true}
	for _, path := range []string{manager.localPath(), manager.versionedPath()} {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("fixture, not executable"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	compatible := func(string) bool { return true }
	device := func(path, kind string) bool { return path == manager.localPath() && kind == "nvidia" }
	if got := manager.installedGPURuntime(compatible, device); got != manager.localPath() {
		t.Fatal("cached CPU won GPU selection", got)
	}
	if got := manager.installedGPURuntime(compatible, func(string, string) bool { return false }); got != "" {
		t.Fatal("CPU binary advertised as GPU", got)
	}
	t.Setenv("OFFGRID_ENABLE_GPU", "false")
	if manager.gpuRequested() {
		t.Fatal("CPU mode requested GPU")
	}
	manager.hasNVIDIA = false
	manager.SetGPUEnabled(true)
	t.Setenv("OFFGRID_ENABLE_GPU", "true")
	if manager.gpuRequested() {
		t.Fatal("no-device CPU fallback blocked")
	}
}

func TestEffectiveGPUConfigurationOverridesEnvironment(t *testing.T) {
	t.Setenv("OFFGRID_ENABLE_GPU", "false")
	manager := &BinaryManager{hasNVIDIA: true}
	manager.SetGPUEnabled(true)
	if runtime.GOOS != "darwin" && !manager.gpuRequested() {
		t.Fatal("configuration-file GPU opt-in was ignored")
	}
	t.Setenv("OFFGRID_ENABLE_GPU", "true")
	manager.SetGPUEnabled(false)
	if manager.gpuRequested() {
		t.Fatal("explicit CPU selection was overridden by environment")
	}
}

func TestModelCachePropagatesGPUPlacement(t *testing.T) {
	t.Setenv("OFFGRID_ENABLE_GPU", "false")
	for _, layers := range []int{0, 8} {
		cache := NewModelCache(1, layers, t.TempDir())
		cache.SetGPUEnabled(true)
		if !cache.binManager.gpuEnabled.Load() {
			t.Fatal("effective GPU placement was not propagated")
		}
		cache.SetGPUEnabled(false)
		if got := cache.binManager.gpuEnabled.Load(); got != (layers > 0) {
			t.Fatalf("layers %d: GPU selection = %v", layers, got)
		}
		cache.StopMonitor()
	}
}
