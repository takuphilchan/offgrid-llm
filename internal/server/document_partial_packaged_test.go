package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/cli"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
	"github.com/takuphilchan/offgrid-llm/internal/users"
)

func TestPackagedPartialImportReview(t *testing.T) {
	if os.Getenv("OFFGRID_TEST_PACKAGED_DOCUMENTS") != "1" {
		t.Skip("requires real app-local PDF and OCR workers")
	}
	cache := t.TempDir()
	for _, key := range []string{"LOCALAPPDATA", "XDG_CACHE_HOME", "HOME"} {
		t.Setenv(key, cache)
	}
	engine := rag.NewEngine(nil, t.TempDir())
	defer engine.Close()
	s := &Server{ragEngine: engine}
	us := users.NewUserStore("")
	_, key, err := us.CreateUser("owner", "isolated-password", users.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	_, other, err := us.CreateUser("other", "isolated-password", users.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	mw := users.NewMiddleware(us)
	mw.SetRequireAuth(true)
	handler := mw.Wrap(users.RequirePermission(users.PermissionRAGManage)(http.HandlerFunc(s.handleDocumentImports)))
	data, err := os.ReadFile(filepath.Join("..", "rag", "testdata", "documents", "corpus", "mixed.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	upload := documentUploadRequest(t, "mixed.pdf", data)
	upload.URL.Path = "/v1/documents/imports"
	upload.Header.Set("Authorization", "Bearer "+key)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, upload)
	var record rag.ImportRecord
	if w.Code != 202 || json.Unmarshal(w.Body.Bytes(), &record) != nil {
		t.Fatal(w.Code, w.Body.String())
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	client, _ := serviceclient.New(server.URL, key, server.Client())
	if _, err = cli.RunKnowledgeService(context.Background(), client, []string{"retry-import", record.ID}, nil, io.Discard, true); err == nil {
		t.Fatal("mixed file indexed without OCR language")
	}
	if err = client.JSON(context.Background(), "GET", "/v1/documents/imports?id="+record.ID, nil, &record); err != nil || !record.ReviewAvailable || record.State != "needs_input" {
		t.Fatal(record, err)
	}
	result, err := cli.RunKnowledgeService(context.Background(), client, []string{"review-import", record.ID}, nil, io.Discard, true)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(result)
	var review rag.ImportExtraction
	if json.Unmarshal(encoded, &review) != nil || review.SHA256 != record.SHA256 || review.State != rag.ExtractionPartial || len(review.Units) != 2 {
		t.Fatal(string(encoded))
	}
	if review.Units[0].State != "ready" || !strings.Contains(review.Units[0].Text, "Native page") || review.Units[1].State != "ocr_required" {
		t.Fatal(review.Units)
	}
	otherClient, _ := serviceclient.New(server.URL, other, server.Client())
	if _, err = cli.RunKnowledgeService(context.Background(), otherClient, []string{"review-import", record.ID}, nil, io.Discard, true); err == nil {
		t.Fatal("cross-user extraction read")
	}
	if len(engine.ListDocuments()) != 0 {
		t.Fatal("partial file published to retrieval")
	}
	if _, err = cli.RunKnowledgeService(context.Background(), client, []string{"remove-import", record.ID}, nil, io.Discard, true); err != nil {
		t.Fatal(err)
	}
	if _, err = cli.RunKnowledgeService(context.Background(), client, []string{"review-import", record.ID}, nil, io.Discard, true); err == nil {
		t.Fatal("removed extraction readable")
	}
}
