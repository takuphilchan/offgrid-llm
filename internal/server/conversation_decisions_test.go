package server

import (
	"context"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestServiceConversationDecisionReleasesOnlyItsHoldWithoutInference(t *testing.T) {
	for _, control := range []string{"", "pause", "stop"} {
		t.Run(control, func(t *testing.T) {
			s, scope, c := contextInboxFixture(t)
			calls := 0
			s.agentRunner = agents.NewRunner(s.agentManager, s.toolRegistry, func(context.Context, *agents.Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
				calls++
				return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "stop", Message: api.ChatMessage{Role: "assistant", Content: "Side question answered"}}}}, nil
			})
			config := agents.DefaultAgentConfig()
			config.TaskFirst = true
			accepted, err := s.agentRunner.AdmitConversationWork(scope, agents.ConversationCommand{RequestID: "work", TargetID: c.ID, ExpectedRevision: c.Revision, Kind: "create_work", Payload: agents.ConversationCommandPayload{Input: &agents.ConversationInput{Content: "Prepare an outline"}}}, "model", config)
			if err != nil {
				t.Fatal(err)
			}
			input := acceptContextInput(t, s, scope, c.ID, "question", "What does that term mean?")
			completeContextInput(t, s, scope, input, config)
			if control != "" {
				work, _ := s.agentManager.WorkItem(scope, accepted.Work.ID)
				if _, err = s.agentRunner.ControlWork(scope, api.WorkCommand{RequestID: control, TargetID: work.ID, ExpectedRevision: work.Revision, Kind: control}); err != nil {
					t.Fatal(err)
				}
			}
			controller := &conversationDeliveryController{server: s}
			for i := 0; i < 2; i++ {
				if err = controller.coordinate(context.Background(), time.Now().Add(time.Duration(i)*time.Second)); err != nil {
					t.Fatal(err)
				}
			}
			entry, err := s.agentManager.ConversationInput(scope, c.ID, input.ID)
			if err != nil || entry.AppliedAt == nil || entry.Incorporation != "answered" || calls != 1 {
				t.Fatal("service did not settle answer without inference", entry, err, calls)
			}
			root, _ := s.agentManager.GetTask(accepted.Work.RunID)
			if root.ConversationHold != nil || root.InstructionVersion != 1 || root.Checkpoint.BudgetedModelCalls != 0 {
				t.Fatal("answer changed work allowance or instructions", root)
			}
			if control != "" && (root.SchedulerReady || root.Status == agents.TaskPending) {
				t.Fatal("incorporation overrode control", root)
			}
		})
	}
}

func TestServiceConversationDecisionRetainsRevokedInputAndRetriesWithoutModel(t *testing.T) {
	s, scope, c := contextInboxFixture(t)
	calls := 0
	s.agentRunner = agents.NewRunner(s.agentManager, s.toolRegistry, func(context.Context, *agents.Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "stop", Message: api.ChatMessage{Role: "assistant", Content: "Answer"}}}}, nil
	})
	input := acceptContextInput(t, s, scope, c.ID, "question", "Explain this")
	completeContextInput(t, s, scope, input, agents.DefaultAgentConfig())
	s.agentRunner.SourceAccess = func(*agents.Task) bool { return false }
	controller := &conversationDeliveryController{server: s}
	now := time.Now()
	if err := controller.coordinate(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	entry, _ := s.agentManager.ConversationInput(scope, c.ID, input.ID)
	if entry.AppliedAt != nil || entry.DecisionIssue == nil || entry.DecisionIssue.Code != "conversation_decision_access_changed" {
		t.Fatal("revoked decision was lost/applied or limitation hidden", entry)
	}
	s.agentRunner.SourceAccess = func(*agents.Task) bool { return true }
	if err := controller.coordinate(context.Background(), now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	entry, _ = s.agentManager.ConversationInput(scope, c.ID, input.ID)
	if entry.AppliedAt != nil {
		t.Fatal("retry backoff bypassed")
	}
	if err := controller.coordinate(context.Background(), now.Add(31*time.Second)); err != nil {
		t.Fatal(err)
	}
	entry, _ = s.agentManager.ConversationInput(scope, c.ID, input.ID)
	if entry.AppliedAt == nil || entry.DecisionIssue != nil || calls != 1 {
		t.Fatal("retry used inference or failed to clear limitation", entry, calls)
	}
}
