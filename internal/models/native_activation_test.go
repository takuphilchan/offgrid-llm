package models

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func nativeFixtureAdapter(m ModelPackageManifest) ModelAdapterSpec {
	return ModelAdapterSpec{ID: m.ModelFamily, Revision: m.AdapterRevision, Engine: m.Engine, Formats: []string{m.Format}, Modalities: []string{"text"}, ToolCalling: "unqualified", Template: "chatml", Tokenization: "template_exact_ids", Streaming: "text_delta_v1", ToolEnvelope: "none", MaxQualifiedContext: m.ContextLength}
}

func TestNativePackageTemplateMustMatchReviewedAdapter(t *testing.T) {
	for _, scenario := range []string{"match", "wrong", "missing"} {
		store := NewPackageStore(t.TempDir())
		manifest := testPackageManifest("transformers-text")
		if scenario == "wrong" {
			manifest.TemplateDigest = strings.Repeat("0", 64)
		} else if scenario == "missing" {
			manifest.TemplateDigest = ""
		}
		if _, err := store.Import(context.Background(), manifest, fixturePackageSource); err != nil {
			t.Fatal(err)
		}
		registry := NewAdapterRegistry()
		if err := registry.Register(nativeFixtureAdapter(manifest)); err != nil {
			t.Fatal(err)
		}
		lease, err := store.AcquireNativeText(context.Background(), manifest.ID, manifest.Revision, "test", registry)
		if scenario != "match" && err == nil || scenario == "match" && err != nil {
			t.Fatalf("scenario=%s, lease=%v, err=%v", scenario, lease, err)
		}
		if lease != nil {
			lease.Release()
		}
	}
}

func TestNativePackageRequiresTrustedPinnedAdapterAndHoldsLease(t *testing.T) {
	store := NewPackageStore(t.TempDir())
	manifest := testPackageManifest("transformers-text")
	if _, err := store.Import(context.Background(), manifest, fixturePackageSource); err != nil {
		t.Fatal(err)
	}
	registry := NewAdapterRegistry()
	if _, err := store.AcquireNativeText(context.Background(), manifest.ID, manifest.Revision, "test", registry); err == nil {
		t.Fatal("unknown model family was admitted")
	}
	if err := registry.Register(nativeFixtureAdapter(manifest)); err != nil {
		t.Fatal(err)
	}
	lease, err := store.AcquireNativeText(context.Background(), manifest.ID, manifest.Revision, "test", registry)
	if err != nil {
		t.Fatal(err)
	}
	if lease.Engine != EngineTransformers || lease.ModelFamily != manifest.ModelFamily || !strings.HasPrefix(lease.ModelDigest, "sha256:") || !filepath.IsAbs(lease.ModelDir) {
		t.Fatalf("native package identity was not pinned: %+v", lease)
	}
	if err := store.Remove(manifest.ID, manifest.Revision); !errors.Is(err, ErrPackageInUse) {
		t.Fatalf("removed an active native package: %v", err)
	}
	lease.Release()
	lease.Release()
	if err := store.Remove(manifest.ID, manifest.Revision); err != nil {
		t.Fatal(err)
	}
}

func TestNativePackageRejectsModifiedArtifactsAndUnqualifiedVision(t *testing.T) {
	for _, scenario := range []string{"tampered", "vision"} {
		t.Run(scenario, func(t *testing.T) {
			store := NewPackageStore(t.TempDir())
			manifest := testPackageManifest("mlx-text")
			if scenario == "vision" {
				manifest.Modalities = []string{"text", "vision"}
			}
			if _, err := store.Import(context.Background(), manifest, fixturePackageSource); err != nil {
				t.Fatal(err)
			}
			registry := NewAdapterRegistry()
			if err := registry.Register(nativeFixtureAdapter(manifest)); err != nil {
				t.Fatal(err)
			}
			if scenario == "tampered" {
				path := filepath.Join(store.modelsDir, PackageDirectory, manifest.ID, manifest.Revision, manifest.Artifacts[0].Path)
				if err := os.WriteFile(path, []byte("modified"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if lease, err := store.AcquireNativeText(context.Background(), manifest.ID, manifest.Revision, "test", registry); err == nil {
				lease.Release()
				t.Fatal("unverified native model was admitted")
			}
			if err := store.Remove(manifest.ID, manifest.Revision); err != nil {
				t.Fatalf("rejected activation leaked a package lease: %v", err)
			}
		})
	}
}

func TestAdapterRegistrySnapshotsTrustedSpecs(t *testing.T) {
	manifest := testPackageManifest("mlx-text")
	registry := NewAdapterRegistry()
	spec := nativeFixtureAdapter(manifest)
	if err := registry.Register(spec); err != nil {
		t.Fatal(err)
	}
	spec.Formats[0] = FormatSafetensors
	spec.Modalities[0] = "vision"
	resolved, ok := registry.Resolve(manifest.ModelFamily, manifest.AdapterRevision)
	if !ok || resolved.Formats[0] != FormatMLXSafetensors || resolved.Modalities[0] != "text" {
		t.Fatalf("caller mutated trusted adapter: %+v", resolved)
	}
	resolved.Formats[0] = FormatSafetensors
	resolved, _ = registry.Resolve(manifest.ModelFamily, manifest.AdapterRevision)
	if resolved.Formats[0] != FormatMLXSafetensors {
		t.Fatal("resolved adapter could mutate registry")
	}
}
