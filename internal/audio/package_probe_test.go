package audio

import (
	"context"
	"path/filepath"
	"testing"
)

func TestFailedSpeechProbeDoesNotCacheIncompatibility(t *testing.T) {
	t.Setenv("OFFGRID_SPEECH_PYTHON", filepath.Join(t.TempDir(), "missing-python"))
	p := &PackageRuntime{}
	if got := p.pythonModules(context.Background()); got != nil {
		t.Fatal("missing runtime reported available", got)
	}
	if !p.probeAt.IsZero() {
		t.Fatal("failed probe poisoned the readiness cache")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := p.pythonModules(ctx); got != nil {
		t.Fatal("cancelled probe reported available", got)
	}
	if !p.probeAt.IsZero() {
		t.Fatal("cancellation was cached as incompatibility")
	}
}
