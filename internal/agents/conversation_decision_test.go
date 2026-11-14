package agents

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestConversationCorrectionRechecksFocusedWorkSources(t *testing.T) {
	m, r, scope, c := controlFixture(t)
	work := admitControlTestWork(t, r, scope, c)
	c, _ = m.Conversation(scope, c.ID)
	command := inboxCommand(c, "source-correction", "Correct this detail")
	command.Payload.Intent = "correction"
	receipt, err := r.AcceptConversationInput(scope, command)
	if err != nil {
		t.Fatal(err)
	}
	r.caller = func(_ context.Context, task *Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		if len(task.Steps) == 0 {
			return taskCall("conversation_decide", `{"kind":"correction","instruction_version":1}`), nil
		}
		return runAnswer(api.ChatMessage{Role: "assistant", Content: "Correction saved"}), nil
	}
	foreground := finishTestConversationTurn(t, r, scope, receipt.Conversation, receipt.Input)
	r.SourceAccess = func(task *Task) bool { return task.ID != work.RunID }
	if _, err = r.ApplyConversationDecision(scope, c.ID, receipt.Input.ID); !errors.Is(err, ErrSourceAccess) {
		t.Fatal("revocation allowed correction", err)
	}
	if _, err = r.IncorporateConversationInput(scope, c.ID, receipt.Input.ID, foreground.ID, "correction", 1); !errors.Is(err, ErrSourceAccess) {
		t.Fatal("direct incorporation bypassed access", err)
	}
	root, _ := m.GetTask(work.RunID)
	if root.InstructionVersion != 1 || len(root.Instructions) != 0 {
		t.Fatal("revoked correction mutated work")
	}
}

func TestConversationDecisionCorrectionUsesNormalTurnAndSettledCheckpoint(t *testing.T) {
	m, r, scope, c := controlFixture(t)
	work := admitControlTestWork(t, r, scope, c)
	c, _ = m.Conversation(scope, c.ID)
	command := inboxCommand(c, "correction", "Preserve the existing draft; change only its title")
	command.Payload.Intent = "correction"
	receipt, err := r.AcceptConversationInput(scope, command)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	r = NewRunner(m, NewToolRegistry(), func(_ context.Context, task *Task, messages []api.ChatMessage, tools []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		if calls == 1 {
			found := false
			for _, tool := range tools {
				found = found || tool.Function.Name == "conversation_decide"
			}
			if !found {
				t.Error("foreground coordination tool missing")
			}
			return taskCall("conversation_decide", `{"kind":"correction","instruction_version":1}`), nil
		}
		entry, _ := m.ConversationInput(scope, c.ID, receipt.Input.ID)
		root, _ := m.GetTask(work.RunID)
		if entry.Decision == nil || entry.AppliedAt != nil || !hasConversationHold(root) || len(root.Instructions) != 0 {
			t.Error("decision was not durably pending", entry)
		}
		if !strings.Contains(messages[len(messages)-1].StringContent(), `"applied":false`) {
			t.Error("tool overstated application")
		}
		return runAnswer(api.ChatMessage{Role: "assistant", Content: "Your correction is recorded and waiting for the current operation to settle."}), nil
	})
	foreground := finishTestConversationTurn(t, r, scope, receipt.Conversation, receipt.Input)
	if calls != 2 {
		t.Fatal("extra classification or retry inference", calls)
	}
	r.active[work.RunID] = func() {}
	if _, err = r.ApplyConversationDecision(scope, c.ID, receipt.Input.ID); !errors.Is(err, ErrRunConflict) {
		t.Fatal("correction bypassed active dispatch", err)
	}
	delete(r.active, work.RunID)
	for i := 0; i < 2; i++ {
		entry, err := r.ApplyConversationDecision(scope, c.ID, receipt.Input.ID)
		if err != nil || entry.AppliedAt == nil || entry.AppliedInstructionVersion != 2 {
			t.Fatal(entry, err)
		}
	}
	root, _ := m.GetTask(work.RunID)
	if root.InstructionVersion != 2 || len(root.Instructions) != 1 || root.Instructions[0].Text != command.Payload.Content || root.Checkpoint.BudgetedModelCalls != 0 || calls != 2 || foreground.Checkpoint.BudgetedModelCalls != 2 {
		t.Fatal("correction rewrote input, repeated, or spent the work allowance", root)
	}
}

