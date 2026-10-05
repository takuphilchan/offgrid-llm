package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/internal/storage"
	"github.com/takuphilchan/offgrid-llm/internal/users"
)

type modelTestTransport func(*http.Request) (*http.Response, error)

func TestSpeechSearchUsesHubForQwenPrefixes(t *testing.T) {
	s, _, _ := newOperationFixture(t)
	for _, query := range []string{"qwe", "qwen", "qwen-custom"} {
		called := false
		s.modelHub = models.NewHuggingFaceClient().WithHTTPClient(&http.Client{Transport: modelTestTransport(func(r *http.Request) (*http.Response, error) {
			called = true
			if r.URL.Query().Get("search") != query {
				t.Errorf("query lost: %s", r.URL)
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`[{"id":"community/qwen-custom","safetensors":{"total":1234}}]`))}, nil
		})})
		w := modelRequest(t, s, "GET", "/api/v2/models/catalog?category=speech_recognition&source=huggingface&q="+query, nil)
		if !called || w.Code != 200 || !strings.Contains(w.Body.String(), "community/qwen-custom") || strings.Contains(w.Body.String(), `"size_bytes":1234`) {
			t.Fatalf("search substituted or mislabeled: %d %s", w.Code, w.Body.String())
		}
	}
}

func (f modelTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func newOperationFixture(t *testing.T) (*Server, models.ModelPackageManifest, *atomic.Int64) {
	t.Helper()
	dir := t.TempDir()
	modelDir := filepath.Join(dir, "models")
	dataDir := filepath.Join(dir, "data")
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		t.Fatal(err)
	}
	data := []byte("synthetic speech bytes")
	hash := sha256.Sum256(data)
	m := models.ModelPackageManifest{SchemaVersion: 1, ID: "fixture", Revision: "rev-1", Name: "Synthetic fixture", Architecture: "whisper", Runtime: models.PackageRuntime{Adapter: "whisper.cpp", Revision: "test-1"}, Capabilities: []models.ModelCapability{models.CapabilityTranscription}, Languages: []string{"en"}, SampleRates: []int{16000}, License: "CC0-1.0", Artifacts: []models.PackageArtifact{{Path: "weights.bin", Role: "weights", Size: int64(len(data)), SHA256: hex.EncodeToString(hash[:]), License: "CC0-1.0", Source: &models.ArtifactSource{Repository: "https://huggingface.co/fixture/speech", Revision: strings.Repeat("a", 40), Path: "weights.bin"}}}}
	ctx, cancel := context.WithCancel(context.Background())
	calls := &atomic.Int64{}
	hf := models.NewHuggingFaceClient().WithHTTPClient(&http.Client{Transport: modelTestTransport(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{StatusCode: 200, ContentLength: int64(len(data)), Body: io.NopCloser(bytes.NewReader(data))}, nil
	})})
	s := &Server{config: &config.Config{DataDir: dataDir, ModelsDir: modelDir}, registry: models.NewRegistry(modelDir), runtimeCtx: ctx, modelHub: hf, downloadProgress: map[string]*DownloadProgress{}, modelResolutions: map[string]modelResolutionRecord{"preview": {ID: "preview", Actor: "local", ExpiresAt: time.Now().Add(time.Hour), Resolution: models.PackageResolution{Manifest: m, Provenance: models.ModelProvenance{Kind: "repository_metadata", Repository: "fixture/speech", Revision: strings.Repeat("a", 40)}}}}}
	t.Cleanup(func() { cancel(); s.downloadWorkers.Wait() })
	return s, m, calls
}
func modelRequest(t *testing.T, s *Server, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	s.handleModelsV2(w, httptest.NewRequest(method, path, bytes.NewReader(b)))
	return w
}
func submitFixtureOperation(t *testing.T, s *Server, request string) ModelOperation {
	t.Helper()
	w := modelRequest(t, s, "POST", "/api/v2/models/operations", modelOperationRequest{Action: "install", RequestID: request, ResolutionID: "preview"})
	if w.Code != 202 {
		t.Fatalf("submit: %d %s", w.Code, w.Body.String())
	}
	var o ModelOperation
	if json.Unmarshal(w.Body.Bytes(), &o) != nil {
		t.Fatal("invalid operation response")
	}
	return o
}

