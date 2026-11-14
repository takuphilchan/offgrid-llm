package server

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/users"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func foregroundWorkModel(_ context.Context, task *agents.Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
	message := api.ChatMessage{Role: "assistant", Content: "Work recorded; admission is pending."}
	finish := "stop"
	if task.ConversationTurn != nil && len(task.Steps) == 0 {
		finish, message.Content = "tool_calls", ""
		message.ToolCalls = []api.ToolCall{{Type: "function", Function: api.FunctionCall{Name: "conversation_decide", Arguments: `{"kind":"new_work","instruction_version":0,"expected_result":"A comparison with citations and limitations"}`}}}
	}
	if task.WorkID != "" {
		message.Content = "Comparison completed"
	}
	return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: finish, Message: message}}}, nil
}

func TestServiceConversationWorkPreparationUsesCurrentConfigAndOneRun(t *testing.T) {
	s, scope, c := contextInboxFixture(t)
	s.config.DefaultModel = "model"
	calls := 0
	s.agentRunner = agents.NewRunner(s.agentManager, s.toolRegistry, func(ctx context.Context, task *agents.Task, messages []api.ChatMessage, tools []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		if task.WorkID != "" {
			if task.Config.Profile != nil || task.Config.Execution != nil || task.Config.ComputerSession != "" || task.Prompt != "Compare the two approaches" {
				t.Error("new work lost input or inherited authority", task)
			}
			found := false
			for _, message := range messages {
				found = found || strings.Contains(message.StringContent(), "A comparison with citations and limitations")
			}
			if !found {
				t.Error("proposed acceptance missing from work context")
			}
		}
		return foregroundWorkModel(ctx, task, messages, tools)
	})
	s.agentRunner.SourceAccess = s.taskSourceAllowed
	input := acceptContextInput(t, s, scope, c.ID, "new-work", "Compare the two approaches")
	foreground, err := s.prepareConversationTurn(context.Background(), scope, c.ID, input.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.agentRunner.Continue(context.Background(), foreground.ID, scope.Actor, "start", "", false); err != nil {
		t.Fatal(err)
	}
	controller := &conversationDeliveryController{server: s}
	now := time.Now()
	for i := 0; i < 2; i++ {
		if err = controller.coordinate(context.Background(), now.Add(time.Duration(i)*time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	entry, _ := s.agentManager.ConversationInput(scope, c.ID, input.ID)
	if entry.CreatedWorkID == "" || entry.Incorporation != "new_work" || entry.AppliedAt == nil || calls != 2 {
		t.Fatal("work not admitted or admission performed inference", entry, calls)
	}
	work, _ := s.agentManager.WorkItem(scope, entry.CreatedWorkID)
	if _, err = s.agentRunner.Continue(context.Background(), work.RunID, scope.Actor, "start", "", false); err != nil || calls != 3 {
		t.Fatal("admitted run did not use existing runner", err, calls)
	}
	s.registry = nil
	retry, err := s.prepareConversationInputWork(context.Background(), scope, c.ID, input.ID)
	if err != nil || retry.CreatedWorkID != work.ID || calls != 3 {
		t.Fatal("completed retry prepared or dispatched another run", retry, err)
	}
}

func TestServiceConversationWorkPreparationDefersMissingModel(t *testing.T) {
	s, scope, c := contextInboxFixture(t)
	s.agentRunner = agents.NewRunner(s.agentManager, s.toolRegistry, foregroundWorkModel)
	input := acceptContextInput(t, s, scope, c.ID, "new-work", "Compare approaches")
	config := agents.DefaultAgentConfig()
	config.TaskFirst = true
	completeContextInput(t, s, scope, input, config)
	s.config.DefaultModel = ""
	controller := &conversationDeliveryController{server: s}
	now := time.Now()
	if err := controller.coordinate(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	entry, _ := s.agentManager.ConversationInput(scope, c.ID, input.ID)
	if entry.CreatedWorkID != "" || entry.DecisionIssue == nil || entry.DecisionIssue.Code != "model_unavailable" {
		t.Fatal("missing model lost decision or admitted work", entry)
	}
	s.config.DefaultModel = "model"
	if err := controller.coordinate(context.Background(), now.Add(31*time.Second)); err != nil {
		t.Fatal(err)
	}
	entry, _ = s.agentManager.ConversationInput(scope, c.ID, input.ID)
	if entry.CreatedWorkID == "" || entry.DecisionIssue != nil {
		t.Fatal("saved decision did not recover", entry)
	}
}

func TestServiceConversationWorkChecksRevokedActorBeforeDispatch(t *testing.T) {
	s := profileFixture(t)
	s.config.RequireAuth, s.config.DefaultModel = true, "model"
	s.userStore = users.NewUserStore(t.TempDir())
	actor, _, err := s.userStore.CreateUser("owner", "fixture-password", users.RoleGuest)
	if err != nil {
		t.Fatal(err)
	}
	scope := agents.ControlScope{Workspace: s.workspaceID, Actor: actor.ID}
	created, err := s.agentManager.ApplyConversationCommand(scope, agents.ConversationCommand{RequestID: "conversation", Kind: "create_conversation"})
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	s.agentRunner = agents.NewRunner(s.agentManager, s.toolRegistry, func(ctx context.Context, task *agents.Task, messages []api.ChatMessage, tools []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		return foregroundWorkModel(ctx, task, messages, tools)
	})
	s.agentRunner.SourceAccess = s.taskSourceAllowed
	input := acceptContextInput(t, s, scope, created.TargetID, "new-work", "Compare approaches")
	config := agents.DefaultAgentConfig()
	config.TaskFirst = true
	completeContextInput(t, s, scope, input, config)
	entry, err := s.prepareConversationInputWork(context.Background(), scope, created.TargetID, input.ID)
	if err != nil {
		t.Fatal(err)
	}
	work, _ := s.agentManager.WorkItem(scope, entry.CreatedWorkID)
	if err = s.userStore.UpdateUser(actor.ID, map[string]any{"disabled": true}); err != nil {
		t.Fatal(err)
	}
	task, err := s.agentRunner.Continue(context.Background(), work.RunID, scope.Actor, "start", "", false)
	if err != nil || task.Status != agents.TaskFailed || calls != 2 {
		t.Fatal("disabled actor reached work inference", task, err, calls)
	}
}
