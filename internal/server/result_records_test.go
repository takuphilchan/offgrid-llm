package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestCanonicalResultHTTPPagingRevocationAndObservation(t *testing.T) {
	s, scope, conversation := contextInboxFixture(t)
	answer := strings.Repeat("Evidence \u754c. ", 1200) + "Final citation [source-42]."
	calls := 0
	s.agentRunner = agents.NewRunner(s.agentManager, s.toolRegistry, func(context.Context, *agents.Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "stop", Message: api.ChatMessage{Role: "assistant", Content: answer}}}}, nil
	})
	defer s.agentRunner.Shutdown(context.Background())
	input := acceptContextInput(t, s, scope, conversation.ID, "saved-result", "Preserve the citations")
	completeContextInput(t, s, scope, input, agents.DefaultAgentConfig())
	input, err := s.agentManager.ConversationInput(scope, conversation.ID, input.ID)
	if err != nil || input.ResultReference == "" {
		t.Fatal(input, err)
	}
	path := "/api/v2/results/" + url.PathEscape(input.ResultReference)
	read := func(server *Server, query string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		server.handleResultRecord(w, httptest.NewRequest("GET", path+query, nil))
		return w
	}
	var assembled strings.Builder
	for offset := 0; ; {
		response := read(s, "?offset="+strconv.Itoa(offset)+"&limit=101")
		var page api.ResultPage
		if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &page) != nil {
			t.Fatal(response.Code, response.Body.String())
		}
		if page.Offset != offset || page.NextOffset <= offset || page.IndependentlyVerified || page.RunID != input.RunID {
			t.Fatal("invalid page", page)
		}
		assembled.WriteString(page.Content)
		if page.Complete {
			break
		}
		offset = page.NextOffset
	}
	if assembled.String() != answer || calls != 1 {
		t.Fatal("truncated or dispatched while observing", calls)
	}
	for _, query := range []string{"?offset=-1", "?limit=0", "?limit=65537", "?offset=" + strconv.Itoa(len(answer)+1), "?offset=10"} {
		if response := read(s, query); response.Code != 400 {
			t.Fatal("invalid page accepted", query, response.Code)
		}
	}
	foreign := *s
	cfg := *s.config
	cfg.RequireAuth = true
	foreign.config = &cfg
	if response := read(&foreign, ""); response.Code != 404 {
		t.Fatal("foreign actor read result", response.Code)
	}
	// Live source access still applies to a sealed result.
	task, _ := s.agentManager.GetTask(input.RunID)
	task.Config.KnowledgeDependencies = []string{"revoked-source"}
	// Source dependencies cannot be injected through HTTP. Use a second fixture
	// run whose admitted configuration genuinely depends on the revoked source.
	second := acceptContextInput(t, s, scope, conversation.ID, "private-result", "Inspect private source")
	completeContextInput(t, s, scope, second, task.Config)
	second, _ = s.agentManager.ConversationInput(scope, conversation.ID, second.ID)
	path = "/api/v2/results/" + url.PathEscape(second.ResultReference)
	if response := read(s, ""); response.Code != 403 || strings.Contains(response.Body.String(), "Final citation") {
		t.Fatal("revoked result leaked", response.Code, response.Body.String())
	}
}
