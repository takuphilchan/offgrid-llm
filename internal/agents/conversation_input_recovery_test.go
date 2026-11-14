package agents

import (
	"context"
	"errors"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestConversationInputContinuePreservesAttemptAndSpentAllowance(t *testing.T) {
	m, r, scope, c := controlFixture(t)
	input := acceptTestConversationInput(t, r, scope, c, "input")
	calls := 0
	r = NewRunner(m, NewToolRegistry(), func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("fixture interruption")
		}
		return runAnswer(api.ChatMessage{Role: "assistant", Content: "Continued answer"}), nil
	})
	foreground, err := r.AdmitConversationTurn(scope, c.ID, input.ID, c.Revision, "model", taskConfig())
	if err != nil {
		t.Fatal(err)
	}
	foreground, err = r.Continue(context.Background(), foreground.ID, scope.Actor, "start", "", false)
	if err == nil || foreground.Status != TaskInterrupted {
		t.Fatal(foreground, err)
	}
	spent := foreground.Checkpoint.BudgetedModelCalls
	command := inputControl(t, m, scope, c.ID, input.ID, "continue")
	r.SourceAccess = func(*Task) bool { return false }
	if _, err = r.ControlConversationInput(scope, c.ID, command); !errors.Is(err, ErrSourceAccess) {
		t.Fatal("revoked continuation", err)
	}
	r.SourceAccess = nil
	for i := 0; i < 2; i++ {
		receipt, err := r.ControlConversationInput(scope, c.ID, command)
		if err != nil || receipt.Input.RunID != foreground.ID || receipt.Input.RunStatus != TaskPending {
			t.Fatal(receipt, err)
		}
	}
	continued, _ := m.GetTask(foreground.ID)
	if continued.Checkpoint.BudgetedModelCalls != spent || !continued.SchedulerReady {
		t.Fatal("continuation reset allowance", continued)
	}
	continued, err = r.Continue(context.Background(), foreground.ID, scope.Actor, "start", "", false)
	if err != nil || continued.Status != TaskCompleted || continued.Checkpoint.BudgetedModelCalls != spent+1 || calls != 2 {
		t.Fatal(continued, err)
	}
}

func TestConversationInputRevalidateRetainsOriginalDecisionAndAppliesOnce(t *testing.T) {
	m, r, scope, c := controlFixture(t)
	work := admitControlTestWork(t, r, scope, c)
	c, _ = m.Conversation(scope, c.ID)
	first := acceptTestConversationInput(t, r, scope, c, "first")
	second := acceptTestConversationInput(t, r, scope, c, "second")
	r.caller = func(_ context.Context, task *Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		if len(task.Steps) == 0 {
			return taskCall("conversation_decide", `{"kind":"correction","instruction_version":1}`), nil
		}
		return runAnswer(api.ChatMessage{Role: "assistant", Content: "Correction pending"}), nil
	}
	finishTestConversationTurn(t, r, scope, c, first)
	c, _ = m.Conversation(scope, c.ID)
	finishTestConversationTurn(t, r, scope, c, second)
	if _, err := r.ApplyConversationDecision(scope, c.ID, first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := r.ApplyConversationDecision(scope, c.ID, second.ID); !errors.Is(err, ErrRunConflict) {
		t.Fatal("stale correction applied", err)
	}
	command := inputControl(t, m, scope, c.ID, second.ID, "revalidate")
	command.Payload.InstructionVersion = 1
	if _, err := r.ControlConversationInput(scope, c.ID, command); !errors.Is(err, ErrRunConflict) {
		t.Fatal("stale revalidation", err)
	}
	command.Payload.InstructionVersion = 2
	for i := 0; i < 2; i++ {
		receipt, err := r.ControlConversationInput(scope, c.ID, command)
		if err != nil || receipt.Input.RevalidatedAt == nil || receipt.Input.Decision.InstructionVersion != 1 || receipt.Input.RevalidatedInstructionVersion != 2 || receipt.Input.AppliedAt != nil {
			t.Fatal(receipt, err)
		}
	}
	entry, err := r.ApplyConversationDecision(scope, c.ID, second.ID)
	if err != nil || entry.AppliedInstructionVersion != 3 {
		t.Fatal(entry, err)
	}
	root, _ := m.GetTask(work.RunID)
	if root.InstructionVersion != 3 || len(root.Instructions) != 2 || root.Checkpoint.BudgetedModelCalls != 0 {
		t.Fatal("revalidation lost/repeated instructions or reset budget", root)
	}
}
