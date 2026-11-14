package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestConversationLinkUsesDurableMembershipAndCurrentSourceAccess(t *testing.T) {
	s := profileFixture(t)
	s.config.EnableConversationPreview = true
	s.agentRunner = agents.NewRunner(s.agentManager, s.toolRegistry, func(context.Context, *agents.Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "stop", Message: api.ChatMessage{Role: "assistant", Content: "Retained research evidence"}}}}, nil
	})
	defer s.agentRunner.Shutdown(context.Background())
	scope := agents.ControlScope{Workspace: s.workspaceID, Actor: "local-admin"}
	first, err := s.agentManager.ApplyConversationCommand(scope, agents.ConversationCommand{RequestID: "first", Kind: "create_conversation"})
	if err != nil {
		t.Fatal(err)
	}
	createWork := agents.ConversationCommand{RequestID: "research", TargetID: first.TargetID, ExpectedRevision: 1, Kind: "create_work", Payload: agents.ConversationCommandPayload{Input: &agents.ConversationInput{Content: "Keep the source interpretation"}}}
	config := agents.DefaultAgentConfig()
	config.TaskFirst = true
	config.ContextWindow = 16384
	config.ScopedCapabilities = true
	accepted, err := s.agentRunner.AdmitConversationWork(scope, createWork, "model", config)
	if err != nil {
		t.Fatal(err)
	}
	done, err := s.agentRunner.Continue(context.Background(), accepted.Work.RunID, scope.Actor, "start", "", false)
	if err != nil {
		t.Fatal(err)
	}
	no := false
	second, err := s.agentManager.ApplyConversationCommand(scope, agents.ConversationCommand{RequestID: "second", Kind: "create_conversation", Payload: agents.ConversationCommandPayload{Preferences: &agents.ConversationPreferences{Model: "model", Restrictions: agents.ResourceRestrictions{Computer: &no}}}})
	if err != nil {
		t.Fatal(err)
	}
	forged := &sessions.Session{Name: second.TargetID, Jobs: []sessions.ConversationJobLink{{JobID: done.ID, TurnID: done.Origin.TurnID}}}
	if err = s.assembleConversationContext(forged, scope.Actor, &agents.AgentConfig{}); err == nil {
		t.Fatal("client supplied link granted context")
	}
	command := agents.ConversationCommand{RequestID: "link", TargetID: second.TargetID, ExpectedRevision: 1, Kind: "link_work", Payload: agents.ConversationCommandPayload{WorkID: accepted.Work.ID}}
	path := "/api/v2/conversations/" + second.TargetID + "/commands"
	w := controlHTTP(s, "POST", path, command)
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	var receipt api.ConversationCommandReceipt
	if err = json.Unmarshal(w.Body.Bytes(), &receipt); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("POST", path, nil)
	next := agents.ConversationCommand{Payload: agents.ConversationCommandPayload{Input: &agents.ConversationInput{Content: "Explain the earlier result"}}}
	resolved, _, ok := s.resolveConversationRun(httptest.NewRecorder(), request, receipt.Conversation, next)
	if !ok {
		t.Fatal("linked context could not be resolved")
	}
	var history strings.Builder
	for _, message := range resolved.ConversationContext {
		history.WriteString(message.StringContent())
	}
	if !strings.Contains(history.String(), "Retained research evidence") || !resolved.RestrictComputer || resolved.Execution != nil || resolved.ComputerSession != "" {
		t.Fatal("link lost context or copied authority", history.String(), resolved)
	}
	// A second owned run whose source is unavailable cannot be linked. This
	// remains an access check even when the actor owns both conversations.
	config.KnowledgeDependencies = []string{"revoked-source"}
	createWork.RequestID, createWork.ExpectedRevision = "restricted-research", 2
	private, err := s.agentRunner.AdmitConversationWork(scope, createWork, "model", config)
	if err != nil {
		t.Fatal(err)
	}
	command.RequestID, command.ExpectedRevision, command.Payload.WorkID = "revoked-link", 2, private.Work.ID
	w = controlHTTP(s, "POST", path, command)
	if w.Code != 403 {
		t.Fatal("revoked source linked", w.Code, w.Body.String())
	}
	saved, err := s.agentManager.Conversation(scope, second.TargetID)
	if err != nil || saved.Revision != 2 || saved.FocusedWorkID != accepted.Work.ID {
		t.Fatal("denied link changed focus", saved, err)
	}
}
