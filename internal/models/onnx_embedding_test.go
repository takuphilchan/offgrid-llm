package models

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const onnxSettingsFixture = `{"schema_version":1,"family":"bert-symmetric","inputs":["input_ids","attention_mask"],"output":"last_hidden_state","pooling":"mean","normalize_l2":true,"dimensions":3,"max_tokens":128,"pad_token":"[PAD]","pad_token_id":0}`

func onnxPackageFixture() (ModelPackageManifest, map[string][]byte) {
	artifacts := map[string][]byte{"model.onnx": []byte("fixture graph, not weights"), "tokenizer.json": []byte(`{}`), "embedding.json": []byte(onnxSettingsFixture)}
	m := ModelPackageManifest{SchemaVersion: 1, ID: "fixture-embedding", Revision: "v1", Name: "ONNX fixture", Architecture: "onnx-embedding", ModelFamily: "bert-symmetric", Runtime: PackageRuntime{Adapter: "onnx-embedding", Revision: ONNXEmbeddingAdapterRevision}, Capabilities: []ModelCapability{CapabilityEmbeddings}, Languages: []string{"en"}, License: "MIT", Engine: EngineONNX, Format: FormatONNX, Backend: "cpu", RunnerRevision: "worker-v1", AdapterRevision: ONNXEmbeddingAdapterRevision, Modalities: []string{"text"}, ToolCalling: "unsupported", ContextLength: 128}
	for _, name := range []string{"model.onnx", "tokenizer.json", "embedding.json"} {
		role := map[string]string{"model.onnx": "weights", "tokenizer.json": "tokenizer", "embedding.json": "config"}[name]
		digest := sha256.Sum256(artifacts[name])
		m.Artifacts = append(m.Artifacts, PackageArtifact{Path: name, Role: role, Size: int64(len(artifacts[name])), SHA256: hex.EncodeToString(digest[:]), License: "MIT"})
	}
	return m, artifacts
}

func TestONNXEmbeddingCatalogReservedIdentityNeverUsesLegacyCollider(t *testing.T) {
	registry := NewRegistry(t.TempDir())
	selector := ONNXEmbeddingSelector("fixture-embedding", "v1")
	file := filepath.Join(t.TempDir(), "keep.gguf")
	if err := os.WriteFile(file, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	registry.models[selector] = &api.ModelMetadata{ID: selector, Path: file, Type: "llm"}
	if _, err := registry.GetModel(selector); err == nil {
		t.Fatal("uninstalled selector used legacy collider")
	}
	if err := registry.DeleteModel(selector); err == nil {
		t.Fatal("reserved selector deleted legacy collider")
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatal("legacy collider file was removed")
	}
	m, artifacts := onnxPackageFixture()
	if _, err := registry.Packages().Import(context.Background(), m, func(_ context.Context, a PackageArtifact) (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(artifacts[a.Path])), nil
	}); err != nil {
		t.Fatal(err)
	}
	metadata, err := registry.GetModel(selector)
	if err != nil || metadata.Path != selector || metadata.Type != "embedding" {
		t.Fatalf("wrong package projection: %+v %v", metadata, err)
	}
	if _, err := registry.GetChatModel(selector); err == nil {
		t.Fatal("embedding selected for chat")
	}
	listed := registry.ListModels()
	if len(listed) != 1 || listed[0].Engine != EngineONNX || listed[0].RuntimeReady || listed[0].Qualified {
		t.Fatalf("unverified readiness claimed: %+v", listed)
	}
}

func TestONNXCatalogReadDoesNotInitializeMissingStorage(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "absent-models")
	registry := NewRegistry(directory)
	if got := registry.ListModels(); len(got) != 0 {
		t.Fatal("missing storage returned models")
	}
	if _, err := os.Stat(directory); !os.IsNotExist(err) {
		t.Fatalf("catalog read initialized storage: %v", err)
	}
}

