package agents

import (
	"context"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestConversationIncorporationRetiresSupersededGraph(t *testing.T) {
	for _, complete := range []bool{false, true} {
		t.Run(map[bool]string{false: "pending-child", true: "completed-child"}[complete], func(t *testing.T) {
			m, r, scope, root, child := workerFixture(t)
			if complete {
				var err error
				child, err = r.Continue(context.Background(), child.ID, scope.Actor, "start", "", false)
				if err != nil || child.Status != TaskCompleted {
					t.Fatal(child, err)
				}
			}
			before, _ := m.GetTask(root.ID)
			c, _ := m.Conversation(scope, root.Origin.Conversation)
			command := inboxCommand(c, "correction", "Use the updated source instead")
			command.Payload.Intent = "correction"
			receipt, err := r.AcceptConversationInput(scope, command)
			if err != nil {
				t.Fatal(err)
			}
			c = receipt.Conversation
			r.caller = func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
				return runAnswer(api.ChatMessage{Role: "assistant", Content: "I will use the updated source."}), nil
			}
			foreground := finishTestConversationTurn(t, r, scope, c, receipt.Input)
			if _, err = r.IncorporateConversationInput(scope, c.ID, receipt.Input.ID, foreground.ID, "correction", root.InstructionVersion); err != nil {
				t.Fatal(err)
			}
			updated, _ := m.GetTask(root.ID)
			if updated.Delegation != nil || updated.Status != TaskPending || !updated.SchedulerReady || hasConversationHold(updated) || updated.InstructionVersion != root.InstructionVersion+1 || !updated.ChildHistory[0].Invalidated || updated.Checkpoint.BudgetedModelCalls != before.Checkpoint.BudgetedModelCalls || len(updated.Checkpoint.Calls) != 0 {
				t.Fatal("correction left old graph runnable or changed allowance", updated)
			}
			retired, _ := m.GetTask(child.ID)
			if hasConversationHold(retired) || retired.SchedulerReady {
				t.Fatal("retired worker retained an active fence/queue entry", retired)
			}
			if complete {
				if retired.Status != TaskCompleted || retired.Result != child.Result || resultReference(retired) != resultReference(child) {
					t.Fatal("correction falsified prior evidence", retired)
				}
			} else if retired.Status != TaskCancelled || retired.ErrorCode != "instruction_superseded" {
				t.Fatal("obsolete assignment remained runnable", retired)
			}
			loaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
			if err = loaded.StorageError(); err != nil {
				t.Fatal("corrected graph cannot recover", err)
			}
		})
	}
}
