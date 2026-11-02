package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

func TestKnowledgeOriginalDownloadPreservesBytesAndExistingFiles(t *testing.T) {
	data := []byte("original\x00 café 日本語\r\n")
	digest := sha256.Sum256(data)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer isolated-key" || r.URL.Path != "/v1/documents/source" || r.URL.Query().Get("id") != "id&other=value" || r.URL.Query().Get("format") != "original" || len(r.URL.Query()) != 2 {
			t.Error("download changed authority or source identity")
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("X-OffGrid-Source-SHA256", hex.EncodeToString(digest[:]))
		_, _ = w.Write(data)
	}))
	defer server.Close()
	client, _ := serviceclient.New(server.URL, "isolated-key", server.Client())
	filename := filepath.Join(t.TempDir(), "source 日本語.pdf")
	args := []string{"original", "id&other=value", "--output", filename}
	if _, err := RunKnowledgeService(context.Background(), client, args, nil, io.Discard, true); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filename)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatal("download changed original bytes", err)
	}
	if _, err = RunKnowledgeService(context.Background(), client, args, nil, io.Discard, true); err == nil {
		t.Fatal("existing destination overwritten")
	}
	got, _ = os.ReadFile(filename)
	if !bytes.Equal(got, data) {
		t.Fatal("existing destination changed")
	}
}

func TestKnowledgeOriginalRejectsInvalidResponsesBeforeWriting(t *testing.T) {
	for _, scenario := range []string{"digest", "type", "empty", "oversize", "missing", "cancelled"} {
		t.Run(scenario, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if scenario == "missing" {
					w.WriteHeader(http.StatusGone)
					_, _ = io.WriteString(w, `{"error":{"code":"document_original_unavailable"}}`)
					return
				}
				data := []byte("source")
				if scenario == "empty" {
					data = nil
				}
				if scenario == "oversize" {
					data = make([]byte, (50<<20)+1)
				}
				digest := sha256.Sum256(data)
				w.Header().Set("Content-Type", "application/octet-stream")
				w.Header().Set("X-OffGrid-Source-SHA256", hex.EncodeToString(digest[:]))
				if scenario == "digest" {
					w.Header().Set("X-OffGrid-Source-SHA256", "wrong")
				}
				if scenario == "type" {
					w.Header().Set("Content-Type", "text/html")
				}
				_, _ = w.Write(data)
			}))
			defer server.Close()
			client, _ := serviceclient.New(server.URL, "", server.Client())
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if scenario == "cancelled" {
				cancel()
			}
			filename := filepath.Join(t.TempDir(), "must-not-exist")
			if _, err := downloadKnowledgeOriginal(ctx, client, "id", filename); err == nil {
				t.Fatal("invalid response accepted")
			}
			if _, err := os.Lstat(filename); !os.IsNotExist(err) {
				t.Fatal("invalid response created an output", err)
			}
		})
	}
}
