package inference

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/models"
)

func nativeRouteFixture(t *testing.T) (*models.PackageStore, *models.AdapterRegistry, models.ModelPackageManifest, TrustedNativeWorker) {
	t.Helper()
	store := models.NewPackageStore(t.TempDir())
	manifest := models.ModelPackageManifest{
		SchemaVersion: 1, ID: "route-test", Revision: "fixture-1", Name: "Isolated route fixture", Architecture: "transformers-text",
		ModelFamily: "qwen2.5", Runtime: models.PackageRuntime{Adapter: "transformers", Revision: "fixture-1"},
		Capabilities: []models.ModelCapability{models.CapabilityChat}, Languages: []string{"en"}, License: "CC0-1.0",
		Engine: models.EngineTransformers, Format: models.FormatSafetensors, Backend: "cpu", RunnerRevision: "fixture-1", AdapterRevision: "fixture-1", ContextLength: 4096,
	}
	template := sha256.Sum256([]byte("chatml"))
	manifest.TemplateDigest = hex.EncodeToString(template[:])
	for _, artifact := range []struct{ path, role string }{{"weights.safetensors", "weights"}, {"config.json", "config"}, {"tokenizer.json", "tokenizer"}} {
		data := []byte(artifact.role)
		sum := sha256.Sum256(data)
		manifest.Artifacts = append(manifest.Artifacts, models.PackageArtifact{Path: artifact.path, Role: artifact.role, Size: int64(len(data)), SHA256: hex.EncodeToString(sum[:]), License: "CC0-1.0"})
	}
	if _, err := store.Import(context.Background(), manifest, func(_ context.Context, artifact models.PackageArtifact) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(artifact.Role)), nil
	}); err != nil {
		t.Fatal(err)
	}
	adapters := models.NewAdapterRegistry()
	if err := adapters.Register(models.ModelAdapterSpec{ID: manifest.ModelFamily, Revision: manifest.AdapterRevision, Engine: manifest.Engine, Formats: []string{manifest.Format}, Modalities: []string{"text"}, ToolCalling: "unsupported", Template: "chatml", Tokenization: "template_exact_ids", Streaming: "text_delta_v1", ToolEnvelope: "none", MaxQualifiedContext: manifest.ContextLength}); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	worker := TrustedNativeWorker{Provider: ProviderTransformers, RunnerRevision: manifest.RunnerRevision, PythonPath: filepath.Join(root, "python"), ScriptPath: filepath.Join(root, "worker.py"), DependencyVersions: map[string]string{"torch": "1.0", "transformers": "1.0", "safetensors": "1.0"}}
	for _, path := range []string{worker.PythonPath, worker.ScriptPath} {
		data := []byte("trusted fixture: " + path)
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		if path == worker.PythonPath {
			worker.PythonSHA256 = "sha256:" + hex.EncodeToString(sum[:])
		} else {
			worker.ScriptSHA256 = "sha256:" + hex.EncodeToString(sum[:])
		}
	}
	return store, adapters, manifest, worker
}

