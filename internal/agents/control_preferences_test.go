package agents

import (
	"encoding/json"
	"testing"
)

func TestConversationPreferencePrecedenceAndRestrictionPersistence(t *testing.T) {
	no := false
	conversation := ConversationPreferences{Model: "selected", Restrictions: ResourceRestrictions{Computer: &no}}
	profile := &AgentProfile{Settings: ProfileSettings{Model: "profile"}}
	project := ConversationPreferences{Model: "project"}
	user := ConversationPreferences{Model: "user"}
	service := ConversationPreferences{Model: "service"}
	for _, tc := range []struct{ request, conversationModel, want, source string }{{"override", "selected", "override", "request"}, {"", "selected", "selected", "conversation"}, {"", "", "profile", "profile"}} {
		conversation.Model = tc.conversationModel
		e := ResolveConversationPreferences(tc.request, conversation, profile, project, user, service)
		if e.Model != tc.want || e.Provenance["model"] != tc.source || e.Restrictions.Computer == nil || *e.Restrictions.Computer {
			t.Fatal("incorrect precedence", e)
		}
	}
	m, _, scope, c := controlFixture(t)
	configure := ConversationCommand{RequestID: "restricted", TargetID: c.ID, ExpectedRevision: 1, Kind: "configure_conversation", Payload: ConversationCommandPayload{Preferences: &conversation}}
	if _, err := m.ApplyConversationCommand(scope, configure); err != nil {
		t.Fatal(err)
	}
	configure.RequestID = "change-model"
	configure.ExpectedRevision = 2
	configure.Payload.Preferences = &ConversationPreferences{Model: "another"}
	if _, err := m.ApplyConversationCommand(scope, configure); err != nil {
		t.Fatal(err)
	}
	reloaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	saved, err := reloaded.Conversation(scope, c.ID)
	if err != nil || saved.Preferences.Restrictions.Computer == nil || *saved.Preferences.Restrictions.Computer {
		t.Fatal("restriction lost on model edit/reload", saved, err)
	}
}

func TestControlExpiredCursorReturnsSnapshotAndRetainsAcceptedInput(t *testing.T) {
	m, r, scope, c := controlFixture(t)
	command := ConversationCommand{RequestID: "retained-input", TargetID: c.ID, ExpectedRevision: 1, Kind: "create_work", Payload: ConversationCommandPayload{Input: &ConversationInput{Content: "Keep the accepted instruction"}}}
	receipt, err := r.AdmitConversationWork(scope, command, "model", taskConfig())
	if err != nil {
		t.Fatal(err)
	}
	if err = m.RetainControlEvents(scope, c.ID, "conversation", receipt.EventCursor); err != nil {
		t.Fatal(err)
	}
	page, err := m.ControlEvents(scope, c.ID, "conversation", 0, 100)
	if err != nil || !page.HistoryGap || len(page.Events) != 0 || page.NextCursor != receipt.EventCursor {
		t.Fatal("missing explicit gap", page, err)
	}
	var snapshot Conversation
	if err = json.Unmarshal(page.Snapshot, &snapshot); err != nil || snapshot.Revision != 2 || snapshot.FocusedWorkID != receipt.Work.ID {
		t.Fatal("wrong gap snapshot", snapshot, err)
	}
	db, err := m.controlDatabase()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var count int
	if err = db.QueryRow(`SELECT COUNT(*) FROM agent_conversation_messages`).Scan(&count); err != nil || count != 1 {
		t.Fatal("event expiration erased history", count, err)
	}
	workPage, err := m.ControlEvents(scope, receipt.Work.ID, "work", 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if err = m.RetainControlEvents(scope, receipt.Work.ID, "work", workPage.NextCursor); err != ErrRunConflict {
		t.Fatal("pruned pending dispatch", err)
	}
}
