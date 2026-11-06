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

	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/models"
)

func TestVisionDownloadCompletesAllAssetsAndRepairsMissing(t *testing.T) {
	dir := t.TempDir()
	revision := strings.Repeat("a", 40)
	contents := map[string]string{"family-Q4_K_M.gguf": "fixture weights", "mmproj-F16.gguf": "fixture projector"}
	var listing []map[string]any
	for name, data := range contents {
		digest := sha256.Sum256([]byte(data))
		listing = append(listing, map[string]any{"type": "file", "path": name, "size": len(data), "lfs": map[string]any{"oid": hex.EncodeToString(digest[:]), "size": len(data)}})
	}
	weightFetches := 0
	hf := models.NewHuggingFaceClient().WithHTTPClient(&http.Client{Transport: modelTestTransport(func(r *http.Request) (*http.Response, error) {
		var body []byte
		switch {
		case strings.Contains(r.URL.Path, "/tree/"):
			body, _ = json.Marshal(listing)
		case strings.Contains(r.URL.Path, "/api/models/"):
			body, _ = json.Marshal(map[string]any{"id": "fixture/vision", "sha": revision, "pipeline_tag": "image-text-to-text"})
		default:
			if !strings.Contains(r.URL.Path, "/resolve/"+revision+"/") {
				t.Errorf("mutable transfer: %s", r.URL)
			}
			name := filepath.Base(r.URL.Path)
			body = []byte(contents[name])
			if name == "family-Q4_K_M.gguf" {
				weightFetches++
			}
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, ContentLength: int64(len(body)), Body: io.NopCloser(bytes.NewReader(body)), Request: r}, nil
	})})
	s := &Server{config: &config.Config{DataDir: dir, ModelsDir: filepath.Join(dir, "models")}, registry: models.NewRegistry(filepath.Join(dir, "models")), runtimeCtx: context.Background(), modelHub: hf}
	request := func() {
		r := httptest.NewRequest(http.MethodPost, "/v1/models/download", strings.NewReader(`{"repository":"fixture/vision","file_name":"family-Q4_K_M.gguf","model_id":"local-model"}`))
		w := httptest.NewRecorder()
		s.handleDownloadModel(w, r)
		if w.Code != 200 {
			t.Fatalf("admission: %d %s", w.Code, w.Body)
		}
		s.downloadWorkers.Wait()
		p := s.downloadProgress["local-model.gguf"]
		if p == nil || p.Status != "complete" || p.Bundle == nil || p.BytesDone != int64(len(contents["family-Q4_K_M.gguf"])+len(contents["mmproj-F16.gguf"])) {
			t.Fatalf("incomplete operation: %+v", p)
		}
	}
	request()
	model, err := s.registry.GetModel("local-model")
	if err != nil || model.ProjectorPath == "" {
		t.Fatalf("missing installed vision assets: %+v %v", model, err)
	}
	// Restart reuses the pinned bundle, even if the upstream default moves.
	if err := s.loadDownloads(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(model.ProjectorPath); err != nil {
		t.Fatal(err)
	}
	request()
	if weightFetches != 1 {
		t.Fatalf("repair redownloaded unchanged weights %d times", weightFetches)
	}
	if _, err := os.Stat(model.ProjectorPath); err != nil {
		t.Fatal(fmt.Errorf("repair failed: %w", err))
	}
	// A retained source binding cannot be replaced by a new request using the
	// same local ID, even if the new source advertises identical weights.
	p := s.downloadProgress["local-model.gguf"]
	oldBundle := *p.Bundle
	oldBundle.Weights = p.Bundle.Weights
	oldSource := *oldBundle.Weights.Source
	oldSource.Path = "earlier-Q4_K_M.gguf"
	oldBundle.Weights.Source = &oldSource
	oldBundle.Weights.Path = oldSource.Path
	p.Bundle, p.SourceFile = &oldBundle, oldSource.Path
	r := httptest.NewRequest(http.MethodPost, "/v1/models/download", strings.NewReader(`{"repository":"fixture/vision","file_name":"family-Q4_K_M.gguf","model_id":"local-model"}`))
	w := httptest.NewRecorder()
	s.handleDownloadModel(w, r)
	if w.Code != http.StatusConflict || s.downloadProgress["local-model.gguf"].SourceFile != oldSource.Path {
		t.Fatalf("source binding was replaced: %d %s", w.Code, w.Body)
	}
}

func TestCatalogDownloadUsesReviewedRevisionAndRejectsChangedDigest(t *testing.T) {
	dir := t.TempDir()
	entry := models.DefaultCatalog().FindModel("qwen3.5-2b")
	file := "Qwen3.5-2B-Q4_K_M.gguf"
	repo := "unsloth/Qwen3.5-2B-GGUF"
	revision, _ := models.DefaultCatalog().PinnedSource(entry.ID, repo, file)
	fetches := 0
	hf := models.NewHuggingFaceClient().WithHTTPClient(&http.Client{Transport: modelTestTransport(func(r *http.Request) (*http.Response, error) {
		fetches++
		if !strings.Contains(r.URL.Path, revision) || strings.Contains(r.URL.Path, "/resolve/") {
			t.Errorf("mutable metadata or premature transfer: %s", r.URL)
		}
		var value any
		if strings.Contains(r.URL.Path, "/tree/") {
			value = []map[string]any{}
			for _, name := range []string{file, "mmproj-F16.gguf"} {
				value = append(value.([]map[string]any), map[string]any{"type": "file", "path": name, "size": 12, "lfs": map[string]any{"oid": strings.Repeat("b", 64), "size": 12}})
			}
		} else {
			value = map[string]any{"id": repo, "sha": revision, "pipeline_tag": "image-text-to-text"}
		}
		body, _ := json.Marshal(value)
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(body)), Request: r}, nil
	})})
	s := &Server{config: &config.Config{ModelsDir: dir}, modelHub: hf}
	body, _ := json.Marshal(map[string]string{"model_id": entry.ID, "repository": repo, "file_name": file})
	w := httptest.NewRecorder()
	s.handleDownloadModel(w, httptest.NewRequest("POST", "/v1/models/download", bytes.NewReader(body)))
	if fetches == 0 || w.Code != 422 || !strings.Contains(w.Body.String(), "catalog_digest_mismatch") {
		t.Fatalf("changed catalog artifact accepted: %d %s", w.Code, w.Body)
	}
	files, _ := os.ReadDir(dir)
	if len(files) != 0 || len(s.downloadProgress) != 0 {
		t.Fatal("rejected artifact changed model storage")
	}
}
