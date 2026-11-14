package agents

import (
	"errors"
	"sync"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestUserPreferencesDurabilityIsolationAndConcurrentRetries(t *testing.T) {
	m, _, scope, _ := controlFixture(t)
	initial, err := m.UserPreferences(scope)
	if err != nil || initial.Revision != 0 || initial.ID != UserPreferencesID {
		t.Fatal(initial, err)
	}
	db, err := m.controlDatabase()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var count int
	if err = db.QueryRow(`SELECT count(*) FROM agent_control_records WHERE kind='user_preferences'`).Scan(&count); err != nil || count != 0 {
		t.Fatal("GET wrote defaults", count, err)
	}
	no := false
	command := api.UserPreferencesCommand{RequestID: "defaults", TargetID: initial.ID, Kind: "configure_user_preferences", Payload: api.UserDefaultValues{Model: "chosen", Delegation: &no}}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			receipt, err := m.ConfigureUserPreferences(scope, command)
			if err != nil || receipt.Revision != 1 {
				t.Error(receipt, err)
			}
		}()
	}
	wg.Wait()
	changed := command
	changed.Payload.Model = "conflicting"
	if _, err = m.ConfigureUserPreferences(scope, changed); !errors.Is(err, ErrRunConflict) {
		t.Fatal("request reuse accepted", err)
	}
	changed.RequestID = "stale-revision"
	if _, err = m.ConfigureUserPreferences(scope, changed); !errors.Is(err, ErrRunConflict) {
		t.Fatal("stale revision accepted", err)
	}
	for _, foreign := range []ControlScope{{scope.Workspace, "bob"}, {"other", scope.Actor}} {
		value, err := m.UserPreferences(foreign)
		if err != nil || value.Revision != 0 || value.Preferences.Model != "" {
			t.Fatal("foreign defaults leaked", value, err)
		}
	}
	loaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	saved, err := loaded.UserPreferences(scope)
	if err != nil || saved.Revision != 1 || saved.Preferences.Model != "chosen" || saved.Preferences.Delegation == nil || *saved.Preferences.Delegation {
		t.Fatal(saved, err)
	}
	page, err := loaded.ControlEvents(scope, UserPreferencesID, "user_preferences", 0, 100)
	if err != nil || len(page.Events) != 1 {
		t.Fatal(page, err)
	}
	changed.ExpectedRevision, changed.Payload = 1, api.UserDefaultValues{}
	if _, err = loaded.ConfigureUserPreferences(scope, changed); err != nil {
		t.Fatal(err)
	}
	saved, err = loaded.UserPreferences(scope)
	if err != nil || saved.Revision != 2 || saved.Preferences.Model != "" || saved.Preferences.Delegation != nil {
		t.Fatal("reset did not inherit defaults", saved, err)
	}
}

func TestUserPreferencesReceiptFailureRollsBackState(t *testing.T) {
	m, _, scope, _ := controlFixture(t)
	db, err := m.controlDatabase()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TRIGGER reject_preferences BEFORE INSERT ON agent_control_commands WHEN NEW.request_id='preferences-fault' BEGIN SELECT RAISE(ABORT,'injected'); END`); err != nil {
		t.Fatal(err)
	}
	command := api.UserPreferencesCommand{RequestID: "preferences-fault", TargetID: UserPreferencesID, Kind: "configure_user_preferences", Payload: api.UserDefaultValues{Model: "not-committed"}}
	if _, err = m.ConfigureUserPreferences(scope, command); err == nil {
		t.Fatal("ignored receipt failure")
	}
	state, err := m.UserPreferences(scope)
	if err != nil || state.Revision != 0 {
		t.Fatal("partial defaults published", state, err)
	}
	var count int
	if err = db.QueryRow(`SELECT count(*) FROM agent_control_events WHERE target_id=?`, UserPreferencesID).Scan(&count); err != nil || count != 0 {
		t.Fatal("partial event published", count, err)
	}
}

func TestUserPreferenceChangeDuringAdmissionRejectsStaleConfiguration(t *testing.T) {
	m, r, scope, c := controlFixture(t)
	config := taskConfig()
	config.EffectivePreferences = &api.EffectiveConversationPreferences{UserPreferencesRevision: 0}
	if _, err := m.ConfigureUserPreferences(scope, api.UserPreferencesCommand{RequestID: "changed", TargetID: UserPreferencesID, Kind: "configure_user_preferences", Payload: api.UserDefaultValues{Model: "new"}}); err != nil {
		t.Fatal(err)
	}
	command := ConversationCommand{RequestID: "stale-defaults", TargetID: c.ID, ExpectedRevision: 1, Kind: "create_work", Payload: ConversationCommandPayload{Input: &ConversationInput{Content: "Prepared with older defaults"}}}
	if _, err := r.AdmitConversationWork(scope, command, "model", config); !errors.Is(err, ErrRunConflict) {
		t.Fatal("stale preferences admitted", err)
	}
	saved, err := m.Conversation(scope, c.ID)
	if err != nil || saved.Revision != 1 || saved.FocusedWorkID != "" {
		t.Fatal("partial work admitted", saved, err)
	}
}
