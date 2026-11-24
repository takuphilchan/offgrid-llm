package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInferenceEndpointConfigurationIsExplicitAndPreserved(t *testing.T) {
	t.Setenv("OFFGRID_INFERENCE_ENDPOINTS_FILE", "")
	if LoadConfig().InferenceEndpointsFile != "" {
		t.Fatal("endpoint enrollment enabled by default")
	}
	environmentPath := filepath.Join(t.TempDir(), "environment-endpoints.json")
	t.Setenv("OFFGRID_INFERENCE_ENDPOINTS_FILE", environmentPath)
	if got := LoadConfig().InferenceEndpointsFile; got != environmentPath {
		t.Fatalf("environment path lost: %q", got)
	}
	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(configPath, []byte(`{"inference_endpoints_file":""}`), 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadFromFile(configPath)
	if err != nil || loaded.InferenceEndpointsFile != "" {
		t.Fatalf("file did not override environment: %+v %v", loaded, err)
	}
	loaded.InferenceEndpointsFile = environmentPath
	if err = loaded.SaveToFile(configPath); err != nil {
		t.Fatal(err)
	}
	reloaded, err := LoadFromFile(configPath)
	if err != nil || reloaded.InferenceEndpointsFile != environmentPath {
		t.Fatalf("configured path not preserved: %+v %v", reloaded, err)
	}
}
