package models

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVisionBundleResolvesGenericPrecisionAlternatives(t *testing.T) {
	f, hf, _ := newPackageHubFixture(t, "vision")
	selected := "NVIDIA-Nemotron-3.5-Bolt-E4B-UD-Q4_K_XL.gguf"
	for _, name := range []string{selected, "NVIDIA-Nemotron-3.5-Bolt-E4B-BF16.gguf", "NVIDIA-Nemotron-3.5-Bolt-E4B-UD-IQ2_M.gguf", "dflash-NVIDIA-Nemotron-3.5-Bolt-E4B-Q8_0.gguf", "imatrix_unsloth.gguf", "mmproj-BF16.gguf", "mmproj-F16.gguf", "mmproj-Q8_0.gguf"} {
		f.add(name, []byte("fixture-"+name), true)
	}
	bundle, err := hf.ResolveGGUFBundle(context.Background(), f.meta.ID, "", selected)
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Projector == nil || bundle.Projector.Path != "mmproj-F16.gguf" || len(bundle.Artifacts()) != 2 || bundle.Revision != fixtureRevision {
		t.Fatalf("wrong bundle: %+v", bundle)
	}
	if bundle.TotalBytes() != bundle.Weights.Size+bundle.Projector.Size {
		t.Fatal("incomplete byte count")
	}
	for _, request := range f.fetches {
		if strings.Contains(request, "/resolve/") {
			t.Fatal("resolution downloaded weights")
		}
	}
}

func TestVisionBundleRejectsAmbiguousOrIncompleteSources(t *testing.T) {
	for _, mode := range []string{"mixed", "missing", "revision", "no-digest", "duplicate-projector"} {
		t.Run(mode, func(t *testing.T) {
			f, hf, _ := newPackageHubFixture(t, "vision")
			f.add("model-Q4_K_M.gguf", []byte("weights"), mode != "no-digest")
			if mode != "missing" {
				f.add("mmproj-F16.gguf", []byte("projector"), true)
			}
			revision := ""
			switch mode {
			case "mixed":
				f.add("unrelated-Q4_K_M.gguf", []byte("other"), true)
			case "missing":
				f.meta.PipelineTag = "image-text-to-text"
			case "revision":
				revision = strings.Repeat("b", 40)
			case "duplicate-projector":
				f.add("projector-F16.gguf", []byte("other"), true)
			}
			if b, err := hf.ResolveGGUFBundle(context.Background(), f.meta.ID, revision, "model-Q4_K_M.gguf"); err == nil || b != nil {
				t.Fatalf("bad source accepted: %+v %v", b, err)
			}
		})
	}
	for _, name := range []string{"mmproj-F16.gguf", "model-00001-of-00002.gguf", "dflash-model.gguf", "imatrix.gguf", "../model.gguf"} {
		if _, err := (*HuggingFaceClient)(nil).ResolveGGUFBundle(context.Background(), "fixture/speech", "", name); err == nil {
			t.Fatalf("not weights: %s", name)
		}
	}
}

func visionFixtureBundle(t *testing.T) (GGUFBundle, *HuggingFaceClient) {
	t.Helper()
	artifact := func(name, role, data string) PackageArtifact {
		h := sha256.Sum256([]byte(data))
		return PackageArtifact{Path: name, Role: role, Size: int64(len(data)), SHA256: hex.EncodeToString(h[:]), Source: &ArtifactSource{Repository: "https://huggingface.co/fixture/vision", Revision: fixtureRevision, Path: name}}
	}
	projector := artifact("mmproj-F16.gguf", "projector", "fixture projector data")
	b := GGUFBundle{SchemaVersion: 1, Repository: "fixture/vision", Revision: fixtureRevision, Weights: artifact("model.gguf", "weights", "fixture weights data"), Projector: &projector}
	hf := downloadClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/resolve/"+fixtureRevision+"/") {
			t.Errorf("mutable source: %s", r.URL.Path)
		}
		data := "fixture weights data"
		if strings.HasSuffix(r.URL.Path, "mmproj-F16.gguf") {
			data = "fixture projector data"
		}
		w.Header().Set("Content-Length", fmt.Sprint(len(data)))
		fmt.Fprint(w, data)
	})
	return b, hf
}

