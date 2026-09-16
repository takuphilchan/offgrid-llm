package server

import (
	"errors"
	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/storage"
	"os"
	"path/filepath"
	"testing"
)

func TestServerOwnsWorkspaceUntilClosed(t *testing.T) {
	first := newTestServer(t)
	cfg := *first.config
	second := NewWithConfig(&cfg)
	if err := second.Start(); !errors.Is(err, storage.ErrWorkspaceInUse) {
		t.Fatalf("second service: %v", err)
	}
	if second.ragEngine != nil || second.agentManager != nil {
		t.Fatal("second service initialized stores before ownership")
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	third := NewWithConfig(&cfg)
	defer third.Close()
	if third.startupErr != nil {
		t.Fatal(third.startupErr)
	}
}

func TestServerFailsClosedOnLayoutError(t *testing.T) {
	root := t.TempDir()
	cfg := config.LoadConfig()
	cfg.DataDir = filepath.Join(root, "current")
	cfg.ModelsDir = filepath.Join(root, "legacy", "models")
	cfg.UseMockEngine = true
	if err := os.MkdirAll(filepath.Join(root, "legacy", "data"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cfg.DataDir, 0700); err != nil {
		t.Fatal(err)
	}
	// A directory where the layout marker must be written is an actionable
	// initialization error, not permission to start on partially migrated data.
	if err := os.Mkdir(filepath.Join(cfg.DataDir, ".layout-version"), 0700); err != nil {
		t.Fatal(err)
	}
	server := NewWithConfig(cfg)
	if server.Start() == nil {
		t.Fatal("started after failed layout initialization")
	}
	owner, err := storage.AcquireOwnership(cfg.DataDir)
	if err != nil {
		t.Fatalf("failed initialization leaked ownership: %v", err)
	}
	owner.Close()
}
