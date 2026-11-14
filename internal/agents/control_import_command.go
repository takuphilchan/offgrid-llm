package agents

import (
	"database/sql"
	"encoding/json"
)

// The trusted session reader holds its mutation lock through this transaction.
// Ownership, manifest, imported input, event and command receipt commit together.
func (m *Manager) ImportConversationCommand(scope ControlScope, c ConversationCommand, input ConversationImport) (*ConversationCommandReceipt, error) {
	if c.Kind != "import_conversation" || validateConversationCommand(c) != nil || c.Payload.LegacyName != input.LegacyName {
		return nil, ErrControlCommand
	}
	return m.withControlCommand(scope, c.RequestID, commandDigest(c), func(tx *sql.Tx) (*ConversationCommandReceipt, error) {
		manifest, err := importConversationTransaction(tx, scope, input)
		if err != nil {
			return nil, err
		}
		record, err := readControlRecord(tx, scope, manifest.ConversationID, "conversation")
		if err != nil {
			return nil, err
		}
		var conversation Conversation
		if err = json.Unmarshal(record.Snapshot, &conversation); err != nil {
			return nil, err
		}
		var cursor int64
		if err = tx.QueryRow(`SELECT COALESCE(MAX(sequence),0) FROM agent_control_events WHERE workspace=? AND actor=? AND target_id=?`, scope.Workspace, scope.Actor, conversation.ID).Scan(&cursor); err != nil {
			return nil, err
		}
		return &ConversationCommandReceipt{RequestID: c.RequestID, TargetID: conversation.ID, Revision: conversation.Revision, EventCursor: cursor, Conversation: &conversation, Import: manifest}, nil
	})
}
