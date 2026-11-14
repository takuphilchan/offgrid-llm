package agents

import (
	"database/sql"
	"encoding/json"
	"strings"
)

// All resource reducers share the same actor-scoped dedupe and commit boundary.
// apply may persist state/events/outbox, but must never invoke external effects.
func (m *Manager) withControlCommand(scope ControlScope, requestID, digest string, apply func(*sql.Tx) (*ConversationCommandReceipt, error)) (*ConversationCommandReceipt, error) {
	db, err := m.controlDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return withControlCommandDatabase(db, scope, requestID, digest, apply)
}

// The runner can open the database before holding the in-memory task lock, then
// keep that lock through commit/publication without reentering StorageError.
func withControlCommandDatabase(db *sql.DB, scope ControlScope, requestID, digest string, apply func(*sql.Tx) (*ConversationCommandReceipt, error)) (*ConversationCommandReceipt, error) {
	if !validProfileOwner(scope.Workspace, scope.Actor) {
		return nil, ErrControlNotFound
	}
	if !validProfileText(requestID, 128, true) || strings.ContainsAny(requestID, "\r\n") || digest == "" {
		return nil, ErrControlCommand
	}
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`UPDATE agent_schema SET version=version WHERE singleton=1`); err != nil {
		return nil, err
	}
	previous, err := recordedControlReceipt(tx, scope, requestID, digest)
	if err != nil || previous != nil {
		return previous, err
	}
	receipt, err := apply(tx)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(receipt)
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(`INSERT INTO agent_control_commands(workspace,actor,request_id,digest,receipt) VALUES(?,?,?,?,?)`, scope.Workspace, scope.Actor, requestID, digest, data); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return receipt, nil
}
