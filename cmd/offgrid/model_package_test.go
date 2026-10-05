package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestManagedSpeechPackageCannotLaunchLlama(t *testing.T) {
	// The guard must run before any binary discovery, download, or process start.
	p := filepath.Join(t.TempDir(), ".packages", "whisper", "rev-1", "weights.bin")
	if err := startLlamaServerInBackground(p); err == nil || !strings.Contains(err.Error(), "speech packages") {
		t.Fatalf("managed speech path was not rejected: %v", err)
	}
}
