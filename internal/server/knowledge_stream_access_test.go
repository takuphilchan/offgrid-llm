package server

import (
	"context"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestKnowledgeRevocationDuringSessionGenerationNeverPublishes(t *testing.T) {
	for _, mode := range []string{"ordinary", "stream", "durable"} {
		t.Run(mode, func(t *testing.T) {
			h := NewSessionHandlers(t.TempDir())
			t.Cleanup(h.closeTurns)
			var allowed atomic.Bool
			allowed.Store(true)
			h.knowledgeAccess = func(string, []string) bool { return allowed.Load() }
			if _, err := h.manager.WithAccess(sessions.Access{All: true}).Create("chat", "model"); err != nil {
				t.Fatal(err)
			}
			revoke := func(ctx context.Context) {
				scope := ctx.Value(knowledgeScopeKey{}).(knowledgeScope)
				*scope.passages = []sessions.KnowledgePassage{{DocumentID: "private", Content: "private source"}}
				allowed.Store(false)
			}
			h.completer = func(ctx context.Context, _ string, _ []api.ChatMessage, _ bool) (string, error) {
				revoke(ctx)
				return "secret answer", nil
			}
			h.streamer = func(ctx context.Context, _ *api.ChatCompletionRequest, _ string, emit func(ChatStreamEvent) error) (ChatStreamResult, error) {
				revoke(ctx)
				err := emit(ChatStreamEvent{Type: "delta", Delta: "secret answer"})
				if err == nil {
					t.Error("revoked delta accepted")
				}
				return ChatStreamResult{Answer: "secret answer"}, err
			}
			body := `{"content":"question"}`
			if mode == "stream" {
				body = `{"content":"question","stream":true}`
			}
			if mode == "durable" {
				body = `{"content":"question","stream":true,"request_id":"turn-one"}`
			}
			w := httptest.NewRecorder()
			h.HandleSessions(w, httptest.NewRequest("POST", "/v1/sessions/chat/generate", strings.NewReader(body)))
			if strings.Contains(w.Body.String(), "secret answer") || strings.Contains(w.Body.String(), "private source") {
				t.Fatal("revoked output leaked", w.Body.String())
			}
			stored, err := h.manager.Load("chat")
			if err != nil || len(stored.Messages) != 0 {
				t.Fatal("revoked answer persisted", stored, err)
			}
			if mode == "durable" && stored.Turn != nil && stored.Turn.Status == "completed" {
				t.Fatal("false completed turn")
			}
		})
	}
}
