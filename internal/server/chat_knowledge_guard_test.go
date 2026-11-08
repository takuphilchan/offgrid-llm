package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/internal/users"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Exercise real persisted collection policy, with isolated retrieval/model
// fixtures so no installed models, documents or service are touched.
func guardedKnowledgeFixture(t *testing.T) (*Server, context.Context, func()) {
	t.Helper()
	e := rag.NewEngine(nil, t.TempDir())
	t.Cleanup(func() { _ = e.Close() })
	store, err := e.ImportStore()
	if err != nil {
		t.Fatal(err)
	}
	doc := &rag.Document{ID: "private", Name: "private.txt", RawContent: "protected evidence", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := store.AddDocument(doc); err != nil {
		t.Fatal(err)
	}
	policy, err := store.CreateCollection("owner", "Private", []string{"local"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.AssignDocumentCollection("owner", doc.ID, policy.ID, 1); err != nil {
		t.Fatal(err)
	}
	s := &Server{ragEngine: e}
	request := &api.ChatCompletionRequest{UseKnowledgeBase: boolPointer(true), Messages: []api.ChatMessage{{Role: "user", Content: "question"}}}
	ctx := s.withChatKnowledgeGuard(context.Background(), request)
	result := &rag.RAGContext{Results: []rag.SearchResult{{DocumentID: doc.ID, Chunk: &rag.Chunk{ID: "chunk", Content: doc.RawContent}}}}
	result.FormatContext()
	if err := injectKnowledge(ctx, request, knowledgeFixture{enabled: true, result: result}); err != nil {
		t.Fatal(err)
	}
	if err := inference.CheckGenerationGuard(ctx); err != nil {
		t.Fatal(err)
	}
	return s, ctx, func() {
		if _, err := store.UpdateCollection("owner", policy.ID, policy.Revision, policy.Name, nil, nil); err != nil {
			t.Fatal(err)
		}
	}
}

func requireKnowledgeDenied(t *testing.T, err error) {
	t.Helper()
	var typed *serviceError
	if !errors.As(err, &typed) || typed.status != 403 || typed.code != "knowledge_unavailable" {
		t.Fatalf("expected current source access error, got %v", err)
	}
}

func TestChatKnowledgeGuardRechecksCurrentAccount(t *testing.T) {
	s, _, _ := guardedKnowledgeFixture(t)
	accounts := users.NewUserStore("")
	user, key, err := accounts.CreateUser("reader", "isolated-password", users.RoleUser)
	if err != nil {
		t.Fatal(err)
	}
	store, err := s.ragEngine.ImportStore()
	if err != nil {
		t.Fatal(err)
	}
	policy, err := store.CreateCollection("owner", "Reader", []string{user.ID}, []string{"owner"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.AssignDocumentCollection("owner", "private", policy.ID, 2); err != nil {
		t.Fatal(err)
	}
	s.config, s.userStore = &config.Config{RequireAuth: true}, accounts
	middleware := users.NewMiddleware(accounts)
	middleware.SetRequireAuth(true)
	called := false
	handler := middleware.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		passages := []sessions.KnowledgePassage{{DocumentID: "private"}}
		ctx := s.withChatKnowledgeGuard(withKnowledgeScope(r.Context(), nil, &passages), &api.ChatCompletionRequest{UseKnowledgeBase: boolPointer(true)})
		if err := inference.CheckGenerationGuard(ctx); err != nil {
			t.Fatal(err)
		}
		if err := accounts.UpdateUser(user.ID, map[string]any{"disabled": true}); err != nil {
			t.Fatal(err)
		}
		requireKnowledgeDenied(t, inference.CheckGenerationGuard(ctx))
	}))
	r := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	r.Header.Set("Authorization", "Bearer "+key)
	handler.ServeHTTP(httptest.NewRecorder(), r)
	if !called {
		t.Fatal("fixture authentication failed")
	}
}

func TestChatKnowledgeGuardPreservesCaptureAndFailsClosed(t *testing.T) {
	s, ctx, revoke := guardedKnowledgeFixture(t)
	revoke()
	requireKnowledgeDenied(t, inference.CheckGenerationGuard(ctx))
	var passages []sessions.KnowledgePassage
	ctx = withKnowledgeScope(context.Background(), []string{"selected"}, &passages)
	ctx = s.withChatKnowledgeGuard(ctx, &api.ChatCompletionRequest{UseKnowledgeBase: boolPointer(true)})
	scope := ctx.Value(knowledgeScopeKey{}).(knowledgeScope)
	if scope.passages != &passages || len(scope.documents) != 1 || scope.documents[0] != "selected" {
		t.Fatal("selected source scope or capture was replaced")
	}
	requireKnowledgeDenied(t, inference.CheckGenerationGuard(ctx))
	plain := s.withChatKnowledgeGuard(context.Background(), &api.ChatCompletionRequest{})
	if err := inference.CheckGenerationGuard(plain); err != nil {
		t.Fatal("ordinary conversation was gated", err)
	}
}

type knowledgeRevokingEngine struct {
	inference.Engine
	revoke func()
	calls  int
}

func (e *knowledgeRevokingEngine) ChatCompletion(context.Context, *api.ChatCompletionRequest) (*api.ChatCompletionResponse, error) {
	e.calls++
	e.revoke()
	return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{Message: api.ChatMessage{Content: "withheld answer"}}}}, nil
}

