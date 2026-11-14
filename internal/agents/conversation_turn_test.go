package agents

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func acceptTestConversationInput(t *testing.T, r *Runner, scope ControlScope, c *Conversation, id string) *api.ConversationInboxEntry {
	t.Helper()
	receipt, err := r.AcceptConversationInput(scope, inboxCommand(c, id, "Explain the current progress"))
	if err != nil {
		t.Fatal(err)
	}
	*c = *receipt.Conversation
	return receipt.Input
}

func TestConversationTurnFIFOConcurrentRetryAndWorkSeparation(t *testing.T) {
	m, r, scope, c := controlFixture(t)
	work := admitControlTestWork(t, r, scope, c)
	c, _ = m.Conversation(scope, c.ID)
	first := acceptTestConversationInput(t, r, scope, c, "first")
	second := acceptTestConversationInput(t, r, scope, c, "second")
	if _, err := r.AdmitConversationTurn(scope, c.ID, second.ID, c.Revision, "model", taskConfig()); !errors.Is(err, ErrRunConflict) {
		t.Fatal("out-of-order delivery", err)
	}
	var wg sync.WaitGroup
	ids := make(chan string, 8)
	for i := 0; i < cap(ids); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			task, err := r.AdmitConversationTurn(scope, c.ID, first.ID, c.Revision, "model", taskConfig())
			if err != nil {
				t.Error(err)
				return
			}
			ids <- task.ID
		}()
	}
	wg.Wait()
	close(ids)
	id := ""
	for got := range ids {
		if id != "" && got != id {
			t.Fatal("duplicate foreground runs", id, got)
		}
		id = got
	}
	if id == "" {
		t.Fatal("no foreground run")
	}
	c, _ = m.Conversation(scope, c.ID)
	if _, err := r.AdmitConversationTurn(scope, c.ID, second.ID, c.Revision, "model", taskConfig()); !errors.Is(err, ErrRunConflict) {
		t.Fatal("overlapping foreground turns", err)
	}
	task, err := r.Continue(context.Background(), id, scope.Actor, "start", "", false)
	if err != nil || task.Status != TaskCompleted || task.Checkpoint.BudgetedModelCalls != 1 || task.WorkID != "" || task.ParentID != "" || task.Result != "Recorded answer" {
		t.Fatal(task, err)
	}
	root, _ := m.GetTask(work.RunID)
	if root.Checkpoint.BudgetedModelCalls != 0 || root.Status != TaskConversationInput || len(root.ConversationHold.InputIDs) != 2 {
		t.Fatal("reply altered work budget or silently incorporated input", root)
	}
	page, err := m.ConversationInbox(scope, c.ID, 0, 10)
	if err != nil || len(page.Items) != 2 || page.Items[0].State != "responded" || page.Items[0].RunID != id || page.Items[0].DeliveredAt == nil || page.Items[0].SettledAt == nil || page.Items[0].ResultReference == "" || page.Items[1].State != "accepted" {
		t.Fatal(page, err)
	}
	// Recorded identity wins over stale preparation on a delivery retry.
	retry, err := r.AdmitConversationTurn(scope, c.ID, first.ID, 1, "", AgentConfig{})
	if err != nil || retry.ID != id || retry.Status != TaskCompleted {
		t.Fatal("retry restarted a settled input", retry, err)
	}
	if CanDeleteTask(task) || m.DeleteTask(id) == nil {
		t.Fatal("legacy deletion orphaned canonical foreground history")
	}
	c, _ = m.Conversation(scope, c.ID)
	next, err := r.AdmitConversationTurn(scope, c.ID, second.ID, c.Revision, "model", taskConfig())
	if err != nil || next.ID == id {
		t.Fatal(next, err)
	}
	for _, name := range []string{"delegate_tasks", "worker_continue", "execution_run", "skill_load"} {
		if qualifiedTool(next, name, capabilities.Descriptor{}, false) {
			t.Fatal("foreground widened authority", name)
		}
	}
}

