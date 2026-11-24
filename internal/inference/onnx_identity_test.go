package inference

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestONNXEmbeddingIdentitySurvivesRelocationButNotContractChanges(t *testing.T) {
	root := t.TempDir()
	config := ONNXEmbeddingRuntimeConfig{SchemaVersion: 1, RunnerRevision: "worker-v1", RuntimeDirectory: filepath.Join(root, "runtime"), StateDirectory: filepath.Join(root, "state"), Python: "python", Script: "worker.py", RuntimeFiles: map[string]string{"python": "sha256:" + strings.Repeat("a", 64), "worker.py": "sha256:" + strings.Repeat("b", 64)}, DependencyVersions: map[string]string{"onnxruntime": "1.0", "onnx": "1.0", "numpy": "1.0", "tokenizers": "1.0"}, NamespaceLauncher: filepath.Join(root, "unshare"), NamespaceLauncherSHA256: "sha256:" + strings.Repeat("c", 64)}
	descriptor := onnxDescriptorFixture()
	identity := onnxEmbeddingIdentity(descriptor, config)
	moved := config
	moved.RuntimeDirectory = filepath.Join(root, "restored", "runtime")
	moved.StateDirectory = filepath.Join(root, "restored", "state")
	moved.NamespaceLauncher = filepath.Join(root, "other-os-prefix", "unshare")
	if got := onnxEmbeddingIdentity(descriptor, moved); got != identity {
		t.Fatalf("byte-identical relocation invalidated Knowledge identity: %s != %s", got, identity)
	}
	encoded, _ := json.Marshal(config)
	for name, change := range map[string]func(*ProviderDescriptor, *ONNXEmbeddingRuntimeConfig){
		"model": func(d *ProviderDescriptor, _ *ONNXEmbeddingRuntimeConfig) {
			d.ModelDigest = "sha256:" + strings.Repeat("d", 64)
		},
		"preprocessing": func(d *ProviderDescriptor, _ *ONNXEmbeddingRuntimeConfig) {
			d.PreprocessDigest = "sha256:" + strings.Repeat("d", 64)
		},
		"runtime bytes": func(_ *ProviderDescriptor, c *ONNXEmbeddingRuntimeConfig) {
			c.RuntimeFiles["python"] = "sha256:" + strings.Repeat("d", 64)
		},
		"dependency": func(_ *ProviderDescriptor, c *ONNXEmbeddingRuntimeConfig) {
			c.DependencyVersions["onnxruntime"] = "2.0"
		},
		"worker selection": func(_ *ProviderDescriptor, c *ONNXEmbeddingRuntimeConfig) { c.Script = "other.py" },
		"runner revision":  func(_ *ProviderDescriptor, c *ONNXEmbeddingRuntimeConfig) { c.RunnerRevision = "worker-v2" },
		"namespace launcher bytes": func(_ *ProviderDescriptor, c *ONNXEmbeddingRuntimeConfig) {
			c.NamespaceLauncherSHA256 = "sha256:" + strings.Repeat("d", 64)
		},
	} {
		t.Run(name, func(t *testing.T) {
			var changed ONNXEmbeddingRuntimeConfig
			if err := json.Unmarshal(encoded, &changed); err != nil {
				t.Fatal(err)
			}
			changedDescriptor := descriptor
			change(&changedDescriptor, &changed)
			if onnxEmbeddingIdentity(changedDescriptor, changed) == identity {
				t.Fatal("changed execution contract retained stale Knowledge identity")
			}
		})
	}
}
