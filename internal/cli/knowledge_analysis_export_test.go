package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestKnowledgeAnalysisExportNoOverwriteOrUnverifiedFile(t *testing.T) {
	for _, command := range []string{"analysis-export", "synthesis-export"} {
		t.Run(command, func(t *testing.T) { testKnowledgeExport(t, command) })
	}
}

func testKnowledgeExport(t *testing.T, command string) {
	content := "政策 17.25"
	hash := sha256.Sum256([]byte(content))
	digest := hex.EncodeToString(hash[:])
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		path := "/v1/document-analyses/export"
		if command == "synthesis-export" {
			path = "/v1/document-analyses/synthesis/export"
		}
		if r.URL.Path != path {
			t.Error("wrong export endpoint", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(serviceclient.DocumentAnalysisExport{AnalysisID: "job", Format: "markdown", Content: content, SHA256: digest})
	}))
	defer server.Close()
	client, err := serviceclient.New(server.URL, "key", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(t.TempDir(), "comparison.md")
	args := []string{command, "job", "markdown", "--output", filename}
	if _, err = RunKnowledgeService(context.Background(), client, args, nil, nil, true); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filename)
	if err != nil || string(data) != content {
		t.Fatal(string(data), err)
	}
	if _, err = RunKnowledgeService(context.Background(), client, args, nil, nil, true); err == nil {
		t.Fatal("overwrote existing file")
	}
	digest = "forged"
	args[4] = filepath.Join(t.TempDir(), "missing.md")
	if _, err = RunKnowledgeService(context.Background(), client, args, nil, nil, true); err == nil {
		t.Fatal("forged file accepted")
	}
	if _, err = os.Stat(args[4]); !os.IsNotExist(err) {
		t.Fatal("created file before validation", err)
	}
	if calls != 3 {
		t.Fatal("export replayed", calls)
	}
}
