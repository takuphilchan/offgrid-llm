package inference

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestBinaryManager_GetLlamaServer_Local(t *testing.T) {
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

	// Test finding it
	foundPath, err := bm.GetLlamaServer()
	if err != nil {
		t.Fatalf("Failed to find local binary: %v", err)
	}

	if foundPath != binaryPath {
		t.Errorf("Expected path %s, got %s", binaryPath, foundPath)
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
