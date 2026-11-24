package inference

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
	"golang.org/x/sys/unix"
)

func TestONNXLinuxSettingsSnapshotRejectsMutationAndFIFO(t *testing.T) {
	root := t.TempDir()
	raw := []byte("original settings bytes")
	digest := sha256.Sum256(raw)
	model := &models.ONNXEmbeddingLease{ModelDir: root, Manifest: models.ModelPackageManifest{Artifacts: []models.PackageArtifact{{Path: "embedding.json", Size: int64(len(raw)), SHA256: hex.EncodeToString(digest[:])}}}}
	path := filepath.Join(root, "embedding.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	snapshot, err := readLinuxONNXSettings(context.Background(), model)
	if err != nil || !bytes.Equal(snapshot, raw) {
		t.Fatal("valid snapshot failed", err)
	}
	if err := os.WriteFile(path, []byte("changed settings bytes!"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readLinuxONNXSettings(context.Background(), model); err == nil {
		t.Fatal("mutated snapshot accepted")
	}
	if !bytes.Equal(snapshot, raw) {
		t.Fatal("accepted immutable buffer changed")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := unix.Mkfifo(path, 0600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := readLinuxONNXSettings(context.Background(), model); done <- err }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("FIFO accepted")
		}
	case <-time.After(time.Second):
		t.Fatal("settings snapshot blocked on FIFO")
	}
}

func TestONNXLinuxRuntimeRejectsUserWritableImportNamespace(t *testing.T) {
	root := t.TempDir()
	data := []byte("immutable claim must be rejected")
	digest := sha256.Sum256(data)
	if err := os.WriteFile(filepath.Join(root, "runtime"), data, 0600); err != nil {
		t.Fatal(err)
	}
	lease := &managedOllamaFiles{}
	defer lease.close()
	if err := lease.onnxRuntimeTree(context.Background(), root, map[string]string{"runtime": "sha256:" + hex.EncodeToString(digest[:])}); err == nil {
		t.Fatal("service-owned runtime directory was accepted as immutable")
	}
}

func TestONNXLinuxProvisionedFactorySchedulingAndCancellation(t *testing.T) {
	filename := os.Getenv("OFFGRID_ONNX_LINUX_FIXTURE_RUNTIME")
	if filename == "" {
		t.Skip("requires explicit root-provisioned immutable fixture runtime and private writable state")
	}
	config, err := LoadONNXEmbeddingRuntimeConfig(filename)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	registry := models.NewRegistry(t.TempDir())
	manifest := models.ModelPackageManifest{SchemaVersion: 1, ID: "fixture", Revision: "v1", Name: "ONNX Linux process fixture", Architecture: "onnx-embedding", ModelFamily: "bert-symmetric", Runtime: models.PackageRuntime{Adapter: "onnx-embedding", Revision: models.ONNXEmbeddingAdapterRevision}, Capabilities: []models.ModelCapability{models.CapabilityEmbeddings}, Languages: []string{"en"}, License: "MIT", Engine: models.EngineONNX, Format: models.FormatONNX, Backend: "cpu", RunnerRevision: config.RunnerRevision, AdapterRevision: models.ONNXEmbeddingAdapterRevision, Modalities: []string{"text"}, ToolCalling: "unsupported", ContextLength: 128}
	artifacts := map[string][]byte{"model.onnx": []byte("isolated fixture, not ONNX weights"), "tokenizer.json": []byte(`{}`), "embedding.json": []byte(`{"schema_version":1,"family":"bert-symmetric","inputs":["input_ids","attention_mask"],"output":"hidden","pooling":"mean","normalize_l2":true,"dimensions":3,"max_tokens":128,"pad_token":"[PAD]","pad_token_id":0}`)}
	for _, entry := range []struct{ name, role string }{{"model.onnx", "weights"}, {"tokenizer.json", "tokenizer"}, {"embedding.json", "config"}} {
		data := artifacts[entry.name]
		digest := sha256.Sum256(data)
		manifest.Artifacts = append(manifest.Artifacts, models.PackageArtifact{Path: entry.name, Role: entry.role, Size: int64(len(data)), SHA256: hex.EncodeToString(digest[:]), License: "MIT"})
	}
	if _, err := registry.Packages().Import(ctx, manifest, func(_ context.Context, a models.PackageArtifact) (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(artifacts[a.Path])), nil
	}); err != nil {
		t.Fatal(err)
	}
	engine := NewEmbeddingEngine()
	scheduler, err := NewProviderScheduler(NewProviderRegistry(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := scheduler.AttachEmbeddingEngine(engine); err != nil {
		t.Fatal(err)
	}
	defer scheduler.Close()
	if err := engine.ConfigureONNXEmbeddingRuntime(registry.Packages(), config); err != nil {
		t.Fatal(err)
	}
	selector := models.ONNXEmbeddingSelector(manifest.ID, manifest.Revision)
	response, err := engine.GenerateEmbeddingsForModel(ctx, selector, DefaultEmbeddingOptions(), &api.EmbeddingRequest{Model: selector, Input: []string{"first", "second"}})
	if err != nil || response == nil || len(response.Data) != 2 || response.Usage.TotalTokens != 4 {
		t.Fatalf("provisioned fixture failed: %+v %v", response, err)
	}
	if err := os.WriteFile(filepath.Join(config.RuntimeDirectory, "new_unpinned.py"), []byte("must not become importable"), 0600); err == nil {
		t.Fatal("runtime namespace remained writable")
	}
	cancelCtx, stop := context.WithTimeout(ctx, 100*time.Millisecond)
	defer stop()
	if _, err := engine.GenerateEmbeddingsForModel(cancelCtx, selector, DefaultEmbeddingOptions(), &api.EmbeddingRequest{Model: selector, Input: "cancel"}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("active cancellation lost: %v", err)
	}
	if err := engine.Unload(); err != nil {
		t.Fatal(err)
	}
	if err := registry.Packages().Remove(manifest.ID, manifest.Revision); err != nil {
		t.Fatal("confirmed retirement leaked model lease", err)
	}
}
