package models

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
)

func TestDiscoveryMetadataDoesNotFetchEveryRepository(t *testing.T) {
	requests := 0
	client := downloadClient(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/api/models" {
			t.Errorf("unexpected eager request: %s", r.URL)
			http.Error(w, "bad", 500)
			return
		}
		w.Write([]byte(`[{"id":"owner/huge-GGUF","tags":["gguf"],"gated":false},{"id":"owner/gated","tags":["gguf"],"gated":"auto"},{"id":"owner/manual","tags":["gguf"],"gated":"manual"}]`))
	})
	result, err := client.SearchModelsContext(context.Background(), SearchFilter{Query: "huge", OnlyGGUF: true, ExcludeGated: true, MetadataOnly: true})
	if err != nil || len(result) != 1 || requests != 1 {
		t.Fatalf("results=%v requests=%d err=%v", result, requests, err)
	}
}

func TestDiscoveryFilesKeepLargeChoicesAndIsolateRepositories(t *testing.T) {
	client := downloadClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("recursive") != "true" {
			t.Error("missing nested files")
		}
		json.NewEncoder(w).Encode([]map[string]any{
			{"type": "file", "path": "weights/model-Q8_0.gguf", "size": int64(80) << 30},
			{"type": "file", "path": "weights/model-Q4_K_M.gguf", "size": int64(40) << 30},
			{"type": "file", "path": "model-00001-of-00002.gguf", "size": 100},
			{"type": "file", "path": "mmproj.gguf", "size": 100},
			{"type": "file", "path": "README.md", "size": 100},
		})
	})
	first, err := client.DiscoverFiles(context.Background(), "owner/large")
	if err != nil || len(first) != 4 {
		t.Fatalf("files=%v err=%v", first, err)
	}
	if !first[0].Supported || first[0].Size != 80<<30 || first[0].Quant != "Q8_0" {
		t.Fatalf("large model was limited: %+v", first[0])
	}
	if first[2].Supported || first[3].Supported {
		t.Fatal("companion file advertised as standalone")
	}
	second, _ := client.DiscoverFiles(context.Background(), "another/large")
	if first[0].ID == second[0].ID {
		t.Fatal("same file name merged repository identities")
	}
	third, _ := client.DiscoverFiles(context.Background(), "owner/large")
	if first[0].ID != third[0].ID {
		t.Fatal("unstable download identity")
	}
}

func TestDiscoveryCancellationAndUpstreamFailures(t *testing.T) {
	client := downloadClient(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) })
	if _, err := client.SearchModels(SearchFilter{MetadataOnly: true}); err == nil {
		t.Fatal("upstream failure disguised as empty search")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.SearchModelsContext(ctx, SearchFilter{MetadataOnly: true}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
	for _, repo := range []string{"../secret", "owner/repo?x=y", "https://example.com", "owner/repo/extra", "owner/../../bad"} {
		if _, err := client.DiscoverFiles(context.Background(), repo); err == nil {
			t.Errorf("accepted repository %q", repo)
		}
	}
}
