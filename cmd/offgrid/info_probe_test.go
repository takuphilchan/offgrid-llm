package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/config"
)

func TestInfoObservationDoesNotCreateMissingModelStorage(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing-models")
	snapshot, err := observeInfo(context.Background(), &config.Config{ModelsDir: missing}, "test")
	if err != nil || len(snapshot.Models) != 0 {
		t.Fatalf("snapshot=%+v err=%v", snapshot, err)
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("info created model storage: %v", err)
	}
}
