package agents

import (
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
)

// Only a trusted service resolver supplies observed host metadata. This is not
// accepted by ApplyConversationCommand or exposed as a client-shaped payload.
func (m *Manager) ApplyProjectFolder(scope ControlScope, c ConversationCommand, observed execution.ProjectContextSnapshot) (*ConversationCommandReceipt, error) {
	if c.Kind != "use_project_folder" || validateConversationCommand(c) != nil {
		return nil, ErrControlCommand
	}
	parsed, err := parseProjectSnapshot(observed)
	if err != nil {
		return nil, err
	}
	return m.withControlCommand(scope, c.RequestID, commandDigest(c), func(tx *sql.Tx) (*ConversationCommandReceipt, error) {
		record, err := readControlRecord(tx, scope, c.TargetID, "conversation")
		if err != nil {
			return nil, err
		}
		if record.Revision != c.ExpectedRevision {
			return nil, ErrRunConflict
		}
		var conversation Conversation
		if err = json.Unmarshal(record.Snapshot, &conversation); err != nil {
			return nil, err
		}
		folder := c.Payload.Folder
		ref := execution.Reference{Host: folder.Host, Session: folder.Session, Grant: folder.Grant, Revision: folder.Revision}
		return writeProjectFolder(tx, scope, &conversation, ref, observed, parsed, c.Kind, c.RequestID)
	})
}

// A persisted reference is not live authority. Every service use must resolve
// it against the current approved host registry; restart never restores grants.
func (m *Manager) ConversationFolder(scope ControlScope, id string) (*execution.Reference, error) {
	db, err := m.controlDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	record, err := readControlRecord(db, scope, id, "conversation")
	if err != nil {
		return nil, err
	}
	var conversation Conversation
	if err = json.Unmarshal(record.Snapshot, &conversation); err != nil {
		return nil, err
	}
	var data []byte
	err = db.QueryRow(`SELECT reference FROM agent_conversation_folders WHERE workspace=? AND actor=? AND conversation_id=? AND project_id=?`, scope.Workspace, scope.Actor, id, conversation.ProjectID).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var ref execution.Reference
	if json.Unmarshal(data, &ref) != nil || ref.Validate() != nil {
		return nil, ErrRunStorage
	}
	return &ref, nil
}
