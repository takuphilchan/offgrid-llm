package inference

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
	"golang.org/x/sys/windows"
)

var onnxFixtureBinary struct {
	sync.Once
	data []byte
	err  error
}

func onnxRuntimeFixture(t *testing.T) (*models.Registry, ONNXEmbeddingRuntimeConfig, string) {
	t.Helper()
	onnxFixtureBinary.Do(func() {
		directory, err := os.MkdirTemp("", "offgrid-onnx-fixture-")
		if err != nil {
			onnxFixtureBinary.err = err
			return
		}
		defer os.RemoveAll(directory)
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		binary := filepath.Join(directory, "fixture.exe")
		command := exec.CommandContext(ctx, "go", "build", "-o", binary, "./testdata/onnxworker")
		if output, err := command.CombinedOutput(); err != nil {
			onnxFixtureBinary.err = fmt.Errorf("build ONNX protocol fixture: %w: %s", err, output)
			return
		}
		onnxFixtureBinary.data, onnxFixtureBinary.err = os.ReadFile(binary)
	})
	if onnxFixtureBinary.err != nil {
		t.Fatal(onnxFixtureBinary.err)
	}
	root := t.TempDir()
	privateFixtureDirectory(t, root)
	config := ONNXEmbeddingRuntimeConfig{SchemaVersion: 1, RunnerRevision: "worker-v1", RuntimeDirectory: filepath.Join(root, "runtime"), Python: "fixture.exe", Script: "worker.py", StateDirectory: filepath.Join(root, "state"), RuntimeFiles: map[string]string{}, DependencyVersions: map[string]string{"onnx": "1.0", "onnxruntime": "1.0", "numpy": "1.0", "tokenizers": "1.0"}}
	for _, directory := range []string{config.RuntimeDirectory, config.StateDirectory} {
		if err := os.Mkdir(directory, 0700); err != nil {
			t.Fatal(err)
		}
	}
	digest := func(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
	for name, data := range map[string][]byte{"fixture.exe": onnxFixtureBinary.data, "fixture._pth": []byte("Lib\n"), "worker.py": []byte("isolated fixture marker; not Python execution")} {
		if err := os.WriteFile(filepath.Join(config.RuntimeDirectory, name), data, 0600); err != nil {
			t.Fatal(err)
		}
		config.RuntimeFiles[name] = "sha256:" + digest(data)
	}
	manifest := models.ModelPackageManifest{SchemaVersion: 1, ID: "fixture", Revision: "v1", Name: "Isolated ONNX protocol fixture", Architecture: "onnx-embedding", ModelFamily: "bert-symmetric", Runtime: models.PackageRuntime{Adapter: "onnx-embedding", Revision: models.ONNXEmbeddingAdapterRevision}, Capabilities: []models.ModelCapability{models.CapabilityEmbeddings}, Languages: []string{"en"}, License: "MIT", Engine: models.EngineONNX, Format: models.FormatONNX, Backend: "cpu", RunnerRevision: config.RunnerRevision, AdapterRevision: models.ONNXEmbeddingAdapterRevision, Modalities: []string{"text"}, ToolCalling: "unsupported", ContextLength: 128}
	artifacts := map[string][]byte{"model.onnx": []byte("not model weights"), "tokenizer.json": []byte(`{}`), "embedding.json": []byte(`{"schema_version":1,"family":"bert-symmetric","inputs":["input_ids","attention_mask"],"output":"hidden","pooling":"mean","normalize_l2":true,"dimensions":3,"max_tokens":128,"pad_token":"[PAD]","pad_token_id":0}`)}
	for _, entry := range []struct{ name, role string }{{"model.onnx", "weights"}, {"tokenizer.json", "tokenizer"}, {"embedding.json", "config"}} {
		data := artifacts[entry.name]
		manifest.Artifacts = append(manifest.Artifacts, models.PackageArtifact{Path: entry.name, Role: entry.role, Size: int64(len(data)), SHA256: digest(data), License: "MIT"})
	}
	registry := models.NewRegistry(filepath.Join(root, "models"))
	if _, err := registry.Packages().Import(context.Background(), manifest, func(_ context.Context, a models.PackageArtifact) (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(artifacts[a.Path])), nil
	}); err != nil {
		t.Fatal(err)
	}
	return registry, config, models.ONNXEmbeddingSelector(manifest.ID, manifest.Revision)
}

