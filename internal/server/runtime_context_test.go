package server

import "testing"

func TestResolveContextWindowHonorsExplicitContext(t *testing.T) {
	got := resolveContextWindow(65536, false, 4096, 8192)
	if got != 65536 {
		t.Fatalf("got %d, want 65536", got)
	}
}

func TestResolveContextWindowAppliesAdaptiveLimits(t *testing.T) {
	got := resolveContextWindow(65536, true, 8192, 4096)
	if got != 4096 {
		t.Fatalf("got %d, want 4096", got)
	}
}

func TestResolveContextWindowUsesSafeDefault(t *testing.T) {
	got := resolveContextWindow(0, false)
	if got != 4096 {
		t.Fatalf("got %d, want 4096", got)
	}
}
