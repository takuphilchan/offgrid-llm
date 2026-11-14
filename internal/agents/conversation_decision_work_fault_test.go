package agents

import (
	"errors"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestConversationDecisionNewWorkRollbackIncludesOldHoldAndOutbox(t *testing.T) {
	m, r, scope, c := controlFixture(t)
	old := admitControlTestWork(t, r, scope, c)
	c, _ = m.Conversation(scope, c.ID)
	input := acceptTestConversationInput(t, r, scope, c, "distinct-objective")
	r = finishNewWorkDecision(t, m, scope, c, input)
	c, _ = m.Conversation(scope, c.ID)
	db, err := m.controlDatabase()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TRIGGER reject_new_work BEFORE INSERT ON agent_control_events WHEN NEW.kind='input_work_admitted' BEGIN SELECT RAISE(ABORT,'work receipt fault'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err = r.AdmitConversationInputWork(scope, c.ID, input.ID, c.Revision, "model", taskConfig()); err == nil {
		t.Fatal("receipt failure ignored")
	}
	retained, _ := m.ConversationInput(scope, c.ID, input.ID)
	root, _ := m.GetTask(old.RunID)
	current, _ := m.Conversation(scope, c.ID)
	if retained.AppliedAt != nil || retained.CreatedWorkID != "" || !hasConversationHold(root) || current.Revision != c.Revision || current.FocusedWorkID != old.ID {
		t.Fatal("partially committed transition", retained, root, current)
	}
	var count int
	if err = db.QueryRow(`SELECT count(*) FROM agent_control_records WHERE workspace=? AND actor=? AND kind='work'`, scope.Workspace, scope.Actor).Scan(&count); err != nil || count != 1 {
		t.Fatal("orphan work record", count, err)
	}
	if err = db.QueryRow(`SELECT count(*) FROM agent_control_events WHERE workspace=? AND actor=? AND kind='work_accepted'`, scope.Workspace, scope.Actor).Scan(&count); err != nil || count != 1 {
		t.Fatal("orphan admission event/outbox", count, err)
	}
	if _, err = db.Exec(`DROP TRIGGER reject_new_work`); err != nil {
		t.Fatal(err)
	}
	// A stop committed before retry must survive release of this input's hold.
	old, _ = m.WorkItem(scope, old.ID)
	if _, err = r.ControlWork(scope, api.WorkCommand{RequestID: "stop", TargetID: old.ID, ExpectedRevision: old.Revision, Kind: "stop"}); err != nil {
		t.Fatal(err)
	}
	c, _ = m.Conversation(scope, c.ID)
	entry, err := r.AdmitConversationInputWork(scope, c.ID, input.ID, c.Revision, "model", taskConfig())
	if err != nil || entry.CreatedWorkID == "" {
		t.Fatal(entry, err)
	}
	root, _ = m.GetTask(old.RunID)
	if root.Status != TaskCancelled || root.SchedulerReady || hasConversationHold(root) {
		t.Fatal("new work revived stopped work", root)
	}
	if _, err = r.IncorporateConversationInput(scope, c.ID, input.ID, entry.RunID, "answered", 0); !errors.Is(err, ErrRunConflict) {
		t.Fatal("new work disposition replaced", err)
	}
}
