//go:build !llama

package inference

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestDocumentRuntimeDoesNotDownloadMissingAssets(t *testing.T) {
	t.Setenv("PATH", "")
	t.Setenv("OFFGRID_LLAMA_SERVER_PATH", "")
	bm := &BinaryManager{version: "missing-fixture", binDir: t.TempDir(), serviceExecutable: filepath.Join(t.TempDir(), "offgrid.exe")}
	_, err := bm.GetLlamaServerContext(WithLocalRuntimeOnly(context.Background()))
	if err == nil || !strings.Contains(err.Error(), "automatic download is disabled") {
		t.Fatal("missing runtime attempted acquisition", err)
	}
}

func TestDocumentGenerationIsBoundedAndNeverRetried(t *testing.T) {
	for _, mode := range []string{"busy", "oversized", "duplicate", "valid"} {
		t.Run(mode, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				switch mode {
				case "busy":
					w.WriteHeader(503)
				case "oversized":
					w.Write([]byte(`{"choices":[{"message":{"content":"` + strings.Repeat("x", (4<<20)+1) + `"}}]}`))
				case "duplicate":
					w.Write([]byte(`{"choices":[],"choices":[]}`))
				default:
					w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"{}"},"finish_reason":"stop"}]}`))
				}
			}))
			defer server.Close()
			engine := NewLlamaHTTPEngine(server.URL)
			_, err := engine.ChatCompletion(WithBoundedDocumentGeneration(context.Background()), &api.ChatCompletionRequest{Messages: []api.ChatMessage{{Role: "user", Content: "fixture"}}})
			if (err == nil) != (mode == "valid") || calls.Load() != 1 {
				t.Fatal(mode, calls.Load(), err)
			}
		})
	}
}
