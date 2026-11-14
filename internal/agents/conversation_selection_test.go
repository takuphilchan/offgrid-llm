package agents

import (
	"errors"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestConversationSelectionRetainsRunIdentityBudgetAndRestrictions(t *testing.T) {
	m, r, scope, first := controlFixture(t)
	work := admitControlTestWork(t, r, scope, first)
	before, _ := m.GetTask(work.RunID)
	no := false
	created, err := m.ApplyConversationCommand(scope, ConversationCommand{RequestID: "second", Kind: "create_conversation", Payload: ConversationCommandPayload{Preferences: &ConversationPreferences{Restrictions: ResourceRestrictions{Computer: &no}}}})
	if err != nil {
		t.Fatal(err)
	}
	command := ConversationCommand{RequestID: "link", TargetID: created.TargetID, ExpectedRevision: 1, Kind: "focus_work", Payload: ConversationCommandPayload{WorkID: work.ID}}
	if _, err = m.ApplyConversationCommand(scope, command); !errors.Is(err, ErrControlNotFound) {
		t.Fatal("focused unlinked work", err)
	}
	command.Kind = "link_work"
	accepted, err := m.ApplyConversationCommand(scope, command)
	if err != nil || accepted.Conversation.FocusedWorkID != work.ID {
		t.Fatal(accepted, err)
	}
	if _, err = m.ApplyConversationCommand(scope, command); err != nil {
		t.Fatal(err)
	}
	runs, err := m.ConversationRuns(scope, created.TargetID)
	if err != nil || len(runs) != 1 || runs[0] != before.ID {
		t.Fatal("link copied execution", runs, err)
	}
	command.RequestID, command.ExpectedRevision, command.Kind, command.Payload.WorkID = "clear", 2, "focus_work", ""
	if _, err = m.ApplyConversationCommand(scope, command); err != nil {
		t.Fatal(err)
	}
	command.RequestID, command.ExpectedRevision, command.Payload.WorkID = "refocus", 3, work.ID
	if _, err = m.ApplyConversationCommand(scope, command); err != nil {
		t.Fatal(err)
	}
	project, err := m.ApplyProjectCommand(scope, api.ProjectCommand{RequestID: "project", Kind: "create_project", Payload: api.ProjectCommandPayload{Name: "Next project"}})
	if err != nil {
		t.Fatal(err)
	}
	command.RequestID, command.ExpectedRevision, command.Kind = "select-project", 4, "select_project"
	command.Payload = ConversationCommandPayload{ProjectID: project.TargetID}
	if _, err = m.ApplyConversationCommand(scope, command); err != nil {
		t.Fatal(err)
	}
	after, _ := m.GetTask(before.ID)
	if after.WorkID != before.WorkID || after.Origin.Conversation != before.Origin.Conversation || after.Config.MaxIterations != before.Config.MaxIterations || after.Checkpoint.BudgetedModelCalls != before.Checkpoint.BudgetedModelCalls || after.ControlEpoch != before.ControlEpoch {
		t.Fatal("selection modified run or budget")
	}
	loaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	saved, err := loaded.Conversation(scope, created.TargetID)
	if err != nil || saved.Revision != 5 || saved.ProjectID != project.TargetID || saved.FocusedWorkID != "" || saved.Preferences.Restrictions.Computer == nil || *saved.Preferences.Restrictions.Computer {
		t.Fatal(saved, err)
	}
	linked, err := loaded.ConversationContainsRun(scope, saved.ID, work.ID, before.ID)
	if err != nil || !linked {
		t.Fatal("project switch erased linked work", linked, err)
	}
}

func TestConversationLinkScopesAndReceiptRollback(t *testing.T) {
	m, r, scope, first := controlFixture(t)
	work := admitControlTestWork(t, r, scope, first)
	created, err := m.ApplyConversationCommand(scope, ConversationCommand{RequestID: "second", Kind: "create_conversation"})
	if err != nil {
		t.Fatal(err)
	}
	command := ConversationCommand{RequestID: "link-fault", TargetID: created.TargetID, ExpectedRevision: 1, Kind: "link_work", Payload: ConversationCommandPayload{WorkID: work.ID}}
	for _, foreign := range []ControlScope{{scope.Workspace, "other"}, {"other", scope.Actor}} {
		owned, err := m.ApplyConversationCommand(foreign, ConversationCommand{RequestID: "conversation", Kind: "create_conversation"})
		if err != nil {
			t.Fatal(err)
		}
		probe := command
		probe.TargetID = owned.TargetID
		if _, err = m.ApplyConversationCommand(foreign, probe); !errors.Is(err, ErrControlNotFound) {
			t.Fatal("foreign link admitted", err)
		}
	}
	db, err := m.controlDatabase()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TRIGGER reject_link BEFORE INSERT ON agent_control_commands WHEN NEW.request_id='link-fault' BEGIN SELECT RAISE(ABORT,'injected'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err = m.ApplyConversationCommand(scope, command); err == nil {
		t.Fatal("ignored receipt fault")
	}
	linked, err := m.ConversationContainsRun(scope, created.TargetID, work.ID, work.RunID)
	if err != nil || linked {
		t.Fatal("partial link committed", linked, err)
	}
	saved, err := m.Conversation(scope, created.TargetID)
	if err != nil || saved.Revision != 1 || saved.FocusedWorkID != "" {
		t.Fatal(saved, err)
	}
}