func TestNativeRoutesRequireCurrentTrustedQualification(t *testing.T) {
	store, adapters, manifest, worker := nativeRouteFixture(t)
	workers := map[ProviderID]TrustedNativeWorker{ProviderTransformers: worker}
	denied, err := NewNativePackageRoutes(store, adapters, workers, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := denied.Select(context.Background(), manifest.ID, manifest.Revision); err == nil {
		t.Fatal("metadata-only native package became runnable")
	}
	allowed := true
	checks := 0
	routes, err := NewNativePackageRoutes(store, adapters, workers, func(context.Context, *models.NativeTextLease, TrustedNativeWorker) error {
		checks++
		if !allowed {
			return errors.New("qualification revoked")
		}
		return nil // isolated fixture only; not real hardware evidence
	})
	if err != nil {
		t.Fatal(err)
	}
	var opens int
	var fake *schedulerFake
	routes.open = func(_ context.Context, lease *models.NativeTextLease, _ TrustedNativeWorker, _ []string) (ProviderRuntime, error) {
		opens++
		fake = &schedulerFake{descriptor: nativeTextDescriptor(lease)}
		return fake, nil
	}
	registry := NewProviderRegistry()
	if err := routes.Register(registry); err != nil {
		t.Fatal(err)
	}
	descriptor, err := routes.Select(context.Background(), manifest.ID, manifest.Revision)
	if err != nil {
		t.Fatal(err)
	}
	allowed = false
	if _, err := registry.Open(context.Background(), descriptor); err == nil || opens != 0 {
		t.Fatalf("revoked qualification opened native worker: err=%v opens=%d", err, opens)
	}
	allowed = true
	runtime, err := registry.Open(context.Background(), descriptor)
	if err != nil {
		t.Fatal(err)
	}
	allowed = false
	request := ProviderRequest{RequestID: "after-revocation", CancellationID: "after-revocation", ModelDigest: descriptor.ModelDigest, Prompt: []byte(`{"messages":[]}`)}
	if err := runtime.Generate(context.Background(), request, func(ProviderEvent) error { return nil }); err == nil || fake.generates.Load() != 0 {
		t.Fatalf("resident runtime generated after qualification was revoked: %v", err)
	}
	if err := runtime.Health(context.Background()); err == nil {
		t.Fatal("resident runtime remained healthy after qualification revocation")
	}
	allowed = true
	if err := runtime.Close(); err != nil {
		t.Fatal(err)
	}
	if opens != 1 || checks < 3 {
		t.Fatalf("route skipped requalification: opens=%d checks=%d", opens, checks)
	}
	if err := os.WriteFile(worker.ScriptPath, []byte("modified script"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Open(context.Background(), descriptor); err == nil || opens != 1 {
		t.Fatalf("changed worker bytes opened native provider: err=%v opens=%d", err, opens)
	}
	if fake.generates.Load() != 0 {
		t.Fatal("fixture provider unexpectedly generated")
	}
}

func TestNativeRoutesPinDependencyVersionsAtInstallation(t *testing.T) {
	store, adapters, manifest, worker := nativeRouteFixture(t)
	routes, err := NewNativePackageRoutes(store, adapters, map[ProviderID]TrustedNativeWorker{ProviderTransformers: worker}, func(context.Context, *models.NativeTextLease, TrustedNativeWorker) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	worker.DependencyVersions["torch"] = "2.0"
	if _, err := routes.Select(context.Background(), manifest.ID, manifest.Revision); err != nil {
		t.Fatalf("caller mutated an installed worker's dependency lock: %v", err)
	}
	if got := routes.workers[ProviderTransformers].DependencyVersions["torch"]; got != "1.0" {
		t.Fatalf("installed native worker dependency changed to %q", got)
	}
}

func TestNativeRouteRetainsPackageLeaseWhenWorkerStopIsUncertain(t *testing.T) {
	for _, scenario := range []string{"resident_close", "factory_failure", "startup_uncertain", "identity_mismatch"} {
		t.Run(scenario, func(t *testing.T) {
			store, adapters, manifest, worker := nativeRouteFixture(t)
			routes, err := NewNativePackageRoutes(store, adapters, map[ProviderID]TrustedNativeWorker{ProviderTransformers: worker}, func(context.Context, *models.NativeTextLease, TrustedNativeWorker) error { return nil })
			if err != nil {
				t.Fatal(err)
			}
			routes.open = func(_ context.Context, lease *models.NativeTextLease, _ TrustedNativeWorker, _ []string) (ProviderRuntime, error) {
				fake := &schedulerFake{descriptor: nativeTextDescriptor(lease), closeErr: errors.New("termination not confirmed")}
				switch scenario {
				case "factory_failure":
					return fake, errors.New("factory failed after process start")
				case "startup_uncertain":
					return nil, errors.Join(errors.New("startup handshake failed"), errProviderStopUncertain)
				case "identity_mismatch":
					fake.descriptor.ModelDigest = "sha256:" + strings.Repeat("0", 64)
				}
				return fake, nil
			}
			registry := NewProviderRegistry()
			if err := routes.Register(registry); err != nil {
				t.Fatal(err)
			}
			descriptor, err := routes.Select(context.Background(), manifest.ID, manifest.Revision)
			if err != nil {
				t.Fatal(err)
			}
			runtime, err := registry.Open(context.Background(), descriptor)
			if scenario == "resident_close" {
				if err != nil {
					t.Fatal(err)
				}
				if err := runtime.Close(); err == nil {
					t.Fatal("uncertain worker stop was reported as clean")
				}
			} else if err == nil || runtime != nil {
				t.Fatalf("unsafe factory result: runtime=%v error=%v", runtime, err)
			}
			if err := store.Remove(manifest.ID, manifest.Revision); err == nil {
				t.Fatal("package removal succeeded while worker stop was uncertain")
			}
		})
	}
}
