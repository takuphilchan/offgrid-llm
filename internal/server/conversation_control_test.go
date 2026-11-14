package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func controlHTTP(s *Server, method, path string, payload any) *httptest.ResponseRecorder {
	data, _ := json.Marshal(payload)
	w := httptest.NewRecorder()
	s.handleConversations(w, httptest.NewRequest(method, path, strings.NewReader(string(data))))
	return w
}

func TestCanonicalConversationRequiresNoProfileAndGetNeverDispatches(t *testing.T) {
	s := profileFixture(t)
	s.config.EnableConversationPreview = true
	s.agentManager = agents.NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	var calls atomic.Int32
	s.agentRunner = agents.NewRunner(s.agentManager, &browserRunTools{RunTools: s.toolRegistry, server: s}, func(_ context.Context, task *agents.Task, messages []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		calls.Add(1)
		if task.Config.Profile != nil {
			t.Error("mandatory profile was fabricated")
		}
		if task.Prompt == "Continue" {
			found := false
			for _, m := range messages {
				if m.StringContent() == "Keep the first constraint" {
					found = true
				}
			}
			if !found {
				t.Error("previous constraint missing")
			}
		}
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "stop", Message: api.ChatMessage{Role: "assistant", Content: "Remembered"}}}}, nil
	})
	stopDelegation := false
	create := agents.ConversationCommand{RequestID: "new-default", Kind: "create_conversation", Payload: agents.ConversationCommandPayload{Preferences: &agents.ConversationPreferences{Model: "model", Delegation: &stopDelegation}}}
	w := controlHTTP(s, "POST", "/api/v2/conversations", create)
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	var created agents.ConversationCommandReceipt
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	command := agents.ConversationCommand{RequestID: "first-input", TargetID: created.TargetID, ExpectedRevision: 1, Kind: "create_work", Payload: agents.ConversationCommandPayload{Input: &agents.ConversationInput{Content: "Keep the first constraint"}}}
	path := "/api/v2/conversations/" + created.TargetID
	w = controlHTTP(s, "POST", path+"/commands", command)
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	var accepted agents.ConversationCommandReceipt
	if err := json.Unmarshal(w.Body.Bytes(), &accepted); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"", "/configuration", "/events?limit=1"} {
		w = controlHTTP(s, "GET", path+suffix, nil)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	w = controlHTTP(s, "POST", path+"/commands", command)
	if w.Code != 200 || calls.Load() != 0 {
		t.Fatal("observation/retry dispatched", w.Code, calls.Load())
	}
	if _, err := s.agentRunner.Continue(context.Background(), accepted.Work.RunID, "local-admin", "start", "", false); err != nil {
		t.Fatal(err)
	}
	command.RequestID = "follow-up"
	command.ExpectedRevision = 2
	command.Payload.Input.Content = "Continue"
	w = controlHTTP(s, "POST", path+"/commands", command)
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &accepted); err != nil {
		t.Fatal(err)
	}
	if _, err := s.agentRunner.Continue(context.Background(), accepted.Work.RunID, "local-admin", "start", "", false); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatal("extra classifier or missing work", calls.Load())
	}
	s.config.EnableConversationPreview = false
	w = controlHTTP(s, "POST", path+"/commands", command)
	if w.Code != 200 {
		t.Fatal("flag stranded accepted identity", w.Code)
	}
	command.RequestID = "new-disabled"
	command.ExpectedRevision = 3
	w = controlHTTP(s, "POST", path+"/commands", command)
	if w.Code != 503 {
		t.Fatal("disabled admission accepted", w.Code)
	}
	w = controlHTTP(s, "GET", path, nil)
	if w.Code != 200 {
		t.Fatal("flag hid conversation", w.Code)
	}
}

func TestCanonicalCommandRejectsUnknownAuthorityAndStaleSettings(t *testing.T) {
	s := profileFixture(t)
	s.config.EnableConversationPreview = true
	w := httptest.NewRecorder()
	s.handleConversations(w, httptest.NewRequest("POST", "/api/v2/conversations", strings.NewReader(`{"request_id":"bad-grant","target_id":"","expected_revision":0,"kind":"create_conversation","payload":{"preferences":{"grant":"forged"}}}`)))
	if w.Code != 400 {
		t.Fatal("unknown authority accepted", w.Code)
	}
	no := false
	w = controlHTTP(s, "POST", "/api/v2/conversations", agents.ConversationCommand{RequestID: "persist-settings", Kind: "create_conversation", Payload: agents.ConversationCommandPayload{Preferences: &agents.ConversationPreferences{Model: "model", Restrictions: agents.ResourceRestrictions{Computer: &no}}}})
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	var accepted agents.ConversationCommandReceipt
	_ = json.Unmarshal(w.Body.Bytes(), &accepted)
	command := agents.ConversationCommand{RequestID: "change-model", TargetID: accepted.TargetID, ExpectedRevision: 1, Kind: "configure_conversation", Payload: agents.ConversationCommandPayload{Preferences: &agents.ConversationPreferences{Model: "other-model"}}}
	w = controlHTTP(s, "POST", "/api/v2/conversations/"+accepted.TargetID+"/commands", command)
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	_ = json.Unmarshal(w.Body.Bytes(), &accepted)
	if accepted.Conversation.Preferences.Restrictions.Computer == nil || *accepted.Conversation.Preferences.Restrictions.Computer {
		t.Fatal("model change cleared restriction")
	}
	command.RequestID = "stale-settings"
	w = controlHTTP(s, "POST", "/api/v2/conversations/"+accepted.TargetID+"/commands", command)
	if w.Code != 409 {
		t.Fatal("stale settings overwrote restriction", w.Code)
	}
}