func TestConversationTurnRestartRetainsDeliveryWithoutDispatch(t *testing.T) {
	m, r, scope, c := controlFixture(t)
	input := acceptTestConversationInput(t, r, scope, c, "input")
	task, err := r.AdmitConversationTurn(scope, c.ID, input.ID, c.Revision, "model", taskConfig())
	if err != nil {
		t.Fatal(err)
	}
	loaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err = loaded.StorageError(); err != nil {
		t.Fatal(err)
	}
	restarted := NewRunner(loaded, NewToolRegistry(), nil)
	retry, err := restarted.AdmitConversationTurn(scope, c.ID, input.ID, c.Revision, "replacement-model", taskConfig())
	if err != nil || retry.ID != task.ID || retry.Model != "model" || retry.Status != TaskInterrupted || len(loaded.coordinationCandidates()) != 0 {
		t.Fatal("delivery retry revived interrupted turn", retry, err)
	}
	page, err := loaded.ConversationInbox(scope, c.ID, 0, 10)
	if err != nil || page.Items[0].RunStatus != TaskInterrupted || page.Items[0].State != "processing" || page.Items[0].SettledAt != nil {
		t.Fatal(page, err)
	}
	if _, err = restarted.AdmitConversationTurn(ControlScope{scope.Workspace, "bob"}, c.ID, input.ID, c.Revision, "model", taskConfig()); !errors.Is(err, ErrControlNotFound) {
		t.Fatal("foreign actor read delivery", err)
	}
}

func TestConversationTurnAdmissionAndReplyRollback(t *testing.T) {
	m, r, scope, c := controlFixture(t)
	input := acceptTestConversationInput(t, r, scope, c, "input")
	db, err := m.controlDatabase()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TRIGGER fail_delivery BEFORE INSERT ON agent_control_outbox WHEN NEW.run_id IS NOT NULL BEGIN SELECT RAISE(ABORT,'delivery fault'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err = r.AdmitConversationTurn(scope, c.ID, input.ID, c.Revision, "model", taskConfig()); err == nil {
		t.Fatal("fault ignored")
	}
	page, _ := m.ConversationInbox(scope, c.ID, 0, 10)
	if page.Items[0].State != "accepted" || page.Items[0].RunID != "" || len(m.tasks) != 0 {
		t.Fatal("failed admission published state", page, m.tasks)
	}
	if _, err = db.Exec(`DROP TRIGGER fail_delivery`); err != nil {
		t.Fatal(err)
	}
	task, err := r.AdmitConversationTurn(scope, c.ID, input.ID, c.Revision, "model", taskConfig())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`CREATE TRIGGER fail_reply BEFORE INSERT ON agent_conversation_messages WHEN NEW.message_id LIKE 'reply-%' BEGIN SELECT RAISE(ABORT,'reply fault'); END`); err != nil {
		t.Fatal(err)
	}
	finish := func(next *Task) error {
		next.Result = "Persistent result"
		finishTask(next, TaskCompleted, "")
		return nil
	}
	if _, err = m.updateTask(task.ID, finish); err == nil {
		t.Fatal("reply fault ignored")
	}
	current := m.tasks[task.ID]
	stored, err := readConversationInbox(db, scope, c.ID, input.ID)
	if err != nil || current.Status != TaskPending || stored.State != "processing" || stored.ResultReference != "" {
		t.Fatal("task and reply diverged", current, stored, err)
	}
	if _, err = db.Exec(`DROP TRIGGER fail_reply`); err != nil {
		t.Fatal(err)
	}
	// A persistence failure closes runner admission until storage recovery.
	// Restart reads the last committed pending turn and marks it interrupted.
	m = NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err = m.StorageError(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err = m.updateTask(task.ID, finish); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err = db.QueryRow(`SELECT count(*) FROM agent_conversation_messages WHERE message_id=?`, "reply-"+input.ID).Scan(&count); err != nil || count != 1 {
		t.Fatal("duplicate/lost reply", count, err)
	}
}
