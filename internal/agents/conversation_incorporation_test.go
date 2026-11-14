package agents

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func finishTestConversationTurn(t *testing.T, r *Runner, scope ControlScope, c *Conversation, input *api.ConversationInboxEntry) *Task {
	t.Helper()
	task, err := r.AdmitConversationTurn(scope, c.ID, input.ID, c.Revision, "model", taskConfig())
	if err != nil {
		t.Fatal(err)
	}
	task, err = r.Continue(context.Background(), task.ID, scope.Actor, "start", "", false)
	if err != nil || task.Status != TaskCompleted {
		t.Fatal(task, err)
	}
	return task
}

func TestConversationIncorporationAnswerPreservesControlAndRecovery(t *testing.T) {
	for _, control := range []string{"", "pause", "stop", "restart", "expired"} {
		t.Run(control, func(t *testing.T) {
			m, r, scope, c := controlFixture(t)
			work := admitControlTestWork(t, r, scope, c)
			c, _ = m.Conversation(scope, c.ID)
			input := acceptTestConversationInput(t, r, scope, c, "input")
			foreground := finishTestConversationTurn(t, r, scope, c, input)
			switch control {
			case "pause", "stop":
				work, _ = m.WorkItem(scope, work.ID)
				if _, err := r.ControlWork(scope, api.WorkCommand{RequestID: "control", TargetID: work.ID, ExpectedRevision: work.Revision, Kind: control}); err != nil {
					t.Fatal(err)
				}
			case "restart":
				m = NewManagerWithPersistence(nil, nil, nil, m.dataDir)
				if err := m.StorageError(); err != nil {
					t.Fatal(err)
				}
				r = NewRunner(m, NewToolRegistry(), nil)
			case "expired":
				if _, err := m.updateTask(work.RunID, func(next *Task) error { next.ComputerSessionExpired = true; return nil }); err != nil {
					t.Fatal(err)
				}
			}
			entry, err := r.IncorporateConversationInput(scope, c.ID, input.ID, foreground.ID, "answered", 0)
			if err != nil || entry.AppliedAt == nil || entry.Incorporation != "answered" {
				t.Fatal(entry, err)
			}
			root, _ := m.GetTask(work.RunID)
			if hasConversationHold(root) || root.Checkpoint.BudgetedModelCalls != 0 || root.InstructionVersion != 1 {
				t.Fatal("answer changed allowance/instructions or retained hold", root)
			}
			if control == "" {
				if root.Status != TaskPending || !root.SchedulerReady {
					t.Fatal("active work did not become eligible", root)
				}
			} else if root.SchedulerReady || root.Status == TaskPending || root.Status == TaskRunning || root.Status == TaskConversationInput {
				t.Fatal("answer overrode control/recovery", root)
			}
		})
	}
}

func TestConversationIncorporationOrderAndNoUnresolvedDispatch(t *testing.T) {
	m, r, scope, c := controlFixture(t)
	work := admitControlTestWork(t, r, scope, c)
	c, _ = m.Conversation(scope, c.ID)
	first := acceptTestConversationInput(t, r, scope, c, "first")
	second := acceptTestConversationInput(t, r, scope, c, "second")
	firstRun := finishTestConversationTurn(t, r, scope, c, first)
	c, _ = m.Conversation(scope, c.ID)
	secondRun := finishTestConversationTurn(t, r, scope, c, second)
	if _, err := r.IncorporateConversationInput(scope, c.ID, second.ID, secondRun.ID, "answered", 0); !errors.Is(err, ErrRunConflict) {
		t.Fatal("input applied out of committed order", err)
	}
	r.active[work.RunID] = func() {}
	if _, err := r.IncorporateConversationInput(scope, c.ID, first.ID, firstRun.ID, "answered", 0); !errors.Is(err, ErrRunConflict) {
		t.Fatal("released active dispatch", err)
	}
	delete(r.active, work.RunID)
	if _, err := r.IncorporateConversationInput(scope, c.ID, first.ID, firstRun.ID, "answered", 0); err != nil {
		t.Fatal(err)
	}
	root, _ := m.GetTask(work.RunID)
	if !hasConversationHold(root) || len(root.ConversationHold.InputIDs) != 1 || root.ConversationHold.InputIDs[0] != second.ID || root.SchedulerReady {
		t.Fatal("first incorporation released second hold", root)
	}
	if _, err := r.IncorporateConversationInput(scope, c.ID, second.ID, secondRun.ID, "answered", 0); err != nil {
		t.Fatal(err)
	}
}

func TestConversationIncorporationCorrectionAtomicAndOnce(t *testing.T) {
	m, r, scope, c := controlFixture(t)
	work := admitControlTestWork(t, r, scope, c)
	c, _ = m.Conversation(scope, c.ID)
	command := inboxCommand(c, "correction", "Keep the user's existing changes")
	command.Payload.Intent = "correction"
	receipt, err := r.AcceptConversationInput(scope, command)
	if err != nil {
		t.Fatal(err)
	}
	c, input := receipt.Conversation, receipt.Input
	foreground := finishTestConversationTurn(t, r, scope, c, input)
	if _, err = r.IncorporateConversationInput(scope, c.ID, input.ID, foreground.ID, "answered", 1); !errors.Is(err, ErrControlCommand) {
		t.Fatal("explicit correction was treated as a side question", err)
	}
	if _, err = r.IncorporateConversationInput(scope, c.ID, input.ID, foreground.ID, "correction", 99); !errors.Is(err, ErrRunConflict) {
		t.Fatal("stale preparation applied", err)
	}
	db, err := m.controlDatabase()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TRIGGER reject_incorporation BEFORE INSERT ON agent_control_events WHEN NEW.kind='input_applied' BEGIN SELECT RAISE(ABORT,'incorporation fault'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err = r.IncorporateConversationInput(scope, c.ID, input.ID, foreground.ID, "correction", 1); err == nil {
		t.Fatal("fault ignored")
	}
	root, _ := m.GetTask(work.RunID)
	if root.InstructionVersion != 1 || !hasConversationHold(root) || len(root.Instructions) != 0 {
		t.Fatal("uncommitted correction published", root)
	}
	if _, err = db.Exec(`DROP TRIGGER reject_incorporation`); err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	for i := 0; i < 8; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			entry, err := r.IncorporateConversationInput(scope, c.ID, input.ID, foreground.ID, "correction", 1)
			if err != nil || entry.AppliedInstructionVersion != 2 {
				t.Error(entry, err)
			}
		}()
	}
	group.Wait()
	root, _ = m.GetTask(work.RunID)
	if root.InstructionVersion != 2 || hasConversationHold(root) || len(root.Instructions) != 1 || root.Instructions[0].Text != command.Payload.Content || root.Checkpoint.BudgetedModelCalls != 0 || root.Status != TaskPending {
		t.Fatal("correction duplicated, rewritten or reset work", root)
	}
}
