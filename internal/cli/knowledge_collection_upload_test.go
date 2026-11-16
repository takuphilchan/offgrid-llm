package cli

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

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

func TestCollectionUploadPinsReviewedAudienceReceipt(t *testing.T) {
	file := filepath.Join(t.TempDir(), "private.txt")
	if err := os.WriteFile(file, []byte("Private source"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, revision := range []any{nil, 1, 2, "2", true} {
		calls := 0
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			if r.URL.Path != "/v1/collections/imports" || r.URL.Query().Get("collection_id") != "collection-fixture" || r.URL.Query().Get("collection_revision") != "2" {
				t.Error("wrong destination", r.URL)
			}
			if _, err := io.Copy(io.Discard, r.Body); err != nil {
				t.Error(err)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(202)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "import-fixture", "state": "accepted", "collection_id": "collection-fixture", "collection_revision": revision})
		}))
		client, err := serviceclient.New(s.URL, "", s.Client())
		if err != nil {
			t.Fatal(err)
		}
		_, err = RunKnowledgeService(context.Background(), client, []string{"import-collection", "collection-fixture", "2", file}, nil, io.Discard, true)
		s.Close()
		if (err == nil) != (revision == 2) {
			t.Fatalf("revision %v: %v", revision, err)
		}
		if calls != 1 {
			t.Fatal("mutation replayed", calls)
		}
	}
}

func TestCollectionUploadOCRSelectionCannotSilentlyFallback(t *testing.T) {
	for _, language := range []string{"chi_sim", "chi_sim+fra"} {
		t.Run(language, func(t *testing.T) { testCollectionUploadOCRSelection(t, language) })
	}
}

func testCollectionUploadOCRSelection(t *testing.T, selection string) {
	capability := "document-ocr-language-selection-v1"
	if strings.Contains(selection, "+") {
		capability = "document-ocr-mixed-languages-v1"
	}
	file := filepath.Join(t.TempDir(), "source.txt")
	if err := os.WriteFile(file, []byte("Source evidence"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"supported", "old", "wrong"} {
		t.Run(mode, func(t *testing.T) {
			posts := 0
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					caps := []string{}
					if strings.Contains(selection, "+") {
						caps = append(caps, "document-ocr-language-selection-v1")
					}
					if mode != "old" {
						caps = append(caps, capability)
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"product": "offgrid", "capabilities": caps})
					return
				}
				posts++
				if r.URL.Query().Get("ocr_language") != selection || r.URL.Query().Get("collection_revision") != "2" {
					t.Error("selection or audience lost")
				}
				_, _ = io.Copy(io.Discard, r.Body)
				language := selection
				if mode == "wrong" {
					language = "eng"
				}
				w.WriteHeader(http.StatusAccepted)
				_ = json.NewEncoder(w).Encode(map[string]any{"id": "import-fixture", "state": "accepted", "collection_id": "collection-fixture", "collection_revision": 2, "ocr_language": language})
			}))
			defer s.Close()
			client, err := serviceclient.New(s.URL, "", s.Client())
			if err != nil {
				t.Fatal(err)
			}
			_, err = RunKnowledgeService(context.Background(), client, []string{"import-collection", "collection-fixture", "2", file, "--ocr-language", selection}, nil, io.Discard, true)
			if (err == nil) != (mode == "supported") {
				t.Fatal(mode, err)
			}
			if mode == "old" && posts != 0 || mode != "old" && posts != 1 {
				t.Fatal("unexpected upload/replay", posts)
			}
		})
	}
}