func TestONNXTypedInventoryKeepsOneEmbeddingPackageAuthority(t *testing.T) {
	manifest, _ := onnxPackageFixture()
	state := PackageState{ID: manifest.ID, Revision: manifest.Revision, Manifest: &manifest, Installed: true, Integrity: "checked"}
	legacy := []api.Model{{ID: ONNXEmbeddingSelector(manifest.ID, manifest.Revision), Type: "embedding", Engine: EngineONNX, Format: FormatONNX}}
	items := Inventory(legacy, []PackageState{state}, CategoryEmbeddings)
	if len(items) != 1 || items[0].Kind != "package" || items[0].Category != CategoryEmbeddings || items[0].Engine != EngineONNX || items[0].Legacy != nil || items[0].Readiness.RuntimeReady {
		t.Fatalf("embedding package duplicated/misclassified: %+v", items)
	}
	if got := Inventory(legacy, []PackageState{state}, CategoryGeneration); len(got) != 0 {
		t.Fatal("embedding package advertised as speech generation")
	}
	catalog := &ModelCatalog{Packages: []PackageCatalogEntry{{ID: manifest.ID, Name: manifest.Name, Variants: []ModelPackageManifest{manifest}}}}
	if entries := catalog.TypedEntries(CategoryEmbeddings); len(entries) != 1 || entries[0].Category != CategoryEmbeddings {
		t.Fatalf("embedding catalog category lost: %+v", entries)
	}
}

func TestONNXEmbeddingPackageImportLeaseAndIdentity(t *testing.T) {
	m, artifacts := onnxPackageFixture()
	store := NewPackageStore(t.TempDir())
	if _, err := store.Import(context.Background(), m, func(_ context.Context, a PackageArtifact) (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(artifacts[a.Path])), nil
	}); err != nil {
		t.Fatal(err)
	}
	lease, err := store.AcquireONNXEmbedding(context.Background(), m.ID, m.Revision, "test")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(lease.ModelDigest, "sha256:") || lease.Manifest.Engine != EngineONNX {
		t.Fatalf("identity lost: %+v", lease)
	}
	if err := store.Remove(m.ID, m.Revision); !errors.Is(err, ErrPackageInUse) {
		t.Fatalf("active lease removed: %v", err)
	}
	lease.Release()
	if err := store.Remove(m.ID, m.Revision); err != nil {
		t.Fatal(err)
	}
}

func TestONNXEmbeddingManifestRejectsUnsupportedLayouts(t *testing.T) {
	for _, scenario := range []string{"wrong-engine", "gpu", "family", "tools", "extra-file", "external-graph", "oversize", "speech", "adapter"} {
		t.Run(scenario, func(t *testing.T) {
			m, _ := onnxPackageFixture()
			switch scenario {
			case "wrong-engine":
				m.Engine = EngineTransformers
			case "gpu":
				m.Backend = "cuda"
			case "family":
				m.ModelFamily = "custom"
			case "tools":
				m.ToolCalling = "qualified"
			case "extra-file":
				m.Artifacts = append(m.Artifacts, PackageArtifact{Path: "extra.json", Role: "config"})
			case "external-graph":
				m.Artifacts[0].Path = "external/model.onnx"
			case "oversize":
				m.Artifacts[0].Size = 513 << 20
			case "speech":
				m.SampleRates = []int{16000}
			case "adapter":
				m.AdapterRevision = "future-v1"
			}
			if m.Validate() == nil {
				t.Fatal("unsupported package admitted")
			}
		})
	}
}

func TestONNXEmbeddingSettingsStrictContract(t *testing.T) {
	if _, err := DecodeONNXEmbeddingSettings([]byte(onnxSettingsFixture)); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"schema_version", "normalize_l2", "pad_token_id", "pooling", "dimensions", "output"} {
		var fields map[string]any
		_ = json.Unmarshal([]byte(onnxSettingsFixture), &fields)
		delete(fields, key)
		raw, _ := json.Marshal(fields)
		if _, err := DecodeONNXEmbeddingSettings(raw); err == nil {
			t.Fatalf("missing %s accepted", key)
		}
	}
	for _, raw := range []string{strings.Replace(onnxSettingsFixture, `"schema_version":1`, `"schema_version":true`, 1), strings.Replace(onnxSettingsFixture, `"dimensions":3`, `"dimensions":4097`, 1), strings.Replace(onnxSettingsFixture, `"pooling":"mean"`, `"pooling":"custom"`, 1), strings.Replace(onnxSettingsFixture, `"pad_token_id":0`, `"pad_token_id":null`, 1), strings.Replace(onnxSettingsFixture, `"normalize_l2":true`, `"normalize_l2":false,"normalize_l2":true`, 1)} {
		if _, err := DecodeONNXEmbeddingSettings([]byte(raw)); err == nil {
			t.Fatalf("invalid settings accepted: %s", raw)
		}
	}
}
