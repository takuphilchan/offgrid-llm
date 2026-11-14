package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func importedInboxFixture(t *testing.T) (*conversationFixture, agents.ControlScope, string) {
	t.Helper()
	f := newConversationFixture(t)
	f.s.bindCanonicalConversationGuard()
	r := httptest.NewRequest("POST", "/api/v2/conversations/import", nil)
	manifest, err := f.s.migrateConversation(r, "chat")
	if err != nil {
		t.Fatal(err)
	}
	return f, f.s.controlScope(r), manifest.ConversationID
}

func readProjectedInbox(t *testing.T, s *Server) *sessions.Session {
	t.Helper()
	w := httptest.NewRecorder()
	s.sessionHandlers.HandleSessionGet(w, httptest.NewRequest("GET", "/v1/sessions/chat", nil), "chat")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var session sessions.Session
	if err := json.Unmarshal(w.Body.Bytes(), &session); err != nil {
		t.Fatal(err)
	}
	return &session
}

func TestCanonicalProjectionIncludesInboxWithoutDuplicatingWorkOrCorrections(t *testing.T) {
	f, scope, id := importedInboxFixture(t)
	s := f.s
	path := filepath.Join(f.directory, "chat.json")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	s.agentRunner = agents.NewRunner(s.agentManager, s.toolRegistry, func(ctx context.Context, task *agents.Task, messages []api.ChatMessage, tools []api.Tool) (*api.ChatCompletionResponse, error) {
		calls.Add(1)
		if task.ConversationTurn != nil && task.Prompt != "Compare the approaches" {
			message := api.ChatMessage{Role: "assistant", Content: "Side question answered"}
			finish := "stop"
			if task.Prompt == "Keep the examples brief" {
				message.Content = "Correction pending"
				if len(task.Steps) == 0 {
					finish, message.Content = "tool_calls", ""
					message.ToolCalls = []api.ToolCall{{Type: "function", Function: api.FunctionCall{Name: "conversation_decide", Arguments: `{"kind":"correction","instruction_version":1}`}}}
				}
			}
			return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: finish, Message: message}}}, nil
		}
		return foregroundWorkModel(ctx, task, messages, tools)
	})
	t.Cleanup(func() { _ = s.agentRunner.Shutdown(context.Background()) })
	config := agents.DefaultAgentConfig()
	config.TaskFirst = true
	first := acceptContextInput(t, s, scope, id, "work", "Compare the approaches")
	completeContextInput(t, s, scope, first, config)
	entry, err := s.prepareConversationInputWork(context.Background(), scope, id, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	work, err := s.agentManager.WorkItem(scope, entry.CreatedWorkID)
	if err != nil {
		t.Fatal(err)
	}
	for _, content := range []string{"What does that term mean?", "Keep the examples brief"} {
		input := acceptContextInput(t, s, scope, id, content, content)
		completeContextInput(t, s, scope, input, config)
		if _, err = s.agentRunner.ApplyConversationDecision(scope, id, input.ID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = s.agentRunner.Continue(context.Background(), work.RunID, scope.Actor, "start", "", false); err != nil {
		t.Fatal(err)
	}
	before := calls.Load()
	currentInput := acceptContextInput(t, s, scope, id, "current-summary", "Summarize the finished comparison")
	acceptContextInput(t, s, scope, id, "future-input", "Future input must not leak")
	conversation, _ := s.agentManager.Conversation(scope, id)
	var history agents.AgentConfig
	if err = s.assembleCanonicalHistory(httptest.NewRequest("GET", "/", nil), conversation, currentInput.ID, &history); err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, message := range history.ConversationContext {
		counts[message.StringContent()]++
	}
	for _, content := range []string{"Compare the approaches", "What does that term mean?", "Keep the examples brief", "Side question answered", "Comparison completed"} {
		if counts[content] != 1 {
			t.Fatalf("model context contains %q %d times", content, counts[content])
		}
	}
	if counts[currentInput.Input.Content] != 0 || counts["Future input must not leak"] != 0 || history.ConversationContext[len(history.ConversationContext)-1].StringContent() != "Comparison completed" {
		t.Fatal("model history leaked later input or reordered outcome", history.ConversationContext)
	}
	for i := 0; i < 2; i++ {
		session := readProjectedInbox(t, s)
		counts := map[string]int{}
		for j, message := range session.Messages {
			counts[message.Content]++
			if j > 0 && message.Timestamp.Before(session.Messages[j-1].Timestamp) {
				t.Fatal("result moved before intervening conversation", session.Messages)
			}
		}
		for _, content := range []string{"Compare the approaches", "What does that term mean?", "Keep the examples brief", "Side question answered", "Comparison completed", "Correction applied to work instruction version 2."} {
			if counts[content] != 1 {
				t.Fatalf("%q appears %d times", content, counts[content])
			}
		}
		if len(session.Jobs) != 4 {
			t.Fatal("incomplete or reordered transcript", session)
		}
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(original, after) || calls.Load() != before {
		t.Fatal("projection mutated import or dispatched inference", err, calls.Load())
	}
}

func TestCanonicalProjectionWithholdsRevokedInboxReplyAndKeepsWithdrawnInput(t *testing.T) {
	f, scope, id := importedInboxFixture(t)
	s := f.s
	s.agentRunner = agents.NewRunner(s.agentManager, s.toolRegistry, func(context.Context, *agents.Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "stop", Message: api.ChatMessage{Role: "assistant", Content: "Private derived answer"}}}}, nil
	})
	t.Cleanup(func() { _ = s.agentRunner.Shutdown(context.Background()) })
	input := acceptContextInput(t, s, scope, id, "private", "My original question")
	config := agents.DefaultAgentConfig()
	config.KnowledgeDependencies = []string{"revoked-source"}
	completeContextInput(t, s, scope, input, config)
	queued := acceptContextInput(t, s, scope, id, "withdraw", "Withdraw this request")
	c, _ := s.agentManager.Conversation(scope, id)
	if _, err := s.agentRunner.ControlConversationInput(scope, id, api.ConversationInputControlCommand{RequestID: "stop", TargetID: queued.ID, ExpectedRevision: c.Revision, Kind: "stop"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.agentRunner.SettleStoppedConversationInput(scope, id, queued.ID); err != nil {
		t.Fatal(err)
	}
	session := readProjectedInbox(t, s)
	data, _ := json.Marshal(session)
	for _, text := range []string{"My original question", "Withdraw this request", "source access changed", "input was withdrawn"} {
		if !strings.Contains(string(data), text) {
			t.Fatal("history or status missing", text, string(data))
		}
	}
	if strings.Contains(string(data), "Private derived") || strings.Contains(string(data), "revoked-source") || session.Turn.Status != "stopped" {
		t.Fatal("revoked result leaked or withdrawal lost", string(data))
	}
	stored, _ := s.agentManager.ConversationInput(scope, id, queued.ID)
	if stored.RunID != "" {
		t.Fatal("read admitted withdrawn input", stored)
	}
}

func TestCanonicalProjectionQueuedInputDoesNotUseLegacyTurnRecovery(t *testing.T) {
	f, scope, id := importedInboxFixture(t)
	input := acceptContextInput(t, f.s, scope, id, "queued", "Wait for the selected model")
	for _, action := range []string{"turn", "turn/recover", "turn/cancel"} {
		method, want := "POST", 409
		if action == "turn" {
			method, want = "GET", 200
		}
		w := httptest.NewRecorder()
		r := httptest.NewRequest(method, "/v1/sessions/chat/"+action, strings.NewReader(`{"id":"`+input.ID+`"}`))
		f.s.sessionHandlers.handleTurn(w, r, "chat", action)
		if w.Code != want {
			t.Fatal(action, w.Code, w.Body.String())
		}
		if method == "POST" && !strings.Contains(w.Body.String(), "canonical_conversation_required") {
			t.Fatal("legacy mutation lacks compatibility explanation", w.Body.String())
		}
	}
	stored, err := f.s.agentManager.ConversationInput(scope, id, input.ID)
	if err != nil || stored.State != "accepted" || stored.RunID != "" || f.calls.Load() != 0 {
		t.Fatal("legacy observation or recovery changed canonical input", stored, err)
	}
}
