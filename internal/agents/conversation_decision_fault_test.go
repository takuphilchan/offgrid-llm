package agents

import (
	"context"
	"errors"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestConversationDecisionProposalWriteFailureCannotApply(t *testing.T) {
	m, r, scope, c := controlFixture(t)
	work := admitControlTestWork(t, r, scope, c)
	c, _ = m.Conversation(scope, c.ID)
	input := acceptTestConversationInput(t, r, scope, c, "correction")
	db, err := m.controlDatabase()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TRIGGER reject_proposal BEFORE UPDATE ON agent_conversation_inbox WHEN json_extract(NEW.payload,'$.decision.kind')='correction' BEGIN SELECT RAISE(ABORT,'proposal fault'); END`); err != nil {
		t.Fatal(err)
	}
	r = NewRunner(m, NewToolRegistry(), func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		return taskCall("conversation_decide", `{"kind":"correction","instruction_version":1}`), nil
	})
	foreground, err := r.AdmitConversationTurn(scope, c.ID, input.ID, c.Revision, "model", taskConfig())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.Continue(context.Background(), foreground.ID, scope.Actor, "start", "", false); err == nil {
		t.Fatal("proposal fault ignored")
	}
	// Storage faults deliberately poison the live manager. Recover from disk
	// after removing the injected fault; do not bypass that admission fence.
	if !errors.Is(m.StorageError(), ErrRunStorage) {
		t.Fatal("storage failure did not fence the manager")
	}
	if _, err = db.Exec(`DROP TRIGGER reject_proposal`); err != nil {
		t.Fatal(err)
	}
	m = NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err = m.StorageError(); err != nil {
		t.Fatal(err)
	}
	r = NewRunner(m, NewToolRegistry(), nil)
	entry, err := m.ConversationInput(scope, c.ID, input.ID)
	if err != nil {
		t.Fatal(err)
	}
	if entry.Decision != nil || entry.AppliedAt != nil {
		t.Fatal("uncommitted proposal published", entry)
	}
	if _, err = r.ApplyConversationDecision(scope, c.ID, input.ID); !errors.Is(err, ErrRunConflict) {
		t.Fatal("uncommitted proposal applied", err)
	}
	root, _ := m.GetTask(work.RunID)
	if !hasConversationHold(root) || root.InstructionVersion != 1 || len(root.Instructions) != 0 {
		t.Fatal("failed proposal changed root", root)
	}
}

func TestConversationDecisionStaleCorrectionRetainedAndPagingBounded(t *testing.T) {
	m, r, scope, c := controlFixture(t)
	work := admitControlTestWork(t, r, scope, c)
	c, _ = m.Conversation(scope, c.ID)
	first := acceptTestConversationInput(t, r, scope, c, "first")
	second := acceptTestConversationInput(t, r, scope, c, "second")
	r = NewRunner(m, NewToolRegistry(), func(_ context.Context, task *Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		if len(task.Steps) == 0 {
			return taskCall("conversation_decide", `{"kind":"correction","instruction_version":1}`), nil
		}
		return runAnswer(api.ChatMessage{Role: "assistant", Content: "Correction recorded, not yet applied"}), nil
	})
	finishTestConversationTurn(t, r, scope, c, first)
	c, _ = m.Conversation(scope, c.ID)
	finishTestConversationTurn(t, r, scope, c, second)
	page, err := m.ConversationDecisions(scope.Workspace, 0, 1)
	if err != nil || len(page.Items) != 1 || page.Items[0].Input.ID != first.ID || !page.HasMore {
		t.Fatal(page, err)
	}
	page, err = m.ConversationDecisions(scope.Workspace, page.NextCursor, 1)
	if err != nil || len(page.Items) != 1 || page.Items[0].Input.ID != second.ID || page.HasMore {
		t.Fatal(page, err)
	}
	if _, err = r.ApplyConversationDecision(scope, c.ID, first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = r.ApplyConversationDecision(scope, c.ID, second.ID); !errors.Is(err, ErrRunConflict) {
		t.Fatal("stale proposal applied", err)
	}
	entry, _ := m.ConversationInput(scope, c.ID, second.ID)
	root, _ := m.GetTask(work.RunID)
	if entry.Decision == nil || entry.AppliedAt != nil || root.InstructionVersion != 2 || !hasConversationHold(root) || len(root.ConversationHold.InputIDs) != 1 || root.ConversationHold.InputIDs[0] != second.ID {
		t.Fatal("stale decision lost or its hold released", entry, root)
	}
}
