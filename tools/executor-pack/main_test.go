package main

import (
	"context"
	"testing"
)

func TestPackBuilderRequiresExplicitTargetAndSafeVersion(t *testing.T) {
	for _, args := range [][]string{nil, {"--output", "relative", "--version", "development"}, {"--output", t.TempDir(), "--version", "unsafe -X another=value"}, {"--output", t.TempDir(), "--version", "development", "--goos", "unknown"}} {
		if err := run(context.Background(), args); err == nil {
			t.Fatal("unsafe pack build accepted", args)
		}
	}
}
