//go:build !llama

package inference

import (
	"context"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestProductionEmbeddingRejectsMissingModel(t *testing.T) {
	e := NewEmbeddingEngine(t.TempDir())
	if err := e.Load(context.Background(), filepath.Join(t.TempDir(), "missing.gguf"), DefaultEmbeddingOptions()); err == nil {
		t.Fatal("accepted nonexistent model")
	}
	if e.IsLoaded() {
		t.Fatal("failed load marked ready")
	}
}

func TestQueuedEmbeddingCallCanBeCancelled(t *testing.T) {
	e := newTestEmbeddingEngine()
	e.mu.Lock()
	defer e.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := e.GenerateEmbeddings(ctx, nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("queued call ignored cancellation: %v", err)
	}
}

func TestHTTPEmbeddingValidatesAndOrdersVectors(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		bad        bool
	}{
		{"ordered", `{"data":[{"index":1,"embedding":[0,2]},{"index":0,"embedding":[3,4]}],"usage":{"prompt_tokens":9}}`, false},
		{"count", `{"data":[]}`, true},
		{"duplicate", `{"data":[{"index":0,"embedding":[3,4]},{"index":0,"embedding":[3,4]}]}`, true},
		{"dimensions", `{"data":[{"index":0,"embedding":[1]},{"index":1,"embedding":[1,2]}]}`, true},
		{"zero", `{"data":[{"index":0,"embedding":[0,0]},{"index":1,"embedding":[1,2]}]}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/embeddings" || r.Header.Get("Authorization") != "Bearer test" {
					t.Error("invalid request")
				}
				_, _ = w.Write([]byte(tc.body))
			}))
			defer s.Close()
			e := &httpEmbedding{url: s.URL, key: "test", client: s.Client(), normalize: true, dimensions: 2}
			vectors, err := e.Embed(context.Background(), []string{"first", "second"})
			if (err != nil) != tc.bad {
				t.Fatalf("error = %v", err)
			}
			if !tc.bad && (math.Abs(float64(vectors[0][0])-0.6) > 1e-6 || e.UsageTokens() != 9) {
				t.Fatalf("wrong vectors or usage: %v", vectors)
			}
		})
	}
}

func TestHTTPEmbeddingCancellationAndSafeErrors(t *testing.T) {
	release := make(chan struct{})
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer fail" {
			w.WriteHeader(500)
			_, _ = w.Write([]byte("PRIVATE DOCUMENT"))
			return
		}
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer s.Close()
	defer close(release)
	e := &httpEmbedding{url: s.URL, client: s.Client()}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := e.Embed(ctx, []string{"text"}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("cancellation: %v", err)
	}
	e.key = "fail"
	if _, err := e.Embed(context.Background(), []string{"text"}); err == nil || err.Error() != "embedding runtime returned HTTP 500; check model, pooling, and input context limit" {
		t.Fatalf("unsafe error: %v", err)
	}
}
