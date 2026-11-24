package inference

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestONNXRuntimeConfigurationStrictBoundsAndNoExecution(t *testing.T) {
	root := t.TempDir()
	pin := "sha256:" + strings.Repeat("a", 64)
	c := ONNXEmbeddingRuntimeConfig{SchemaVersion: 1, RunnerRevision: "worker-v1", RuntimeDirectory: filepath.Join(root, "runtime"), StateDirectory: filepath.Join(root, "state"), Python: "python.exe", Script: "worker.py", RuntimeFiles: map[string]string{"python.exe": pin, "python._pth": pin, "worker.py": pin}, DependencyVersions: map[string]string{"onnx": "1.0", "onnxruntime": "1.0", "numpy": "1.0", "tokenizers": "1.0"}}
	raw, _ := json.Marshal(c)
	decoded, err := DecodeONNXEmbeddingRuntimeConfig(strings.NewReader(string(raw)))
	if err != nil || decoded.RunnerRevision != c.RunnerRevision {
		t.Fatal(err)
	}
	path := filepath.Join(root, "runtime.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadONNXEmbeddingRuntimeConfig(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(c.RuntimeDirectory); !os.IsNotExist(err) {
		t.Fatal("configuration read created runtime")
	}
	for _, bad := range []string{strings.Replace(string(raw), `"schema_version":1`, `"schema_version":1,"schema_version":1`, 1), strings.Replace(string(raw), `"schema_version":1`, `"unknown":1,"schema_version":1`, 1), strings.Repeat(" ", 4<<20) + string(raw)} {
		if _, err := DecodeONNXEmbeddingRuntimeConfig(strings.NewReader(bad)); err == nil {
			t.Fatal("ambiguous/oversize configuration accepted")
		}
	}
	c.RuntimeFiles["../escape"] = pin
	if c.Validate() == nil {
		t.Fatal("runtime path escape accepted")
	}
}
