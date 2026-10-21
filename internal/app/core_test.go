package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/config"
)

func disposableCoreConfig(t *testing.T) *config.Config {
	t.Helper()
	root := t.TempDir()
	cfg := config.LoadConfig()
	cfg.ModelsDir = filepath.Join(root, "models")
	cfg.DataDir = filepath.Join(root, "state")
	cfg.UseMockEngine = true
	cfg.InferenceSlots = 1
	return cfg
}

func TestNewCoreOwnsAndReleasesDisposableWorkspace(t *testing.T) {
	cfg := disposableCoreConfig(t)
	first, err := newCore(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if first.workspace == "" || first.registry == nil || first.monitor == nil {
		t.Fatalf("core missing required state: %+v", first)
	}
	if err := first.initProviderScheduler(cfg); err != nil {
		t.Fatal(err)
	}
	if err := first.initModelCache(cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := newCore(cfg); err == nil {
		t.Fatal("second core acquired an already-owned workspace")
	}
	if err := first.owned.close(); err != nil {
		t.Fatal(err)
	}
	second, err := newCore(cfg)
	if err != nil {
		t.Fatalf("workspace was not released: %v", err)
	}
	if second.workspace != first.workspace {
		t.Fatalf("workspace identity changed: first=%q second=%q", first.workspace, second.workspace)
	}
	if err := second.owned.close(); err != nil {
		t.Fatal(err)
	}
}

func TestNewCoreRejectsInvalidConfigBeforeWorkspaceOwnership(t *testing.T) {
	cfg := disposableCoreConfig(t)
	cfg.InferenceSlots = -1
	if _, err := newCore(cfg); err == nil {
		t.Fatal("invalid inference slots accepted")
	}
	if _, err := os.Stat(cfg.DataDir); !os.IsNotExist(err) {
		t.Fatalf("invalid startup created workspace: %v", err)
	}
	cfg.InferenceSlots = 1
	core, err := newCore(cfg)
	if err != nil {
		t.Fatalf("invalid config left workspace owned: %v", err)
	}
	if err := core.owned.close(); err != nil {
		t.Fatal(err)
	}
}

func TestNewCoreLayoutFailureReleasesWorkspace(t *testing.T) {
	cfg := disposableCoreConfig(t)
	if err := os.MkdirAll(filepath.Join(cfg.DataDir, ".layout-version"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := newCore(cfg); err == nil {
		t.Fatal("layout marker directory was accepted")
	}
	if err := os.Remove(filepath.Join(cfg.DataDir, ".layout-version")); err != nil {
		t.Fatal(err)
	}
	core, err := newCore(cfg)
	if err != nil {
		t.Fatalf("layout failure retained workspace ownership: %v", err)
	}
	if err := core.owned.close(); err != nil {
		t.Fatal(err)
	}
}
