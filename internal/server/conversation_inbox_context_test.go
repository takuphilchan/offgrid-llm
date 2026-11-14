package server

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func contextInboxFixture(t *testing.T) (*Server, agents.ControlScope, *agents.Conversation) {
	t.Helper()
	s := profileFixture(t)
	scope := agents.ControlScope{Workspace: s.workspaceID, Actor: "local-admin"}
	receipt, err := s.agentManager.ApplyConversationCommand(scope, agents.ConversationCommand{RequestID: "conversation", Kind: "create_conversation"})
	if err != nil {
		t.Fatal(err)
	}
	return s, scope, receipt.Conversation
}

func acceptContextInput(t *testing.T, s *Server, scope agents.ControlScope, conversation, id, content string) *api.ConversationInboxEntry {
	t.Helper()
	c, err := s.agentManager.Conversation(scope, conversation)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := s.agentRunner.AcceptConversationInput(scope, api.ConversationInputCommand{RequestID: id, TargetID: c.ID, ExpectedRevision: c.Revision, Kind: "submit_input", Payload: api.ConversationInboxInput{Content: content}})
	if err != nil {
		t.Fatal(err)
	}
	return receipt.Input
}

func completeContextInput(t *testing.T, s *Server, scope agents.ControlScope, input *api.ConversationInboxEntry, config agents.AgentConfig) *agents.Task {
	t.Helper()
	c, _ := s.agentManager.Conversation(scope, input.ConversationID)
	task, err := s.agentRunner.AdmitConversationTurn(scope, c.ID, input.ID, c.Revision, "model", config)
	if err != nil {
		t.Fatal(err)
	}
	task, err = s.agentRunner.Continue(context.Background(), task.ID, scope.Actor, "start", "", false)
	if err != nil || task.Status != agents.TaskCompleted {
		t.Fatal(task, err)
	}
	return task
}

func TestConversationInboxContextRetainsLongReplyAndStopsBeforeCurrentInput(t *testing.T) {
	s, scope, c := contextInboxFixture(t)
	longAnswer := strings.Repeat("Retained source-backed explanation. ", 1200)
	s.agentRunner = agents.NewRunner(s.agentManager, s.toolRegistry, func(context.Context, *agents.Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "stop", Message: api.ChatMessage{Role: "assistant", Content: longAnswer}}}}, nil
	})
	first := acceptContextInput(t, s, scope, c.ID, "first", "Always preserve my source files")
	completeContextInput(t, s, scope, first, agents.DefaultAgentConfig())
	second := acceptContextInput(t, s, scope, c.ID, "second", "Current request")
	acceptContextInput(t, s, scope, c.ID, "third", "Later correction must not enter the earlier turn")
	request := httptest.NewRequest("POST", "/api/v2/conversations/"+c.ID, nil)
	config := agents.AgentConfig{}
	if err := s.assembleConversationInboxContext(request, c.ID, second.ID, &config); err != nil {
		t.Fatal(err)
	}
	if len(config.ConversationContext) != 2 || config.ConversationContext[0].StringContent() != first.Input.Content || config.ConversationContext[1].StringContent() != longAnswer {
		t.Fatal("history was truncated, duplicated or included future input", len(config.ConversationContext))
	}
	config = agents.AgentConfig{}
	if err := s.assembleConversationInboxContext(request, c.ID, "", &config); err != nil || len(config.ConversationContext) != 6 {
		t.Fatal("queued input was lost or presented as completed", len(config.ConversationContext), err)
	}
	for _, index := range []int{3, 5} {
		if !strings.Contains(config.ConversationContext[index].StringContent(), "queued") {
			t.Fatal("unexecuted input missing status")
		}
	}
}

func TestConversationInboxContextRevocationRetainsUserInput(t *testing.T) {
	s, scope, c := contextInboxFixture(t)
	s.agentRunner = agents.NewRunner(s.agentManager, s.toolRegistry, func(context.Context, *agents.Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "stop", Message: api.ChatMessage{Role: "assistant", Content: "Private derived result"}}}}, nil
	})
	input := acceptContextInput(t, s, scope, c.ID, "private", "Keep my original constraint")
	pinned := agents.DefaultAgentConfig()
	pinned.KnowledgeDependencies = []string{"revoked-source"}
	task := completeContextInput(t, s, scope, input, pinned)
	config := agents.AgentConfig{}
	if err := s.assembleConversationInboxContext(httptest.NewRequest("POST", "/", nil), c.ID, "", &config); err != nil {
		t.Fatal(err)
	}
	if len(config.ConversationContext) != 2 || config.ConversationContext[0].StringContent() != input.Input.Content || !strings.Contains(config.ConversationContext[1].StringContent(), "source access changed") || len(config.KnowledgeDependencies) != 0 {
		t.Fatal("revocation erased input or retained derived content", config.ConversationContext)
	}
	forged := *input
	forged.RunID, forged.ID = task.ID, "input-other"
	if err := s.appendConversationInputContext(scope, c.ID, forged, &agents.AgentConfig{}); !errors.Is(err, agents.ErrControlNotFound) {
		t.Fatal("forged foreground link accepted", err)
	}
	if _, err := s.agentManager.ConversationInput(agents.ControlScope{Workspace: scope.Workspace, Actor: "other"}, c.ID, input.ID); !errors.Is(err, agents.ErrControlNotFound) {
		t.Fatal("foreign input exposed", err)
	}
}
