package agents

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestConversationDecisionQueueRecoveryAndReceiptFault(t *testing.T) {
	m, r, scope, c := controlFixture(t)
	work := admitControlTestWork(t, r, scope, c)
	c, _ = m.Conversation(scope, c.ID)
	input := acceptTestConversationInput(t, r, scope, c, "question")
	finishTestConversationTurn(t, r, scope, c, input)
	issue := api.ConversationDeliveryIssue{Code: "waiting", Message: "Waiting for dispatch settlement", RetryAt: time.Now().Add(time.Second).UTC()}
	if err := m.DeferConversationDecision(scope, c.ID, input.ID, issue); err != nil {
		t.Fatal(err)
	}
	foreign, err := m.ConversationDecisions("foreign", 0, 1)
	if err != nil || len(foreign.Items) != 0 {
		t.Fatal(foreign, err)
	}
	page, err := m.ConversationDecisions(scope.Workspace, 0, 1)
	if err != nil || len(page.Items) != 1 || page.Items[0].Scope != scope || page.Items[0].Input.ID != input.ID || page.Items[0].Input.DecisionIssue == nil {
		t.Fatal(page, err)
	}
	if _, err = r.ApplyConversationDecision(ControlScope{scope.Workspace, "foreign"}, c.ID, input.ID); !errors.Is(err, ErrControlNotFound) {
		t.Fatal("foreign decision access", err)
	}
	m = NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err = m.StorageError(); err != nil {
		t.Fatal(err)
	}
	r = NewRunner(m, NewToolRegistry(), nil)
	db, err := m.controlDatabase()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TRIGGER reject_decision BEFORE INSERT ON agent_control_events WHEN NEW.kind='input_applied' BEGIN SELECT RAISE(ABORT,'decision fault'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err = r.ApplyConversationDecision(scope, c.ID, input.ID); err == nil {
		t.Fatal("receipt write failure ignored")
	}
	entry, _ := m.ConversationInput(scope, c.ID, input.ID)
	root, _ := m.GetTask(work.RunID)
	if entry.AppliedAt != nil || !hasConversationHold(root) || entry.DecisionIssue == nil {
		t.Fatal("partial decision published", entry)
	}
	if _, err = db.Exec(`DROP TRIGGER reject_decision`); err != nil {
		t.Fatal(err)
	}
	if entry, err = r.ApplyConversationDecision(scope, c.ID, input.ID); err != nil || entry.DecisionIssue != nil {
		t.Fatal(entry, err)
	}
	root, _ = m.GetTask(work.RunID)
	if root.Status != TaskInterrupted || root.SchedulerReady || hasConversationHold(root) {
		t.Fatal("bookkeeping resumed interrupted work", root)
	}
	if err = m.DeferConversationDecision(scope, c.ID, input.ID, issue); !errors.Is(err, ErrRunConflict) {
		t.Fatal("late deferral overwrote application receipt", err)
	}
	page, err = m.ConversationDecisions(scope.Workspace, 0, 1)
	if err != nil || len(page.Items) != 0 {
		t.Fatal("applied decision still queued", page, err)
	}
}

func TestConversationDecisionRecordedBeforeFailureDoesNotApply(t *testing.T) {
	m, r, scope, c := controlFixture(t)
	work := admitControlTestWork(t, r, scope, c)
	c, _ = m.Conversation(scope, c.ID)
	input := acceptTestConversationInput(t, r, scope, c, "correction")
	calls := 0
	r = NewRunner(m, NewToolRegistry(), func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		if calls == 1 {
			return taskCall("conversation_decide", `{"kind":"correction","instruction_version":1}`), nil
		}
		return nil, errors.New("fixture inference failed")
	})
	foreground, err := r.AdmitConversationTurn(scope, c.ID, input.ID, c.Revision, "model", taskConfig())
	if err != nil {
		t.Fatal(err)
	}
	foreground, err = r.Continue(context.Background(), foreground.ID, scope.Actor, "start", "", false)
	if err == nil || foreground.Status != TaskInterrupted {
		t.Fatal(foreground, err)
	}
	entry, _ := m.ConversationInput(scope, c.ID, input.ID)
	if entry.Decision == nil || entry.AppliedAt != nil {
		t.Fatal("failed decision lost or applied", entry)
	}
	if _, err = r.ApplyConversationDecision(scope, c.ID, input.ID); !errors.Is(err, ErrRunConflict) {
		t.Fatal("failed foreground applied decision", err)
	}
	root, _ := m.GetTask(work.RunID)
	if !hasConversationHold(root) || root.InstructionVersion != 1 {
		t.Fatal("failure changed work", root)
	}
}