func TestConversationDecisionAnswerNeedsOneCallAndCannotChangeDisposition(t *testing.T) {
	m, r, scope, c := controlFixture(t)
	work := admitControlTestWork(t, r, scope, c)
	c, _ = m.Conversation(scope, c.ID)
	input := acceptTestConversationInput(t, r, scope, c, "question")
	foreground := finishTestConversationTurn(t, r, scope, c, input)
	entry, _ := m.ConversationInput(scope, c.ID, input.ID)
	if entry.Decision == nil || entry.Decision.Kind != "answered" || entry.AppliedAt != nil || foreground.Checkpoint.BudgetedModelCalls != 1 {
		t.Fatal("ordinary answer added inference or silently applied", entry, foreground.Checkpoint.BudgetedModelCalls)
	}
	if _, err := r.IncorporateConversationInput(scope, c.ID, input.ID, foreground.ID, "correction", 1); !errors.Is(err, ErrRunConflict) {
		t.Fatal("completed answer changed into a correction", err)
	}
	r.SourceAccess = func(*Task) bool { return false }
	if _, err := r.ApplyConversationDecision(scope, c.ID, input.ID); !errors.Is(err, ErrSourceAccess) {
		t.Fatal("revoked decision applied", err)
	}
	r.SourceAccess = nil
	if _, err := r.ApplyConversationDecision(scope, c.ID, input.ID); err != nil {
		t.Fatal(err)
	}
	root, _ := m.GetTask(work.RunID)
	if hasConversationHold(root) || root.InstructionVersion != 1 || root.Checkpoint.BudgetedModelCalls != 0 {
		t.Fatal("side question changed work", root)
	}
}

func TestConversationDecisionDoesNotExpandQualifiedNativeCatalog(t *testing.T) {
	foreground := &Task{Config: taskConfig(), ConversationTurn: &ConversationTurnBinding{InputID: "input"}}
	if !qualifiedTool(foreground, "conversation_decide", capabilities.Descriptor{}, false) {
		t.Fatal("unconstrained foreground decision missing")
	}
	foreground.Config.ToolAdapter = &inference.AgentToolPin{Adapter: inference.NativeAgentToolAdapter, Scope: "invalid"}
	if qualifiedTool(foreground, "conversation_decide", capabilities.Descriptor{}, false) || scopedToolName("conversation_decide") {
		t.Fatal("new tool inherited native qualification")
	}
	foreground.Config.ToolAdapter = &inference.AgentToolPin{Adapter: inference.NativeAgentToolAdapter}
	if qualifiedTool(foreground, "conversation_decide", capabilities.Descriptor{}, false) {
		t.Fatal("new tool inherited legacy full-suite qualification")
	}
	ordinary := &Task{Config: taskConfig()}
	if qualifiedTool(ordinary, "conversation_decide", capabilities.Descriptor{}, false) {
		t.Fatal("work/worker acquired foreground control")
	}
}

func TestConversationDecisionExplicitIntentsDoNotBecomeAnswers(t *testing.T) {
	for _, intent := range []string{"correction", "new_work", "answer"} {
		t.Run(intent, func(t *testing.T) {
			m, r, scope, c := controlFixture(t)
			admitControlTestWork(t, r, scope, c)
			c, _ = m.Conversation(scope, c.ID)
			command := inboxCommand(c, "explicit", "Accepted input")
			command.Payload.Intent = intent
			receipt, err := r.AcceptConversationInput(scope, command)
			if intent == "answer" {
				if err != ErrControlCommand {
					t.Fatal("unidentified answer admitted", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			finishTestConversationTurn(t, r, scope, receipt.Conversation, receipt.Input)
			entry, _ := m.ConversationInput(scope, c.ID, receipt.Input.ID)
			if entry.Decision != nil || entry.AppliedAt != nil {
				t.Fatal("explicit input silently became a side question", entry)
			}
		})
	}
}
