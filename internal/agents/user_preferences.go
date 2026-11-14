package agents

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

const UserPreferencesID = "user-defaults"

func readUserPreferences(q controlReader, scope ControlScope) (*api.UserPreferences, error) {
	if !validProfileOwner(scope.Workspace, scope.Actor) {
		return nil, ErrControlNotFound
	}
	r, err := readControlRecord(q, scope, UserPreferencesID, "user_preferences")
	if errors.Is(err, ErrControlNotFound) {
		// Reading default preferences must not write a record or emit an event.
		return &api.UserPreferences{ID: UserPreferencesID}, nil
	}
	if err != nil {
		return nil, err
	}
	var preferences api.UserPreferences
	err = json.Unmarshal(r.Snapshot, &preferences)
	return &preferences, err
}

func (m *Manager) UserPreferences(scope ControlScope) (*api.UserPreferences, error) {
	db, err := m.controlDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return readUserPreferences(db, scope)
}

func (m *Manager) ConfigureUserPreferences(scope ControlScope, command api.UserPreferencesCommand) (*ConversationCommandReceipt, error) {
	if command.TargetID != UserPreferencesID || command.Kind != "configure_user_preferences" || command.ExpectedRevision < 0 || !validProfileText(command.Payload.Model, 256, false) {
		return nil, ErrControlCommand
	}
	data, _ := json.Marshal(command)
	digest := sha256.Sum256(append([]byte("user-preferences-v1\x00"), data...))
	return m.withControlCommand(scope, command.RequestID, hex.EncodeToString(digest[:]), func(tx *sql.Tx) (*ConversationCommandReceipt, error) {
		current, err := readUserPreferences(tx, scope)
		if err != nil {
			return nil, err
		}
		if current.Revision != command.ExpectedRevision {
			return nil, ErrRunConflict
		}
		current.Revision++
		current.Preferences = command.Payload
		if err = putControlRecord(tx, scope, current.ID, "user_preferences", current.Revision, current); err != nil {
			return nil, err
		}
		cursor, err := appendControlEvent(tx, scope, current.ID, current.Revision, command.Kind, current, "")
		if err != nil {
			return nil, err
		}
		return &ConversationCommandReceipt{RequestID: command.RequestID, TargetID: current.ID, Revision: current.Revision, EventCursor: cursor, UserPreferences: current}, nil
	})
}