func (e *knowledgeRevokingEngine) ChatCompletionStream(_ context.Context, _ *api.ChatCompletionRequest, emit inference.TokenCallback) error {
	e.calls++
	if err := emit("before revocation"); err != nil {
		return err
	}
	e.revoke()
	return emit("withheld answer")
}

type knowledgeRevokingRawEngine struct{ *knowledgeRevokingEngine }

func (e *knowledgeRevokingRawEngine) ChatCompletionStreamRaw(_ context.Context, _ *api.ChatCompletionRequest, emit inference.ChatCompletionStreamCallback) error {
	e.calls++
	if err := emit(json.RawMessage(`{"choices":[{"delta":{"content":"before revocation"},"finish_reason":null}]}`)); err != nil {
		return err
	}
	e.revoke()
	return emit(json.RawMessage(`{"choices":[{"delta":{"content":"withheld answer"},"finish_reason":"stop"}]}`))
}

func TestLegacyChatRevocationStopsDeliveryAndReleasesLease(t *testing.T) {
	for _, mode := range []string{"complete", "session stream", "HTTP stream", "raw HTTP stream", "raw session stream", "before dispatch"} {
		t.Run(mode, func(t *testing.T) {
			s, ctx, revoke := guardedKnowledgeFixture(t)
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "model.gguf"), []byte("fixture"), 0600); err != nil {
				t.Fatal(err)
			}
			s.registry = models.NewRegistry(dir)
			if err := s.registry.ScanModels(); err != nil {
				t.Fatal(err)
			}
			s.config = &config.Config{MaxContextSize: 8192, ChatContextSize: 8192}
			gate := inference.NewLifecycleGate(1)
			s.inferenceLifecycle = gate
			release, err := gate.AcquireWithContext(ctx, "model", 8192, func(context.Context, string) error { return nil })
			if err != nil {
				t.Fatal(err)
			}
			release()
			engine := &knowledgeRevokingEngine{revoke: revoke}
			s.engine = engine
			if strings.HasPrefix(mode, "raw ") {
				s.engine = &knowledgeRevokingRawEngine{engine}
			}
			// The context already carries captured source provenance from retrieval.
			request := &api.ChatCompletionRequest{Model: "model", Messages: []api.ChatMessage{{Role: "user", Content: "question"}}}
			switch mode {
			case "complete", "before dispatch":
				if mode == "before dispatch" {
					revoke()
				}
				response, err := s.completeChat(ctx, request)
				requireKnowledgeDenied(t, err)
				if response != nil {
					t.Fatal("revoked answer returned")
				}
			case "session stream", "raw session stream":
				var text strings.Builder
				result, err := s.streamSessionChat(ctx, request, "interactive", func(event ChatStreamEvent) error { text.WriteString(event.Delta); return nil })
				requireKnowledgeDenied(t, err)
				if text.String() != "before revocation" || result.Answer != "" {
					t.Fatalf("revoked output: %+v %q", result, text.String())
				}
			case "HTTP stream", "raw HTTP stream":
				request.Stream = true
				w := httptest.NewRecorder()
				r := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model":"model","stream":true,"messages":[{"role":"user","content":"question"}]}`)).WithContext(ctx)
				s.handleChatCompletions(w, r)
				body := w.Body.String()
				if !strings.Contains(body, "knowledge_unavailable") || strings.Contains(body, "withheld answer") || strings.Contains(body, "[DONE]") {
					t.Fatal(body)
				}
			}
			wantCalls := 1
			if mode == "before dispatch" {
				wantCalls = 0
			}
			if engine.calls != wantCalls || gate.Status().Active != 0 {
				t.Fatalf("calls=%d lease=%+v", engine.calls, gate.Status())
			}
		})
	}
}

