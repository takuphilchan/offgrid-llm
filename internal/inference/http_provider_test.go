package inference

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func httpProviderFixture(t *testing.T, id ProviderID, generate http.HandlerFunc) (HTTPEndpoint, *atomic.Int32) {
	t.Helper()
	var released atomic.Int32
	digest := "sha256:" + strings.Repeat("a", 64)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fixture-credential" {
			t.Error("credential missing")
		}
		switch r.URL.Path {
		case "/api/version", "/version":
			fmt.Fprint(w, `{"version":"fixture-1"}`)
		case "/api/tags":
			fmt.Fprintf(w, `{"models":[{"name":"fixture-model","digest":%q}]}`, strings.TrimPrefix(digest, "sha256:"))
		case "/v1/models":
			fmt.Fprint(w, `{"data":[{"id":"fixture-model"}]}`)
		default:
			generate(w, r)
		}
	}))
	t.Cleanup(server.Close)
	d := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: id, RunnerRevision: "fixture-1", AdapterRevision: HTTPEndpointAdapterRevision, ModelDigest: digest, Format: FormatGGUF, Backend: "fixture-owned", ContextLength: 8192, Capabilities: []ProviderCapability{CapabilityText, CapabilityStreaming, CapabilityToolCalls}}
	if id == ProviderVLLM {
		d.Format = FormatSafetensors
	}
	return HTTPEndpoint{URL: server.URL, Model: "fixture-model", RuntimeVersion: "fixture-1", AllowedIPs: []string{"127.0.0.1"}, Descriptor: d, Authorize: func(context.Context) (string, error) { return "fixture-credential", nil }, AcquireLocal: func(context.Context) (func() error, error) { return func() error { released.Add(1); return nil }, nil }}, &released
}

func httpProviderRequest(t *testing.T, endpoint HTTPEndpoint, tools []api.Tool) ProviderRequest {
	t.Helper()
	maxTokens := 64
	raw, err := json.Marshal(api.ChatCompletionRequest{Model: "public-alias", Messages: []api.ChatMessage{{Role: "user", Content: "hello"}}, MaxTokens: &maxTokens, Tools: tools})
	if err != nil {
		t.Fatal(err)
	}
	var toolJSON []byte
	if len(tools) > 0 {
		toolJSON, _ = json.Marshal(tools)
	}
	return ProviderRequest{RequestID: "request-1", CancellationID: "cancel-1", ModelDigest: endpoint.Descriptor.ModelDigest, ContextLength: 8192, MaxOutputTokens: 64, MaxOutputBytes: 4096, Prompt: raw, Tools: toolJSON}
}

func TestHTTPProviderRejectsRevocationAndCancellationBeforeLocalLaunch(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		endpoint, _ := httpProviderFixture(t, ProviderOllama, func(http.ResponseWriter, *http.Request) { t.Error("generation dispatched") })
		launched := false
		endpoint.AcquireLocal = func(context.Context) (func() error, error) {
			launched = true
			return func() error { return nil }, nil
		}
		ctx := context.Background()
		if cancelled {
			var cancel context.CancelFunc
			ctx, cancel = context.WithCancel(ctx)
			cancel()
		} else {
			endpoint.Authorize = func(context.Context) (string, error) { return "", errors.New("revoked") }
		}
		if provider, err := NewHTTPProvider(ctx, endpoint); err == nil || provider != nil || launched {
			t.Fatal("cancelled or unauthorized request acquired a local process lease")
		}
	}
}

func TestHTTPProvidersNormalizeRealWireStreamsThroughScheduler(t *testing.T) {
	for _, id := range []ProviderID{ProviderOllama, ProviderVLLM} {
		t.Run(string(id), func(t *testing.T) {
			endpoint, released := httpProviderFixture(t, id, func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				if json.NewDecoder(r.Body).Decode(&body) != nil || body["model"] != "fixture-model" || body["stream"] != true {
					t.Error("upstream request not normalized")
				}
				if id == ProviderOllama {
					fmt.Fprintln(w, `{"model":"fixture-model","message":{"role":"assistant","content":"Hello"},"done":false}`)
					fmt.Fprintln(w, `{"model":"fixture-model","message":{"content":"!"},"done":true,"done_reason":"stop","prompt_eval_count":3,"eval_count":2}`)
				} else {
					fmt.Fprint(w, "data: {\"model\":\"fixture-model\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Hello!\"}}]}\n\n")
					fmt.Fprint(w, "data: {\"model\":\"fixture-model\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n")
					fmt.Fprint(w, "data: {\"model\":\"fixture-model\",\"choices\":[],\"usage\":{\"prompt_tokens\":3,\"completion_tokens\":2,\"total_tokens\":5}}\n\ndata: [DONE]\n\n")
				}
			})
			routes, err := NewHTTPEndpointRoutes(map[string]HTTPEndpoint{"fixture@r1": endpoint})
			if err != nil {
				t.Fatal(err)
			}
			registry := NewProviderRegistry()
			if err = routes.Register(registry); err != nil {
				t.Fatal(err)
			}
			scheduler, err := NewProviderScheduler(registry, 1)
			if err != nil {
				t.Fatal(err)
			}
			defer scheduler.Close()
			var text strings.Builder
			var done bool
			_, err = scheduler.GenerateTimed(context.Background(), endpoint.Descriptor, httpProviderRequest(t, endpoint, nil), func(event ProviderEvent) error {
				if event.Type == ProviderEventDelta {
					text.WriteString(event.Text)
				}
				if event.Type == ProviderEventDone {
					done = true
				}
				return nil
			})
			if err != nil || text.String() != "Hello!" || !done {
				t.Fatalf("text=%q done=%v error=%v", text.String(), done, err)
			}
			if err = scheduler.Close(); err != nil || released.Load() != 1 {
				t.Fatalf("release=%d error=%v", released.Load(), err)
			}
		})
	}
}

