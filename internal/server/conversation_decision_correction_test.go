package server

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestServiceConversationCorrectionReceiptContextAndConflicts(t *testing.T) {
	for _, mode := range []string{"apply", "stale", "stop"} {
		t.Run(mode, func(t *testing.T) {
			s, scope, c := contextInboxFixture(t)
			calls := 0
			s.agentRunner = agents.NewRunner(s.agentManager, s.toolRegistry, func(_ context.Context, task *agents.Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
				calls++
				message := api.ChatMessage{Role: "assistant", Content: "Correction is pending."}
				finish := "stop"
				if len(task.Steps) == 0 {
					finish = "tool_calls"
					message.Content = ""
					message.ToolCalls = []api.ToolCall{{Type: "function", Function: api.FunctionCall{Name: "conversation_decide", Arguments: `{"kind":"correction","instruction_version":1}`}}}
				}
				return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: finish, Message: message}}}, nil
			})
			config := agents.DefaultAgentConfig()
			config.TaskFirst = true
			accepted, err := s.agentRunner.AdmitConversationWork(scope, agents.ConversationCommand{RequestID: "work", TargetID: c.ID, ExpectedRevision: c.Revision, Kind: "create_work", Payload: agents.ConversationCommandPayload{Input: &agents.ConversationInput{Content: "Prepare an outline"}}}, "model", config)
			if err != nil {
				t.Fatal(err)
			}
			first := acceptContextInput(t, s, scope, c.ID, "first", "Keep the existing headings")
			completeContextInput(t, s, scope, first, config)
			target := first
			if mode == "stale" {
				target = acceptContextInput(t, s, scope, c.ID, "second", "Preserve the citations too")
				completeContextInput(t, s, scope, target, config)
			} else if mode == "stop" {
				work, _ := s.agentManager.WorkItem(scope, accepted.Work.ID)
				if _, err = s.agentRunner.ControlWork(scope, api.WorkCommand{RequestID: "stop", TargetID: work.ID, ExpectedRevision: work.Revision, Kind: "stop"}); err != nil {
					t.Fatal(err)
				}
			}
			before := calls
			controller := &conversationDeliveryController{server: s}
			if err = controller.coordinate(context.Background(), time.Now()); err != nil {
				t.Fatal(err)
			}
			entry, err := s.agentManager.ConversationInput(scope, c.ID, target.ID)
			if err != nil || calls != before {
				t.Fatal("decision settlement called inference", entry, err, calls)
			}
			if mode == "apply" {
				if entry.AppliedAt == nil || entry.AppliedInstructionVersion != 2 {
					t.Fatal("correction not incorporated", entry)
				}
			} else {
				want := "conversation_decision_stale"
				if mode == "stop" {
					want = "conversation_decision_work_finished"
				}
				if entry.AppliedAt != nil || entry.DecisionIssue == nil || entry.DecisionIssue.Code != want {
					t.Fatal("conflict was applied or hidden", entry)
				}
			}
			contextConfig := agents.AgentConfig{}
			if err = s.assembleConversationInboxContext(httptest.NewRequest("GET", "/", nil), c.ID, "", &contextConfig); err != nil {
				t.Fatal(err)
			}
			last := contextConfig.ConversationContext[len(contextConfig.ConversationContext)-1].StringContent()
			if mode == "apply" && !strings.Contains(last, "applied to work instruction version 2") || mode != "apply" && !strings.Contains(last, "remains pending, not applied") {
				t.Fatal("later context trusts stale reply instead of actual control receipt", last)
			}
		})
	}
}
