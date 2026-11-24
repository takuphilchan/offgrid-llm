package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestONNXRuntimeConfigurationRequiresExplicitEnrollment(t *testing.T) {
	t.Setenv("OFFGRID_ONNX_EMBEDDING_RUNTIME_FILE", "")
	if LoadConfig().ONNXEmbeddingRuntimeFile != "" {
		t.Fatal("ONNX enabled by default")
	}
	enrollment := filepath.Join(t.TempDir(), "runtime.json")
	t.Setenv("OFFGRID_ONNX_EMBEDDING_RUNTIME_FILE", enrollment)
	if LoadConfig().ONNXEmbeddingRuntimeFile != enrollment {
		t.Fatal("environment enrollment lost")
	}
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"onnx_embedding_runtime_file":""}`), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFromFile(path)
	if err != nil || cfg.ONNXEmbeddingRuntimeFile != "" {
		t.Fatalf("explicit file override lost: %v", err)
	}
	cfg.ONNXEmbeddingRuntimeFile = enrollment
	if err := cfg.SaveToFile(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadFromFile(path)
	if err != nil || loaded.ONNXEmbeddingRuntimeFile != enrollment {
		t.Fatalf("enrollment round trip lost: %v", err)
	}
}