func TestModelOperationsPersistDeduplicateAndRejectConflictingRequests(t *testing.T) {
	s, _, calls := newOperationFixture(t)
	s.downloadStateWriter = func(p string, v any) error {
		err := storage.WriteJSON(p, v)
		if err != nil {
			t.Logf("snapshot error: %v", err)
		}
		return err
	}
	o := submitFixtureOperation(t, s, "request-one")
	// The accepted ID is already in the durable snapshot, even if the worker
	// has not reached the first network request yet.
	b, err := os.ReadFile(filepath.Join(s.config.DataDir, "downloads.json"))
	if err != nil {
		t.Fatal(err)
	}
	var snapshot downloadSnapshot
	if json.Unmarshal(b, &snapshot) != nil || snapshot.Operations[o.ID] == nil {
		t.Fatal("202 preceded persistence")
	}
	s.downloadWorkers.Wait()
	again := submitFixtureOperation(t, s, "request-one")
	if again.ID != o.ID || again.State != "complete" || calls.Load() != 1 {
		t.Fatal("retry did not return completed work", again.State, again.ErrorCode, again.Message, calls.Load())
	}
	preview := s.modelResolutions["preview"]
	preview.ID = "fresh-preview"
	s.modelResolutions[preview.ID] = preview
	fresh := modelRequest(t, s, "POST", "/api/v2/models/operations", modelOperationRequest{Action: "install", RequestID: "request-one", ResolutionID: preview.ID})
	if fresh.Code != 202 || !strings.Contains(fresh.Body.String(), o.ID) || calls.Load() != 1 {
		t.Fatal("same immutable source with a refreshed preview was not deduplicated", fresh.Body.String())
	}
	w := modelRequest(t, s, "POST", "/api/v2/models/operations", modelOperationRequest{Action: "install", RequestID: "request-one", ResolutionID: "changed"})
	if w.Code != 409 {
		t.Fatal("conflicting request accepted", w.Code)
	}
	w = modelRequest(t, s, "GET", "/api/v2/models?category=speech_recognition", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "fixture") {
		t.Fatal("package absent from inventory", w.Body.String())
	}
	if len(s.registry.ListModels()) != 0 {
		t.Fatal("speech entered legacy model list")
	}
}

func TestModelOperationAcceptanceFailureStartsNoWorker(t *testing.T) {
	s, _, calls := newOperationFixture(t)
	s.downloadStateWriter = func(string, any) error { return syscall.ENOSPC }
	w := modelRequest(t, s, "POST", "/api/v2/models/operations", modelOperationRequest{Action: "install", RequestID: "request-one", ResolutionID: "preview"})
	if w.Code == 202 {
		t.Fatal("accepted unpersisted work")
	}
	s.downloadWorkers.Wait()
	if calls.Load() != 0 || len(s.modelOperations) != 0 {
		t.Fatal("failed acceptance ran inference/transfer")
	}
}

func TestModelLostCompletionAcknowledgementReconcilesAfterRestart(t *testing.T) {
	s, m, calls := newOperationFixture(t)
	s.downloadStateWriter = func(p string, v any) error {
		if snapshot, ok := v.(downloadSnapshot); ok {
			for _, o := range snapshot.Operations {
				if o.State == "complete" {
					return syscall.ENOSPC
				}
			}
		}
		return storage.WriteJSON(p, v)
	}
	o := submitFixtureOperation(t, s, "request-one")
	s.downloadWorkers.Wait()
	if s.modelOperations[o.ID].State != "interrupted" {
		t.Fatal("unpersisted success announced")
	}
	restarted := &Server{config: s.config, registry: models.NewRegistry(s.config.ModelsDir)}
	if err := restarted.loadDownloads(); err != nil {
		t.Fatal(err)
	}
	recovered := restarted.modelOperations[o.ID]
	if recovered.State != "complete" || recovered.BytesDone != m.Artifacts[0].Size || calls.Load() != 1 {
		t.Fatal("lost acknowledgement not reconciled", recovered.State, calls.Load())
	}
	if restarted.downloadCancelFuncs != nil {
		t.Fatal("recovery started a worker")
	}
}

