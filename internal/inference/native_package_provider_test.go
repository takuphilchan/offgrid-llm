package inference

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/models"
)

func TestNativeProviderRequiresTrustedAdapterAndReleasesFailedStart(t *testing.T) {
	store := models.NewPackageStore(t.TempDir())
	manifest := models.ModelPackageManifest{
		SchemaVersion: 1, ID: "native-test", Revision: "fixture-1", Name: "Synthetic bytes", Architecture: "transformers-text",
		ModelFamily: "qwen2.5", Runtime: models.PackageRuntime{Adapter: "transformers", Revision: "fixture-1"},
		Capabilities: []models.ModelCapability{models.CapabilityChat}, Languages: []string{"en"}, License: "CC0-1.0",
		Engine: models.EngineTransformers, Format: models.FormatSafetensors, Backend: "cpu", RunnerRevision: "fixture-1", AdapterRevision: "fixture-1", ContextLength: 4096,
	}
	for _, artifact := range []struct{ path, role string }{{"weights.safetensors", "weights"}, {"config.json", "config"}, {"tokenizer.json", "tokenizer"}} {
		data := []byte(artifact.role)
		digest := sha256.Sum256(data)
		manifest.Artifacts = append(manifest.Artifacts, models.PackageArtifact{Path: artifact.path, Role: artifact.role, Size: int64(len(data)), SHA256: hex.EncodeToString(digest[:]), License: "CC0-1.0"})
	}
	if _, err := store.Import(context.Background(), manifest, func(_ context.Context, artifact models.PackageArtifact) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(artifact.Role)), nil
	}); err != nil {
		t.Fatal(err)
	}
	adapters := models.NewAdapterRegistry()
	if _, err := StartNativePackageProvider(context.Background(), store, adapters, manifest.ID, manifest.Revision, "test", TrustedNativeWorker{}, nil); err == nil {
		t.Fatal("unregistered model family started")
	}
	if err := adapters.Register(models.ModelAdapterSpec{ID: manifest.ModelFamily, Revision: manifest.AdapterRevision, Engine: manifest.Engine, Formats: []string{manifest.Format}, Modalities: []string{"text"}, ToolCalling: "unqualified", Template: "chatml", Tokenization: "template_exact_ids", Streaming: "text_delta_v1", ToolEnvelope: "none", MaxQualifiedContext: 4096}); err != nil {
		t.Fatal(err)
	}
	missingExecutable := filepath.Join(t.TempDir(), "missing-python")
	missingScript := filepath.Join(t.TempDir(), "worker.py")
	worker := TrustedNativeWorker{Provider: ProviderTransformers, RunnerRevision: manifest.RunnerRevision, PythonPath: missingExecutable, PythonSHA256: "sha256:" + strings.Repeat("a", 64), ScriptPath: missingScript, ScriptSHA256: "sha256:" + strings.Repeat("b", 64), DependencyVersions: map[string]string{"torch": "1.0", "transformers": "1.0", "safetensors": "1.0"}}
	if _, err := StartNativePackageProvider(context.Background(), store, adapters, manifest.ID, manifest.Revision, "test", worker, nil); err == nil {
		t.Fatal("missing trusted worker executable was accepted")
	}
	if err := store.Remove(manifest.ID, manifest.Revision); err != nil {
		t.Fatalf("failed provider startup leaked package lease: %v", err)
	}
}

func TestTrustedNativeWorkerBindsRunnerAndFilesBeforeLaunch(t *testing.T) {
	root := t.TempDir()
	python := filepath.Join(root, "python")
	script := filepath.Join(root, "worker.py")
	for path, content := range map[string]string{python: "trusted interpreter fixture", script: "trusted worker fixture"} {
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	digest := func(path string) string {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		return "sha256:" + hex.EncodeToString(sum[:])
	}
	lease := &models.NativeTextLease{Engine: models.EngineTransformers, RunnerRevision: "reviewed-1"}
	worker := TrustedNativeWorker{Provider: ProviderTransformers, RunnerRevision: "reviewed-1", PythonPath: python, PythonSHA256: digest(python), ScriptPath: script, ScriptSHA256: digest(script), DependencyVersions: map[string]string{"torch": "1.0", "transformers": "1.0", "safetensors": "1.0"}}
	if err := worker.Validate(context.Background(), lease); err != nil {
		t.Fatal(err)
	}
	wrong := worker
	wrong.RunnerRevision = "other"
	if err := wrong.Validate(context.Background(), lease); err == nil {
		t.Fatal("different runner revision was accepted")
	}
	if err := os.WriteFile(script, []byte("changed worker"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := worker.Validate(context.Background(), lease); err == nil {
		t.Fatal("modified worker script was accepted")
	}
}

func TestNativeDependencyIdentityRequiresExactPinnedVersions(t *testing.T) {
	worker := TrustedNativeWorker{Provider: ProviderMLX, DependencyVersions: map[string]string{"mlx": "1.2.3", "mlx-lm": "0.28.0", "transformers": "5.7.0"}}
	first, err := worker.dependencyIdentity()
	if err != nil || !completeSHA256Digest(first) {
		t.Fatalf("valid dependency lock rejected: digest=%q error=%v", first, err)
	}
	worker.DependencyVersions["mlx-lm"] = "0.29.0"
	changed, err := worker.dependencyIdentity()
	if err != nil || changed == first {
		t.Fatalf("changed native library retained runtime identity: %q, %v", changed, err)
	}
	delete(worker.DependencyVersions, "transformers")
	if _, err := worker.dependencyIdentity(); err == nil {
		t.Fatal("missing required native library pin was accepted")
	}
	worker.DependencyVersions["transformers"] = "5.7.0"
	worker.DependencyVersions["../../untrusted"] = "1.0"
	if _, err := worker.dependencyIdentity(); err == nil {
		t.Fatal("invalid native library identity was accepted")
	}
}