func TestONNXOwnedEmbeddingPackageSchedulingAndCancellation(t *testing.T) {
	if os.Getenv("OFFGRID_ONNX_WINDOWS_FIXTURE_RUNTIME") == "" {
		t.Skip("requires explicit administrator-provisioned read-only fixture runtime; prior writable-runtime success is superseded")
	}
	registry, config, selector := onnxRuntimeFixture(t)
	provisioned, err := LoadONNXEmbeddingRuntimeConfig(os.Getenv("OFFGRID_ONNX_WINDOWS_FIXTURE_RUNTIME"))
	if err != nil {
		t.Fatal(err)
	}
	provisioned.StateDirectory = config.StateDirectory
	config = provisioned
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
	metadata, err := registry.GetModel(selector)
	if err != nil {
		t.Fatal(err)
	}
	response, err := engine.GenerateEmbeddingsForModel(context.Background(), metadata.Path, DefaultEmbeddingOptions(), &api.EmbeddingRequest{Model: selector, Input: []string{"first", "second"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Data) != 2 || len(response.Data[0].Embedding) != 3 || response.Usage.TotalTokens != 4 {
		t.Fatalf("incorrect vectors/usage: %+v", response)
	}
	if err := registry.Packages().Remove("fixture", "v1"); !errors.Is(err, models.ErrPackageInUse) {
		t.Fatalf("active package removed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(config.RuntimeDirectory, config.Script), []byte("replacement"), 0600); err == nil {
		t.Fatal("active worker pin was writable")
	}
	state, err := registry.GetModel(selector)
	if err != nil || state.Type != "embedding" {
		t.Fatalf("catalog lost leased model: %v", err)
	}
	parent := managedFixtureProcess(t, config.StateDirectory, "parent")
	defer windows.CloseHandle(parent)
	descendant := managedFixtureProcess(t, config.StateDirectory, "descendant")
	defer windows.CloseHandle(descendant)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err = engine.GenerateEmbeddingsForModel(ctx, metadata.Path, DefaultEmbeddingOptions(), &api.EmbeddingRequest{Model: selector, Input: "cancel"})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("cancel did not remain cancellation: %v", err)
	}
	for _, handle := range []windows.Handle{parent, descendant} {
		status, err := windows.WaitForSingleObject(handle, 0)
		if err != nil || status != windows.WAIT_OBJECT_0 {
			t.Fatalf("owned process still alive after cancellation: status=%d err=%v", status, err)
		}
	}
	if err := engine.Unload(); err != nil {
		t.Fatal(err)
	}
	if err := registry.Packages().Remove("fixture", "v1"); err != nil {
		t.Fatalf("confirmed cleanup retained package: %v", err)
	}
}

func TestONNXRuntimeRejectsChangedInventoryBeforeLaunch(t *testing.T) {
	for _, mode := range []string{"changed", "extra", "missing", "wrong-runner"} {
		t.Run(mode, func(t *testing.T) {
			registry, config, selector := onnxRuntimeFixture(t)
			switch mode {
			case "changed":
				_ = os.WriteFile(filepath.Join(config.RuntimeDirectory, config.Script), []byte("changed"), 0600)
			case "extra":
				_ = os.WriteFile(filepath.Join(config.RuntimeDirectory, "extra.py"), []byte("extra"), 0600)
			case "missing":
				_ = os.Remove(filepath.Join(config.RuntimeDirectory, config.Script))
			case "wrong-runner":
				config.RunnerRevision = "different"
			}
			engine := NewEmbeddingEngine()
			if err := engine.ConfigureONNXEmbeddingRuntime(registry.Packages(), config); err != nil {
				t.Fatal(err)
			}
			if err := engine.Load(context.Background(), selector, DefaultEmbeddingOptions()); err == nil {
				t.Fatal("unsafe runtime started")
			}
			if _, err := os.Stat(filepath.Join(config.StateDirectory, "parent.pid")); !os.IsNotExist(err) {
				t.Fatalf("worker launched on invalid inventory: %v", err)
			}
			if err := registry.Packages().Remove("fixture", "v1"); err != nil {
				t.Fatalf("failed launch leaked package lease: %v", err)
			}
		})
	}
}

func TestONNXRuntimeEnrollmentCapturesMaps(t *testing.T) {
	registry, config, _ := onnxRuntimeFixture(t)
	engine := NewEmbeddingEngine()
	if err := engine.ConfigureONNXEmbeddingRuntime(registry.Packages(), config); err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(config)
	config.RuntimeFiles[config.Script] = "changed"
	config.DependencyVersions["onnx"] = "changed"
	impl, handled, err := engine.newModelImplementation("embedding:fixture@v1")
	if err != nil || !handled {
		t.Fatal(err)
	}
	after, _ := json.Marshal(impl.(*ONNXEmbeddingImpl).config)
	if !bytes.Equal(before, after) {
		t.Fatal("caller changed captured runtime pins")
	}
}

func TestONNXRuntimeRejectsPythonSearchPathDelegationBeforeLaunch(t *testing.T) {
	for _, content := range []string{"import site\n", "../external\n", "C:\\external\n"} {
		t.Run(content, func(t *testing.T) {
			registry, config, selector := onnxRuntimeFixture(t)
			name := "fixture._pth"
			if err := os.WriteFile(filepath.Join(config.RuntimeDirectory, name), []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
			digest := sha256.Sum256([]byte(content))
			config.RuntimeFiles[name] = "sha256:" + hex.EncodeToString(digest[:])
			engine := NewEmbeddingEngine()
			if err := engine.ConfigureONNXEmbeddingRuntime(registry.Packages(), config); err != nil {
				t.Fatal(err)
			}
			if err := engine.Load(context.Background(), selector, DefaultEmbeddingOptions()); err == nil {
				t.Fatal("external Python initialization admitted")
			}
			if _, err := os.Stat(filepath.Join(config.StateDirectory, "parent.pid")); !os.IsNotExist(err) {
				t.Fatal("worker launched before Python path checks")
			}
		})
	}
}
