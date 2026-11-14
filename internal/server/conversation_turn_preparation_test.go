package server

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/users"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestServiceConversationPreparationPreservesContextWithoutDispatch(t *testing.T) {
	s, scope, c := contextInboxFixture(t)
	s.config.DefaultModel = "model"
	var calls atomic.Int32
	s.agentRunner = agents.NewRunner(s.agentManager, s.toolRegistry, func(_ context.Context, task *agents.Task, messages []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		calls.Add(1)
		if task.Prompt == "Second question" {
			var assembled strings.Builder
			for _, message := range messages {
				assembled.WriteString(message.StringContent())
			}
			for _, want := range []string{"Keep original files", "First reply", "Second question"} {
				if !strings.Contains(assembled.String(), want) {
					t.Error("missing context", want)
				}
			}
			if strings.Contains(assembled.String(), "Later queued instruction") {
				t.Error("future input reached earlier turn")
			}
		}
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "stop", Message: api.ChatMessage{Role: "assistant", Content: "First reply"}}}}, nil
	})
	first := acceptContextInput(t, s, scope, c.ID, "first", "Keep original files")
	task, err := s.prepareConversationTurn(context.Background(), scope, c.ID, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 0 || task.Config.Profile != nil || task.Config.Execution != nil || task.Config.ComputerSession != "" || task.Config.ComputerAccessOnDemand || !task.Config.RestrictComputer || task.WorkID != "" {
		t.Fatal("preparation dispatched or inherited authority", task)
	}
	if _, err = s.agentRunner.Continue(context.Background(), task.ID, scope.Actor, "start", "", false); err != nil {
		t.Fatal(err)
	}
	second := acceptContextInput(t, s, scope, c.ID, "second", "Second question")
	acceptContextInput(t, s, scope, c.ID, "third", "Later queued instruction")
	next, err := s.prepareConversationTurn(context.Background(), scope, c.ID, second.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.agentRunner.Continue(context.Background(), next.ID, scope.Actor, "start", "", false); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatal("extra classification/generation", calls.Load())
	}
	// Retry resolves accepted identity before consulting mutable availability.
	s.registry = nil
	retry, err := s.prepareConversationTurn(context.Background(), scope, c.ID, first.ID)
	if err != nil || retry.ID != task.ID || retry.Status != agents.TaskCompleted || calls.Load() != 2 {
		t.Fatal(retry, err)
	}
}

func TestServiceConversationPreparationRechecksActorAndRetainsUndeliveredInput(t *testing.T) {
	s := profileFixture(t)
	s.config.RequireAuth, s.config.DefaultModel = true, "model"
	s.agentRunner = agents.NewRunner(s.agentManager, s.toolRegistry, func(context.Context, *agents.Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		t.Error("revoked actor reached inference")
		return nil, errors.New("unexpected inference")
	})
	s.agentRunner.SourceAccess = s.taskSourceAllowed
	s.userStore = users.NewUserStore(t.TempDir())
	actor, _, err := s.userStore.CreateUser("owner", "fixture-password", users.RoleGuest)
	if err != nil {
		t.Fatal(err)
	}
	scope := agents.ControlScope{Workspace: s.workspaceID, Actor: actor.ID}
	created, err := s.agentManager.ApplyConversationCommand(scope, agents.ConversationCommand{RequestID: "new", Kind: "create_conversation"})
	if err != nil {
		t.Fatal(err)
	}
	input := acceptContextInput(t, s, scope, created.TargetID, "input", "Explain this")
	if err = s.userStore.UpdateUser(actor.ID, map[string]any{"disabled": true}); err != nil {
		t.Fatal(err)
	}
	_, err = s.prepareConversationTurn(context.Background(), scope, created.TargetID, input.ID)
	var issue *serviceError
	if !errors.As(err, &issue) {
		t.Fatal("disabled actor prepared work", err)
	}
	retained, err := s.agentManager.ConversationInput(scope, created.TargetID, input.ID)
	if err != nil || retained.RunID != "" || retained.State != "accepted" {
		t.Fatal(retained, err)
	}
	if err = s.userStore.UpdateUser(actor.ID, map[string]any{"disabled": false}); err != nil {
		t.Fatal(err)
	}
	task, err := s.prepareConversationTurn(context.Background(), scope, created.TargetID, input.ID)
	if err != nil || task.Actor != actor.ID || len(task.Config.AllowedCapabilities) != 0 {
		t.Fatal(task, err)
	}
	if _, err = s.prepareConversationTurn(context.Background(), agents.ControlScope{Workspace: "other", Actor: actor.ID}, created.TargetID, input.ID); !errors.Is(err, agents.ErrControlNotFound) {
		t.Fatal("cross-workspace preparation", err)
	}
	if err = s.userStore.UpdateUser(actor.ID, map[string]any{"disabled": true}); err != nil {
		t.Fatal(err)
	}
	blocked, err := s.agentRunner.Continue(context.Background(), task.ID, scope.Actor, "start", "", false)
	if err != nil || blocked.Status != agents.TaskFailed || blocked.Checkpoint.BudgetedModelCalls != 0 || blocked.Error != agents.ErrSourceAccess.Error() {
		t.Fatal("revocation after preparation allowed execution", blocked, err)
	}
}