func TestHTTPProviderRejectsTruncationAndLateToolFailure(t *testing.T) {
	for _, id := range []ProviderID{ProviderOllama, ProviderVLLM} {
		t.Run(string(id), func(t *testing.T) {
			endpoint, _ := httpProviderFixture(t, id, func(w http.ResponseWriter, r *http.Request) {
				if id == ProviderOllama {
					fmt.Fprintln(w, `{"model":"fixture-model","message":{"tool_calls":[{"function":{"name":"read","arguments":{"path":"x"}}}]},"done":false}`)
				} else {
					fmt.Fprint(w, "data: {\"model\":\"fixture-model\",\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"a\",\"type\":\"function\",\"function\":{\"name\":\"read\",\"arguments\":\"{\\\"path\\\":\\\"x\\\"}\"}}]},\"finish_reason\":\"tool_calls\"}]}\n\n")
				}
			})
			p, err := NewHTTPProvider(context.Background(), endpoint)
			if err != nil {
				t.Fatal(err)
			}
			defer p.Close()
			tools := []api.Tool{{Type: "function", Function: api.FunctionDef{Name: "read", Parameters: map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string"}}, "required": []string{"path"}}}}}
			var toolDelivered bool
			err = p.Generate(context.Background(), httpProviderRequest(t, endpoint, tools), func(event ProviderEvent) error {
				toolDelivered = toolDelivered || event.Type == ProviderEventTool
				return nil
			})
			if err == nil || toolDelivered {
				t.Fatalf("truncated tool delivered=%v error=%v", toolDelivered, err)
			}
		})
	}
}

func TestHTTPProviderCancellationAndRevocationDoNotReplay(t *testing.T) {
	started := make(chan struct{})
	var calls atomic.Int32
	endpoint, _ := httpProviderFixture(t, ProviderOllama, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		close(started)
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(200)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	})
	var revoked atomic.Bool
	endpoint.Authorize = func(context.Context) (string, error) {
		if revoked.Load() {
			return "", errors.New("revoked")
		}
		return "fixture-credential", nil
	}
	p, err := NewHTTPProvider(context.Background(), endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	req := httpProviderRequest(t, endpoint, nil)
	done := make(chan error, 1)
	go func() { done <- p.Generate(context.Background(), req, func(ProviderEvent) error { return nil }) }()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("generation not started")
	}
	if err = p.Cancel(context.Background(), req.CancellationID); err != nil {
		t.Fatal(err)
	}
	select {
	case err = <-done:
		if err == nil {
			t.Fatal("cancelled request succeeded")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancel did not stop read")
	}
	if err = p.Generate(context.Background(), req, func(ProviderEvent) error { return nil }); err == nil {
		t.Fatal("request replayed")
	}
	revoked.Store(true)
	req.RequestID = "request-2"
	if err = p.Generate(context.Background(), req, func(ProviderEvent) error { return nil }); err == nil {
		t.Fatal("revoked request admitted")
	}
	if calls.Load() != 1 {
		t.Fatalf("dispatched %d requests", calls.Load())
	}
}

func TestHTTPProviderEndpointScopeAndOwnership(t *testing.T) {
	endpoint, _ := httpProviderFixture(t, ProviderOllama, func(w http.ResponseWriter, r *http.Request) { t.Fatal("generation was not authorized") })
	for _, mutate := range []func(*HTTPEndpoint){func(e *HTTPEndpoint) { e.AcquireLocal = nil }, func(e *HTTPEndpoint) { e.URL += "/v1" }, func(e *HTTPEndpoint) { e.Remote = true }, func(e *HTTPEndpoint) { e.AllowedIPs = []string{"169.254.169.254"} }, func(e *HTTPEndpoint) { e.AllowedIPs = []string{"192.168.1.1"} }, func(e *HTTPEndpoint) { e.RuntimeVersion = "changed" }, func(e *HTTPEndpoint) { e.Descriptor.ModelDigest = "sha256:" + strings.Repeat("b", 64) }} {
		changed := endpoint
		mutate(&changed)
		if p, err := NewHTTPProvider(context.Background(), changed); err == nil {
			p.Close()
			t.Fatal("invalid endpoint accepted")
		}
	}
}