func TestVisionBundleInstallResumeAndAssociation(t *testing.T) {
	b, hf := visionFixtureBundle(t)
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	err := hf.AcquireGGUFBundle(ctx, dir, "local.gguf", b, func(done, total int64) {
		if done >= b.Weights.Size {
			cancel()
		}
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
	if _, ok := InstalledCompanion(filepath.Join(dir, "local.gguf")); ok {
		t.Fatal("incomplete model gained vision")
	}
	if err := hf.AcquireGGUFBundle(context.Background(), dir, "local.gguf", b, nil); err != nil {
		t.Fatal(err)
	}
	projector, ok := InstalledCompanion(filepath.Join(dir, "local.gguf"))
	if !ok {
		t.Fatal("verified companion not associated")
	}
	registry := NewRegistry(dir)
	if err := registry.ScanModels(); err != nil {
		t.Fatal(err)
	}
	if list := registry.ListModels(); len(list) != 1 || list[0].ID != "local" {
		t.Fatalf("companion leaked into models: %+v", list)
	}
	model, _ := registry.GetModel("local")
	if model.ProjectorPath != projector {
		t.Fatal("registry lost generic projector")
	}
	if err := os.WriteFile(projector, []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, ok := InstalledCompanion(filepath.Join(dir, "local.gguf")); ok {
		t.Fatal("changed projector stayed ready")
	}
	if err := hf.AcquireGGUFBundle(context.Background(), dir, "local.gguf", b, nil); err != nil {
		t.Fatal(err)
	}
	if _, ok := InstalledCompanion(filepath.Join(dir, "local.gguf")); !ok {
		t.Fatal("corrupt companion not repaired")
	}
	backups, err := filepath.Glob(projector + ".corrupt-*")
	if err != nil || len(backups) != 1 {
		t.Fatalf("missing recoverable corrupt file: %v %v", backups, err)
	}
	old, _ := os.ReadFile(backups[0])
	if string(old) != "changed" {
		t.Fatal("corrupt original not retained")
	}
}

func TestVisionReceiptRejectsMalformedAndEscapingReferences(t *testing.T) {
	for _, mode := range []string{"malformed", "traversal", "other-model", "revision", "digest"} {
		t.Run(mode, func(t *testing.T) {
			b, hf := visionFixtureBundle(t)
			dir := t.TempDir()
			if err := hf.AcquireGGUFBundle(context.Background(), dir, "local.gguf", b, nil); err != nil {
				t.Fatal(err)
			}
			p := filepath.Join(dir, "local.gguf.assets.json")
			data, _ := os.ReadFile(p)
			var receipt modelCompanionReceipt
			if err := json.Unmarshal(data, &receipt); err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "traversal":
				receipt.ProjectorFile = "../../outside.gguf"
			case "other-model":
				receipt.ModelFile = "another.gguf"
			case "revision":
				receipt.Bundle.Revision = "main"
			case "digest":
				receipt.Bundle.Projector.SHA256 = strings.Repeat("b", 64)
			}
			data, _ = json.Marshal(receipt)
			if mode == "malformed" {
				data = []byte("{")
			}
			if err := os.WriteFile(p, data, 0600); err != nil {
				t.Fatal(err)
			}
			if _, ok := InstalledCompanion(filepath.Join(dir, "local.gguf")); ok {
				t.Fatal("unsafe receipt accepted")
			}
		})
	}
}

func TestVisionBundleLargeLanguageModelAndRetainedProgress(t *testing.T) {
	b, _ := visionFixtureBundle(t)
	b.Weights.Size = 80 << 30
	if err := b.Validate(); err != nil {
		t.Fatalf("large language model rejected: %v", err)
	}
	b.Weights.Size = maxGGUFBundleBytes
	if err := b.Validate(); err == nil {
		t.Fatal("aggregate limit bypassed")
	}
	b, hf := visionFixtureBundle(t)
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	_ = hf.AcquireGGUFBundle(ctx, dir, "local.gguf", b, func(done, total int64) {
		if done >= b.Weights.Size {
			cancel()
		}
	})
	if n := RetainedGGUFBundleBytes(dir, "local.gguf", b); n != b.Weights.Size {
		t.Fatalf("lost retained weights: %d", n)
	}
}

func TestVisionBundleDoesNotFollowCompanionDirectoryLink(t *testing.T) {
	b, hf := visionFixtureBundle(t)
	dir, outside := t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dir, modelAssetsDirectory)); err != nil {
		t.Skipf("directory symlinks unavailable: %v", err)
	}
	if err := hf.AcquireGGUFBundle(context.Background(), dir, "local.gguf", b, nil); err == nil {
		t.Fatal("linked companion directory accepted")
	}
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 0 {
		t.Fatalf("outside directory modified: %v %v", entries, err)
	}
}

func TestVisionBundlePreservesExistingWeightsAndNamespaces(t *testing.T) {
	b, hf := visionFixtureBundle(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "existing.gguf"), []byte("user-owned different model"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := hf.AcquireGGUFBundle(context.Background(), dir, "existing.gguf", b, nil); err == nil {
		t.Fatal("foreign existing model reused")
	}
	data, _ := os.ReadFile(filepath.Join(dir, "existing.gguf"))
	if string(data) != "user-owned different model" {
		t.Fatal("existing model overwritten")
	}
	var paths []string
	for _, name := range []string{"one.gguf", "two.gguf"} {
		if err := hf.AcquireGGUFBundle(context.Background(), dir, name, b, nil); err != nil {
			t.Fatal(err)
		}
		p, ok := InstalledCompanion(filepath.Join(dir, name))
		if !ok {
			t.Fatal("missing binding")
		}
		paths = append(paths, p)
	}
	if paths[0] == paths[1] {
		t.Fatal("model-specific namespaces collided")
	}
	b.Projector.SHA256 = strings.Repeat("b", 64)
	if err := hf.AcquireGGUFBundle(context.Background(), dir, "corrupt.gguf", b, nil); err == nil {
		t.Fatal("bad artifact digest accepted")
	}
	if _, ok := InstalledCompanion(filepath.Join(dir, "corrupt.gguf")); ok {
		t.Fatal("corrupt artifact gained vision")
	}
}
