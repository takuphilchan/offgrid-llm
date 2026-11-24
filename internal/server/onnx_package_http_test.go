package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestONNXPackageHTTPImportAndBothCatalogsRemainUnqualified(t *testing.T) {
	s, _, _ := newOperationFixture(t)
	manifest := models.ModelPackageManifest{SchemaVersion: 1, ID: "offline-embedding", Revision: "v1", Name: "Metadata-only ONNX fixture", Architecture: "onnx-embedding", ModelFamily: "bert-symmetric", Runtime: models.PackageRuntime{Adapter: "onnx-embedding", Revision: models.ONNXEmbeddingAdapterRevision}, Capabilities: []models.ModelCapability{models.CapabilityEmbeddings}, Languages: []string{"en"}, License: "MIT", Engine: models.EngineONNX, Format: models.FormatONNX, Backend: "cpu", RunnerRevision: "worker-v1", AdapterRevision: models.ONNXEmbeddingAdapterRevision, Modalities: []string{"text"}, ToolCalling: "unsupported", ContextLength: 128}
	artifacts := map[string][]byte{"model.onnx": []byte("metadata fixture, not ONNX weights"), "tokenizer.json": []byte(`{}`), "embedding.json": []byte(`{"schema_version":1,"family":"bert-symmetric","inputs":["input_ids","attention_mask"],"output":"hidden","pooling":"mean","normalize_l2":true,"dimensions":3,"max_tokens":128,"pad_token":"[PAD]","pad_token_id":0}`)}
	for _, entry := range []struct{ name, role string }{{"model.onnx", "weights"}, {"tokenizer.json", "tokenizer"}, {"embedding.json", "config"}} {
		raw := artifacts[entry.name]
		digest := sha256.Sum256(raw)
		manifest.Artifacts = append(manifest.Artifacts, models.PackageArtifact{Path: entry.name, Role: entry.role, Size: int64(len(raw)), SHA256: hex.EncodeToString(digest[:]), License: "MIT"})
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormField("manifest")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewEncoder(part).Encode(manifest); err != nil {
		t.Fatal(err)
	}
	for _, artifact := range manifest.Artifacts {
		part, err := writer.CreateFormFile(artifact.Path, artifact.Path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(artifacts[artifact.Path]); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, modelPackagePath, &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()
	requestBodyLimitMiddleware(http.HandlerFunc(s.handleModelPackages)).ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("ONNX public import: %d %s", response.Code, response.Body.String())
	}

	response = modelRequest(t, s, http.MethodGet, "/api/v2/models?category=embeddings", nil)
	var inventory struct {
		Models []models.InstalledModel `json:"models"`
	}
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &inventory) != nil || len(inventory.Models) != 1 {
		t.Fatalf("typed embedding inventory: %d %s", response.Code, response.Body.String())
	}
	model := inventory.Models[0]
	if model.ID != manifest.ID || model.Kind != "package" || model.Category != models.CategoryEmbeddings || model.Engine != models.EngineONNX || model.Legacy != nil || model.Readiness.RuntimeReady || model.Readiness.Qualified {
		t.Fatalf("ONNX package was duplicated, reclassified or falsely qualified: %+v", model)
	}
	response = httptest.NewRecorder()
	s.handleListModels(response, httptest.NewRequest(http.MethodGet, "/v1/models", nil))
	var public api.ModelListResponse
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &public) != nil || len(public.Data) != 1 {
		t.Fatalf("public embedding catalog: %d %s", response.Code, response.Body.String())
	}
	entry := public.Data[0]
	if entry.ID != models.ONNXEmbeddingSelector(manifest.ID, manifest.Revision) || entry.Type != "embedding" || entry.CapabilityStatus["embeddings"] != "installed_unqualified" || entry.RuntimeReady || entry.Qualified {
		t.Fatalf("public catalog lost package routing or readiness: %+v", entry)
	}
}