func TestInterruptedModelWorkDoesNotResumeItselfAndRetainsSource(t *testing.T) {
	s, _, calls := newOperationFixture(t)
	s.downloadStateWriter = func(p string, v any) error {
		if snapshot, ok := v.(downloadSnapshot); ok {
			for _, o := range snapshot.Operations {
				if o.State != "queued" {
					return syscall.ENOSPC
				}
			}
		}
		return storage.WriteJSON(p, v)
	}
	o := submitFixtureOperation(t, s, "request-one")
	s.downloadWorkers.Wait()
	restarted := &Server{config: s.config, registry: models.NewRegistry(s.config.ModelsDir)}
	if err := restarted.loadDownloads(); err != nil {
		t.Fatal(err)
	}
	if restarted.modelOperations[o.ID].State != "interrupted" || calls.Load() != 1 || restarted.modelOperations[o.ID].Target.Package.Artifacts[0].Source.Revision != strings.Repeat("a", 40) {
		t.Fatal("restart lost state or source")
	}
}

func TestModelOperationsCancelMustSettleBeforeResumeOrDelete(t *testing.T) {
	s, m, _ := newOperationFixture(t)
	entered, release := make(chan struct{}), make(chan struct{})
	s.modelHub = models.NewHuggingFaceClient().WithHTTPClient(&http.Client{Transport: modelTestTransport(func(r *http.Request) (*http.Response, error) {
		close(entered)
		<-r.Context().Done()
		<-release
		return nil, r.Context().Err()
	})})
	o := submitFixtureOperation(t, s, "request-one")
	<-entered
	w := modelRequest(t, s, "POST", "/api/v2/models/operations/"+o.ID+"/cancel", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "cancelling") {
		t.Fatal("cancel did not report settling")
	}
	w = modelRequest(t, s, "POST", "/api/v2/models/operations/"+o.ID+"/resume", nil)
	if w.Code != 409 {
		t.Fatal("resume raced cancelled file handle")
	}
	w = httptest.NewRecorder()
	s.handleModelPackages(w, httptest.NewRequest("POST", modelPackagePath+"/"+m.ID+"/"+m.Revision+"/remove", nil))
	if w.Code != 409 {
		t.Fatal("delete ignored pending worker")
	}
	close(release)
	s.downloadWorkers.Wait()
	w = modelRequest(t, s, "POST", "/api/v2/models/operations/"+o.ID+"/discard", nil)
	if w.Code != 200 {
		t.Fatal("discard failed", w.Body.String())
	}
	w = modelRequest(t, s, "POST", "/api/v2/models/operations/"+o.ID+"/resume", nil)
	if w.Code != 409 {
		t.Fatal("discarded operation resumed")
	}
}

func TestModelRepairRequiresTrustedReceiptAndPreservesLeasedPackage(t *testing.T) {
	s, m, _ := newOperationFixture(t)
	o := submitFixtureOperation(t, s, "request-one")
	s.downloadWorkers.Wait()
	_, release, err := s.registry.Packages().Acquire(context.Background(), m.ID, m.Revision, "active speech job")
	if err != nil {
		t.Fatal(err)
	}
	w := modelRequest(t, s, "POST", "/api/v2/models/operations", modelOperationRequest{Action: "repair", RequestID: "request-repair", SourceOperationID: o.ID})
	if w.Code != 202 {
		t.Fatal("repair submit", w.Body.String())
	}
	var repair ModelOperation
	json.Unmarshal(w.Body.Bytes(), &repair)
	s.downloadWorkers.Wait()
	if s.modelOperations[repair.ID].ErrorCode != "package_in_use" {
		t.Fatal("repair ignored active lease")
	}
	release()
	w = modelRequest(t, s, "POST", "/api/v2/models/operations/"+repair.ID+"/resume", nil)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	s.downloadWorkers.Wait()
	if s.modelOperations[repair.ID].State != "complete" {
		t.Fatal("repair did not complete", s.modelOperations[repair.ID])
	}
}

