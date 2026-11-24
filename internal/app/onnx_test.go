package app

import (
	"encoding/json"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestONNXStartupEnrollmentUsesExistingCoreWithoutSelectingModel(t *testing.T) {
	cfg := disposableCoreConfig(t)
	root := t.TempDir()
	pin := "sha256:" + strings.Repeat("a", 64)
	enrollment := inference.ONNXEmbeddingRuntimeConfig{SchemaVersion: 1, RunnerRevision: "worker-v1", RuntimeDirectory: filepath.Join(root, "runtime"), StateDirectory: filepath.Join(root, "state"), Python: "python.exe", Script: "worker.py", RuntimeFiles: map[string]string{"python.exe": pin, "python._pth": pin, "worker.py": pin}, DependencyVersions: map[string]string{"onnx": "1.0", "onnxruntime": "1.0", "numpy": "1.0", "tokenizers": "1.0"}}
	raw, _ := json.Marshal(enrollment)
	cfg.ONNXEmbeddingRuntimeFile = filepath.Join(root, "runtime.json")
	if err := os.WriteFile(cfg.ONNXEmbeddingRuntimeFile, raw, 0600); err != nil {
		t.Fatal(err)
	}
	core, err := newCore(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer core.owned.close()
	if err := core.initProviderScheduler(cfg); err != nil {
		t.Fatal(err)
	}
	if core.embedding.IsLoaded() {
		t.Fatal("enrollment selected an embedding model")
	}
	if _, err := os.Stat(enrollment.RuntimeDirectory); !os.IsNotExist(err) {
		t.Fatal("enrollment provisioned runtime files")
	}
}
