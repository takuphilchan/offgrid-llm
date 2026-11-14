package agents

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func inputControl(t *testing.T, m *Manager, scope ControlScope, conversation, input, kind string) api.ConversationInputControlCommand {
	t.Helper()
	c, err := m.Conversation(scope, conversation)
	if err != nil {
		t.Fatal(err)
	}
	return api.ConversationInputControlCommand{RequestID: kind + "-" + input, TargetID: input, ExpectedRevision: c.Revision, Kind: kind}
}

func TestConversationInputStopQueuedOutOfOrderAndAfterRestart(t *testing.T) {
	m, r, scope, c := controlFixture(t)
	work := admitControlTestWork(t, r, scope, c)
	c, _ = m.Conversation(scope, c.ID)
	first := acceptTestConversationInput(t, r, scope, c, "first")
	second := acceptTestConversationInput(t, r, scope, c, "second")
	command := inputControl(t, m, scope, c.ID, second.ID, "stop")
	r.SourceAccess = func(*Task) bool { return false } // withdrawal remains possible
	for i := 0; i < 2; i++ {
		receipt, err := r.ControlConversationInput(scope, c.ID, command)
		if err != nil || receipt.Input.State != "stopping" || receipt.EventCursor < 1 {
			t.Fatal(receipt, err)
		}
	}
	entry, err := r.SettleStoppedConversationInput(scope, c.ID, second.ID)
	if err != nil || entry.State != "stopped" || entry.HoldReleasedAt == nil || entry.RunID != "" {
		t.Fatal(entry, err)
	}
	root, _ := m.GetTask(work.RunID)
	if !hasConversationHold(root) || len(root.ConversationHold.InputIDs) != 1 || root.ConversationHold.InputIDs[0] != first.ID || root.SchedulerReady {
		t.Fatal("withdrawing later input released earlier hold", root)
	}
	command = inputControl(t, m, scope, c.ID, first.ID, "stop")
	if _, err = r.ControlConversationInput(scope, c.ID, command); err != nil {
		t.Fatal(err)
	}
	m = NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err = m.StorageError(); err != nil {
		t.Fatal(err)
	}
	r = NewRunner(m, NewToolRegistry(), nil)
	entry, err = r.SettleStoppedConversationInput(scope, c.ID, first.ID)
	if err != nil || entry.State != "stopped" {
		t.Fatal(entry, err)
	}
	root, _ = m.GetTask(work.RunID)
	if hasConversationHold(root) || root.Status != TaskInterrupted || root.SchedulerReady {
		t.Fatal("withdrawal resumed recovered work", root)
	}
	page, err := m.ConversationDeliveries(scope.Workspace, 0, 10)
	if err != nil || len(page.Items) != 0 {
		t.Fatal("withdrawn input redelivered", page, err)
	}
}

func TestConversationInputStopCancelsGenerationBeforeLateDecision(t *testing.T) {
	m, r, scope, c := controlFixture(t)
	input := acceptTestConversationInput(t, r, scope, c, "input")
	started := make(chan struct{})
	r = NewRunner(m, NewToolRegistry(), func(ctx context.Context, _ *Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		close(started)
		<-ctx.Done()
		return taskCall("conversation_decide", `{"kind":"new_work","instruction_version":0,"expected_result":"Late work must not start"}`), nil
	})
	defer r.Shutdown(context.Background())
	foreground, err := r.AdmitConversationTurn(scope, c.ID, input.ID, c.Revision, "model", taskConfig())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.Continue(context.Background(), foreground.ID, scope.Actor, "start", "", true); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("generation did not start")
	}
	command := inputControl(t, m, scope, c.ID, input.ID, "stop")
	db, err := m.controlDatabase()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TRIGGER reject_input_stop BEFORE INSERT ON agent_control_events WHEN NEW.kind='input_stop' BEGIN SELECT RAISE(ABORT,'stop fault'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err = r.ControlConversationInput(scope, c.ID, command); err == nil {
		t.Fatal("stop receipt fault ignored")
	}
	unchanged, _ := m.GetTask(foreground.ID)
	retained, _ := m.ConversationInput(scope, c.ID, input.ID)
	if unchanged.Status != TaskRunning || retained.StoppedAt != nil || retained.State != "processing" {
		t.Fatal("uncommitted Stop published/cancelled", unchanged, retained)
	}
	if _, err = db.Exec(`DROP TRIGGER reject_input_stop`); err != nil {
		t.Fatal(err)
	}
	receipt, err := r.ControlConversationInput(scope, c.ID, command)
	if err != nil || receipt.Input.StoppedAt == nil || receipt.Input.State != "stopping" {
		t.Fatal(receipt, err)
	}
	settleTask(t, r, foreground.ID)
	r.settleWorkControls()
	entry, err := r.SettleStoppedConversationInput(scope, c.ID, input.ID)
	if err != nil || entry.State != "stopped" || entry.Decision != nil || entry.CreatedWorkID != "" {
		t.Fatal("late proposal survived withdrawal", entry, err)
	}
	command = inputControl(t, m, scope, c.ID, input.ID, "continue")
	if _, err = r.ControlConversationInput(scope, c.ID, command); !errors.Is(err, ErrRunConflict) {
		t.Fatal("stopped attempt silently replaced", err)
	}
}

func TestConversationInputStopBeforeWorkAdmissionAndAfterApplication(t *testing.T) {
	for _, admit := range []bool{false, true} {
		m, r, scope, c := controlFixture(t)
		input := acceptTestConversationInput(t, r, scope, c, "input")
		r = finishNewWorkDecision(t, m, scope, c, input)
		c, _ = m.Conversation(scope, c.ID)
		if admit {
			if _, err := r.AdmitConversationInputWork(scope, c.ID, input.ID, c.Revision, "model", taskConfig()); err != nil {
				t.Fatal(err)
			}
		}
		command := inputControl(t, m, scope, c.ID, input.ID, "stop")
		receipt, err := r.ControlConversationInput(scope, c.ID, command)
		if err != nil {
			t.Fatal(err)
		}
		if admit {
			if receipt.Input.CreatedWorkID == "" || receipt.Input.StoppedAt != nil {
				t.Fatal("Stop undid admitted work", receipt)
			}
		} else {
			c, _ = m.Conversation(scope, c.ID)
			if _, err = r.AdmitConversationInputWork(scope, c.ID, input.ID, c.Revision, "model", taskConfig()); !errors.Is(err, ErrRunConflict) {
				t.Fatal("withdrawn decision created work", err)
			}
		}
	}
}