func TestModelOperationOwnershipAndManagementPermissions(t *testing.T) {
	s, _, _ := newOperationFixture(t)
	o := submitFixtureOperation(t, s, "request-one")
	s.downloadWorkers.Wait()
	store := users.NewUserStore("")
	reader, key, err := store.CreateUser("reader", "password", users.RoleUser)
	if err != nil {
		t.Fatal(err)
	}
	_, adminKey, err := store.CreateUser("manager", "password", users.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	s.config.RequireAuth = true
	auth := users.NewMiddleware(store)
	auth.SetRequireAuth(true)
	handler := auth.Wrap(http.HandlerFunc(s.handleModelsV2))
	for _, test := range []struct {
		key, method, path string
		status            int
	}{{"", "GET", "/api/v2/models", 401}, {key, "GET", "/api/v2/models", 200}, {key, "GET", "/api/v2/models/operations/" + o.ID, 404}, {key, "POST", "/api/v2/models/operations/" + o.ID + "/resume", 403}, {adminKey, "GET", "/api/v2/models/operations/" + o.ID, 200}} {
		r := httptest.NewRequest(test.method, test.path, nil)
		if test.key != "" {
			r.Header.Set("Authorization", "Bearer "+test.key)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != test.status {
			t.Fatalf("%s: %d want %d %s", test.path, w.Code, test.status, w.Body.String())
		}
	}
	s.modelOperations[o.ID].ActorID = reader.ID
	r := httptest.NewRequest("GET", "/api/v2/models/operations/"+o.ID, nil)
	r.Header.Set("Authorization", "Bearer "+key)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal("owner cannot inspect own snapshot")
	}
}

func TestCuratedPreviewAndLegacyProjection(t *testing.T) {
	s, _, calls := newOperationFixture(t)
	items, err := models.CuratedPackageResolutions()
	if err != nil {
		t.Fatal(err)
	}
	w := modelRequest(t, s, "POST", "/api/v2/models/resolve", map[string]any{"catalog_id": items[0].Manifest.ID})
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var preview modelResolutionRecord
	if json.Unmarshal(w.Body.Bytes(), &preview) != nil || preview.ID == "" || preview.Preflight.RuntimeAvailable || len(preview.Resolution.Manifest.Artifacts) == 0 {
		t.Fatal("bad preview")
	}
	if calls.Load() != 0 {
		t.Fatal("preview downloaded weights")
	}
	w = modelRequest(t, s, "GET", "/api/v2/models/catalog?category=speech_generation", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "kokoro") {
		t.Fatal("shared catalog missing speech")
	}
	w = httptest.NewRecorder()
	s.handleModelCatalog(w, httptest.NewRequest("GET", "/v1/catalog", nil))
	if strings.Contains(w.Body.String(), "kokoro") || strings.Contains(w.Body.String(), "piper") {
		t.Fatal("legacy catalog contaminated")
	}
}

func TestHuggingFaceRepositorySearchDoesNotRequireHubIndex(t *testing.T) {
	s, _, _ := newOperationFixture(t)
	w := modelRequest(t, s, "GET", "/api/v2/models/catalog?category=speech_recognition&source=huggingface&q=Qwen/Qwen3-ASR-0.6B", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Qwen/Qwen3-ASR-0.6B") {
		t.Fatalf("explicit repository was not returned: %d %s", w.Code, w.Body.String())
	}
}
