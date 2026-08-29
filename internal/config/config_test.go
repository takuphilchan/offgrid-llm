package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromFilePreservesExplicitFalseBooleans(t *testing.T) {
	for _, key := range []string{
		"OFFGRID_USE_MMAP", "OFFGRID_FLASH_ATTENTION", "OFFGRID_CONT_BATCHING",
		"OFFGRID_ADAPTIVE_CONTEXT", "OFFGRID_PREWARM_MODELS", "OFFGRID_SMART_MLOCK",
		"OFFGRID_PROTECT_DEFAULT", "OFFGRID_FAST_SWITCH",
	} {
		t.Setenv(key, "")
	}

	path := filepath.Join(t.TempDir(), "config.yaml")
	data := []byte("use_mmap: false\nflash_attention: false\ncont_batching: false\nadaptive_context: false\nprewarm_models: false\nsmart_mlock: false\nprotect_default: false\nfast_switch_mode: false\n")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.UseMmap || cfg.FlashAttention || cfg.ContBatching || cfg.AdaptiveContext ||
		cfg.PrewarmModels || cfg.SmartMlock || cfg.ProtectDefault || cfg.FastSwitchMode {
		t.Fatal("explicit false boolean was overwritten by a default")
	}
}

func TestLoadFromFileKeepsBooleanDefaultsWhenOmitted(t *testing.T) {
	for _, key := range []string{
		"OFFGRID_USE_MMAP", "OFFGRID_FLASH_ATTENTION", "OFFGRID_CONT_BATCHING",
		"OFFGRID_ADAPTIVE_CONTEXT", "OFFGRID_PREWARM_MODELS", "OFFGRID_SMART_MLOCK",
		"OFFGRID_PROTECT_DEFAULT", "OFFGRID_FAST_SWITCH",
	} {
		t.Setenv(key, "")
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("server_port: 12000\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.UseMmap || !cfg.FlashAttention || !cfg.ContBatching || !cfg.AdaptiveContext ||
		!cfg.PrewarmModels || !cfg.SmartMlock || !cfg.ProtectDefault || !cfg.FastSwitchMode {
		t.Fatal("omitted boolean did not retain its default")
	}
}

func TestLoadConfigHonorsDataDirectory(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "state")
	t.Setenv("OFFGRID_DATA_DIR", dataDir)
	cfg := LoadConfig()
	if cfg.DataDir != dataDir {
		t.Fatalf("DataDir = %q, want %q", cfg.DataDir, dataDir)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Fatalf("data path is not a directory: %s", dataDir)
	}
}

func TestImporterRejectsTraversalPaths(t *testing.T) {
	root := t.TempDir()
	importer := NewImporter(root, filepath.Join(root, "config.yaml"), false)
	for _, name := range []string{
		"config/../outside",
		"templates/../../outside",
		"templates/..\\outside",
		"prompts/C:/outside",
		"prompts/\\\\server\\share",
		"templates/prompt.yaml:stream",
	} {
		if target, err := importer.resolveTargetPath(name); err == nil || target != "" {
			t.Fatalf("resolveTargetPath(%q) = %q, %v; want rejection", name, target, err)
		}
	}
	target, err := importer.resolveTargetPath("templates/nested/prompt.yaml")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "templates", "nested", "prompt.yaml")
	if target != want {
		t.Fatalf("safe target = %q, want %q", target, want)
	}
}
