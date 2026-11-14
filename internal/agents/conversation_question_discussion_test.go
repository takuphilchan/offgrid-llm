package agents

import (
	"context"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestConversationSideQuestionPreservesPendingWorkerClarification(t *testing.T) {
	m, r, scope, root, child, c := conversationQuestionFixture(t)
	command := inboxCommand(c, "side-question", "What does offline runtime mean?")
	command.Payload.Intent = "message"
	accepted, err := r.AcceptConversationInput(scope, command)
	if err != nil {
		t.Fatal(err)
	}
	r.caller = func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		return runAnswer(api.ChatMessage{Role: "assistant", Content: "It runs the model locally without a hosted inference service."}), nil
	}
	foreground := finishTestConversationTurn(t, r, scope, accepted.Conversation, accepted.Input)
	if _, err = r.ApplyConversationDecision(scope, c.ID, accepted.Input.ID); err != nil {
		t.Fatal(err)
	}
	parent, _ := m.GetTask(root.ID)
	worker, _ := m.GetTask(child.ID)
	if parent.Status != TaskInput || parent.PendingInput == nil || parent.PendingInput.ID != root.PendingInput.ID || hasConversationHold(parent) || hasConversationHold(worker) || worker.WorkerQuestion.State != "pending" || parent.Checkpoint.BudgetedModelCalls != root.Checkpoint.BudgetedModelCalls || foreground.Checkpoint.BudgetedModelCalls != 1 {
		t.Fatal("side question answered the clarification, held the graph or changed the work budget")
	}
	c, _ = m.Conversation(scope, c.ID)
	if _, err = r.AcceptConversationInput(scope, questionInputCommand(c, parent)); err != nil {
		t.Fatal("clarification could not be answered after discussion", err)
	}
}
