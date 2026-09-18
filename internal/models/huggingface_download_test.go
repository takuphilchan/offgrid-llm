package models

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

type downloadTransport struct{ server *url.URL }

func (transport downloadTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	copy := request.Clone(request.Context())
	copy.URL.Scheme, copy.URL.Host = transport.server.Scheme, transport.server.Host
	return http.DefaultTransport.RoundTrip(copy)
}

func downloadClient(t *testing.T, handler http.HandlerFunc) *HuggingFaceClient {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	address, _ := url.Parse(server.URL)
	client := NewHuggingFaceClient()
	client.client.Transport = downloadTransport{server: address}
	return client
}

func TestHuggingFaceDownloadFinalization(t *testing.T) {
	content := bytes.Repeat([]byte("model-fixture"), 8192)
	for _, mode := range []string{"fresh", "resume", "complete-partial", "ignored-range"} {
		t.Run(mode, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "模型 — model.gguf")
			offset := 0
			if mode != "fresh" {
				offset = len(content) / 2
				if mode == "complete-partial" {
					offset = len(content)
				}
				if err := os.WriteFile(path+".tmp", content[:offset], 0o600); err != nil {
					t.Fatal(err)
				}
			}
			requests := 0
			client := downloadClient(t, func(w http.ResponseWriter, r *http.Request) {
				requests++
				if offset > 0 && r.Header.Get("Range") != fmt.Sprintf("bytes=%d-", offset) {
					t.Error("missing resume range")
				}
				if mode == "complete-partial" {
					w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", len(content)))
					w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
					return
				}
				if mode == "resume" {
					w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", offset, len(content)-1, len(content)))
					w.WriteHeader(http.StatusPartialContent)
					_, _ = w.Write(content[offset:])
				} else {
					w.Header().Set("Content-Length", fmt.Sprint(len(content)))
					_, _ = w.Write(content)
				}
			})
			var done, total int64
			if err := client.DownloadGGUFContext(context.Background(), "fixture/model", "model.gguf", path, func(d, size int64) { done, total = d, size }); err != nil {
				t.Fatal(err)
			}
			assertDownloadedContent(t, filepath.Dir(path), filepath.Base(path), content)
			if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
				t.Fatalf("partial was not promoted: %v", err)
			}
			if requests != 1 || done != int64(len(content)) || total != done {
				t.Fatalf("requests=%d progress=%d/%d", requests, done, total)
			}
		})
	}
}

func TestHuggingFaceCancelledDownloadRetainsPartial(t *testing.T) {
	content := bytes.Repeat([]byte("x"), 1024*1024)
	client := downloadClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprint(len(content)))
		_, _ = w.Write(content)
	})
	path := filepath.Join(t.TempDir(), "cancelled.gguf")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := client.DownloadGGUFContext(ctx, "fixture/model", "model.gguf", path, func(done, total int64) {
		if done > 0 {
			cancel()
		}
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("cancelled model was published")
	}
	if info, err := os.Stat(path + ".tmp"); err != nil || info.Size() == 0 {
		t.Fatalf("partial missing: %v", err)
	}
}

func TestHuggingFaceTruncatedDownloadDoesNotPublish(t *testing.T) {
	client := downloadClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "1000")
		_, _ = w.Write([]byte("partial"))
	})
	path := filepath.Join(t.TempDir(), "incomplete.gguf")
	if err := client.DownloadGGUF("fixture/model", "model.gguf", path, nil); err == nil {
		t.Fatal("accepted truncated response")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("published truncated model")
	}
}