type revokingNativeRuntime struct {
	nativeChatRuntimeFixture
	revoke func()
}

func (f *revokingNativeRuntime) Generate(ctx context.Context, request inference.ProviderRequest, emit func(inference.ProviderEvent) error) error {
	return f.nativeChatRuntimeFixture.Generate(ctx, request, func(event inference.ProviderEvent) error {
		if event.Type == inference.ProviderEventDelta {
			f.revoke()
		}
		return emit(event)
	})
}

func TestNativeChatRevocationPreservesAccessError(t *testing.T) {
	for _, mode := range []string{"complete", "session", "HTTP", "loading"} {
		t.Run(mode, func(t *testing.T) {
			s, ctx, revoke := guardedKnowledgeFixture(t)
			d := inference.ProviderDescriptor{ProtocolRevision: inference.ProviderProtocolRevision, Provider: inference.ProviderTransformers, RunnerRevision: "fixture", AdapterRevision: "fixture", ModelDigest: "sha256:fixture", Format: inference.FormatSafetensors, Backend: "cpu", Capabilities: []inference.ProviderCapability{inference.CapabilityText, inference.CapabilityStreaming}}
			runtime := &revokingNativeRuntime{nativeChatRuntimeFixture: nativeChatRuntimeFixture{descriptor: d}, revoke: revoke}
			registry := inference.NewProviderRegistry()
			if err := registry.Register(d.Provider, func(context.Context, inference.ProviderDescriptor) (inference.ProviderRuntime, error) {
				if mode == "loading" {
					revoke()
				}
				return runtime, nil
			}); err != nil {
				t.Fatal(err)
			}
			var err error
			s.providerScheduler, err = inference.NewProviderScheduler(registry, 1)
			if err != nil {
				t.Fatal(err)
			}
			defer s.providerScheduler.Close()
			request := &api.ChatCompletionRequest{Model: "fixture@r1", Messages: []api.ChatMessage{{Role: "user", Content: "question"}}}
			switch mode {
			case "complete", "loading":
				response, err := s.completeNativeChat(ctx, request, d)
				requireKnowledgeDenied(t, err)
				if response != nil {
					t.Fatal("revoked answer returned")
				}
			case "session":
				_, err := s.streamNativeSessionChat(ctx, request, d, time.Now(), func(event ChatStreamEvent) error {
					if event.Type == "delta" {
						t.Fatal("revoked text emitted")
					}
					return nil
				})
				requireKnowledgeDenied(t, err)
			case "HTTP":
				w := httptest.NewRecorder()
				s.handleNativeChatCompletionsStream(w, httptest.NewRequest("POST", "/v1/chat/completions", nil).WithContext(ctx), request, d)
				body := w.Body.String()
				if !strings.Contains(body, "knowledge_unavailable") || strings.Contains(body, "fixture answer") || strings.Contains(body, "[DONE]") {
					t.Fatal(body)
				}
			}
			wantCalls := 1
			if mode == "loading" {
				wantCalls = 0
			}
			if runtime.called != wantCalls {
				t.Fatalf("calls=%d", runtime.called)
			}
		})
	}
}
