package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
	"golang.org/x/sys/windows"
)

// Full public handler + real owned process + durable Knowledge fixture. Its
// vectors are intentionally synthetic; this is not ONNX semantic qualification.
func TestONNXPublicEmbeddingsAndKnowledgePreservePackageIdentity(t *testing.T) {
	if os.Getenv("OFFGRID_ONNX_WINDOWS_FIXTURE_RUNTIME") == "" {
		t.Skip("requires explicit administrator-provisioned read-only fixture runtime; prior writable-runtime success is superseded")
	}
	s := newTestServer(t)
	root := t.TempDir()
	onnxServerPrivateDirectory(t, root)
	onnxServerPrivateDirectory(t, s.config.ModelsDir)
	runtimeDir, stateDir := filepath.Join(root, "runtime"), filepath.Join(root, "state")
	for _, directory := range []string{runtimeDir, stateDir} {
		if err := os.Mkdir(directory, 0700); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	config, err := inference.LoadONNXEmbeddingRuntimeConfig(os.Getenv("OFFGRID_ONNX_WINDOWS_FIXTURE_RUNTIME"))
	if err != nil {
		t.Fatal(err)
	}
	config.StateDirectory = stateDir
	digest := func(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
	configRaw, _ := json.Marshal(config)
	configPath := filepath.Join(root, "runtime.json")
	if err := os.WriteFile(configPath, configRaw, 0600); err != nil {
		t.Fatal(err)
	}
	captured, err := inference.LoadONNXEmbeddingRuntimeConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.embeddingEngine.ConfigureONNXEmbeddingRuntime(s.registry.Packages(), captured); err != nil {
		t.Fatal(err)
	}
	manifest := models.ModelPackageManifest{SchemaVersion: 1, ID: "fixture", Revision: "v1", Name: "ONNX API fixture", Architecture: "onnx-embedding", ModelFamily: "bert-symmetric", Runtime: models.PackageRuntime{Adapter: "onnx-embedding", Revision: models.ONNXEmbeddingAdapterRevision}, Capabilities: []models.ModelCapability{models.CapabilityEmbeddings}, Languages: []string{"en"}, License: "MIT", Engine: models.EngineONNX, Format: models.FormatONNX, Backend: "cpu", RunnerRevision: config.RunnerRevision, AdapterRevision: models.ONNXEmbeddingAdapterRevision, Modalities: []string{"text"}, ToolCalling: "unsupported", ContextLength: 128}
	artifacts := map[string][]byte{"model.onnx": []byte("not ONNX model weights"), "tokenizer.json": []byte(`{}`), "embedding.json": []byte(`{"schema_version":1,"family":"bert-symmetric","inputs":["input_ids","attention_mask"],"output":"hidden","pooling":"mean","normalize_l2":true,"dimensions":3,"max_tokens":128,"pad_token":"[PAD]","pad_token_id":0}`)}
	importPackage := func() {
		t.Helper()
		manifest.Artifacts = nil
		for _, entry := range []struct{ name, role string }{{"model.onnx", "weights"}, {"tokenizer.json", "tokenizer"}, {"embedding.json", "config"}} {
			data := artifacts[entry.name]
			manifest.Artifacts = append(manifest.Artifacts, models.PackageArtifact{Path: entry.name, Role: entry.role, Size: int64(len(data)), SHA256: digest(data), License: "MIT"})
		}
		if _, err := s.registry.Packages().Import(ctx, manifest, func(_ context.Context, a models.PackageArtifact) (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(artifacts[a.Path])), nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	importPackage()
	selector := models.ONNXEmbeddingSelector(manifest.ID, manifest.Revision)
	s.ragEngine.SetModelResolver(func(id string) (string, error) {
		metadata, err := s.registry.GetModel(id)
		if err != nil {
			return "", err
		}
		if metadata.Type != "embedding" {
			return "", fmt.Errorf("model is not embedding")
		}
		return metadata.Path, nil
	})
	s.startRAGAutoRestore()
	s.startupWorkers.Wait()
	if s.ragEngine.IsEnabled() {
		t.Fatal("new ONNX package changed default knowledge selection")
	}
	if _, err := os.Stat(filepath.Join(stateDir, "parent.pid")); !os.IsNotExist(err) {
		t.Fatalf("package import/auto-restore launched unselected provider: %v", err)
	}
	response := httptest.NewRecorder()
	s.handleEmbeddings(response, httptest.NewRequest(http.MethodPost, "/v1/embeddings", strings.NewReader(`{"model":"`+selector+`","input":["first","second"]}`)))
	var embeddings api.EmbeddingResponse
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &embeddings) != nil || len(embeddings.Data) != 2 || embeddings.Usage.TotalTokens != 4 {
		t.Fatalf("public embedding response: %d %s", response.Code, response.Body.String())
	}
	if err := s.ragEngine.Enable(ctx, selector); err != nil {
		t.Fatal(err)
	}
	document, err := s.ragEngine.IngestText(ctx, "fixture.txt", "The isolated fixture library closes at 17:00.", nil)
	if err != nil {
		t.Fatal(err)
	}
	result, err := s.ragEngine.Search(ctx, "When does the library close?", rag.DefaultSearchOptions())
	if err != nil || result == nil || len(result.Results) == 0 || result.Results[0].DocumentID != document.ID {
		t.Fatalf("durable fixture retrieval failed: %+v %v", result, err)
	}
	if err := s.embeddingEngine.Unload(); err != nil {
		t.Fatal(err)
	}
	if err := s.registry.Packages().Remove(manifest.ID, manifest.Revision); err != nil {
		t.Fatal(err)
	}
	artifacts["model.onnx"] = []byte("changed fixture identity, not model weights")
	importPackage()
	if err := s.ragEngine.Enable(ctx, selector); err == nil || !strings.Contains(err.Error(), "differs from the saved index") {
		t.Fatalf("changed package reused stale vectors: %v", err)
	}
	if s.ragEngine.IsEnabled() {
		t.Fatal("changed identity left knowledge enabled")
	}
	if docs := s.ragEngine.ListDocuments(); len(docs) != 1 || docs[0].ID != document.ID {
		t.Fatal("identity failure discarded retained document")
	}
}

func onnxServerPrivateDirectory(t *testing.T, path string) {
	t.Helper()
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	sd, err := windows.SecurityDescriptorFromString("D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;" + user.User.Sid.String() + ")")
	if err != nil {
		t.Fatal(err)
	}
	acl, _, err := sd.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil); err != nil {
		t.Fatal(err)
	}
}
