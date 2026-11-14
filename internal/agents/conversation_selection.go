package agents

import (
	"database/sql"
	"encoding/json"
	"errors"
)

// Selection never changes a run, transfers a grant, or renews a budget. The
// service checks source access before accepting links and again on retrieval.
func applyConversationSelection(tx *sql.Tx, scope ControlScope, c ConversationCommand, conversation *Conversation) error {
	if c.Kind == "clear_project_folder" {
		conversation.ContextRevision++
		_, err := tx.Exec(`DELETE FROM agent_conversation_folders WHERE workspace=? AND actor=? AND conversation_id=?`, scope.Workspace, scope.Actor, conversation.ID)
		return err
	}
	if c.Kind == "select_project" {
		if _, err := readControlRecord(tx, scope, c.Payload.ProjectID, "project"); err != nil {
			return err
		}
		conversation.ProjectID = c.Payload.ProjectID
		conversation.ContextRevision++
		conversation.FocusedWorkID = ""
		_, err := tx.Exec(`DELETE FROM agent_conversation_folders WHERE workspace=? AND actor=? AND conversation_id=?`, scope.Workspace, scope.Actor, conversation.ID)
		return err
	}
	if c.Kind != "link_work" && c.Kind != "focus_work" {
		return nil
	}
	if c.Payload.WorkID == "" {
		conversation.FocusedWorkID = ""
		return nil
	}
	record, err := readControlRecord(tx, scope, c.Payload.WorkID, "work")
	if err != nil {
		return err
	}
	var work WorkItem
	if err = json.Unmarshal(record.Snapshot, &work); err != nil {
		return err
	}
	var data []byte
	if err = tx.QueryRow(`SELECT snapshot FROM agent_tasks WHERE id=? AND actor=?`, work.RunID, scope.Actor).Scan(&data); errors.Is(err, sql.ErrNoRows) {
		return ErrControlNotFound
	} else if err != nil {
		return err
	}
	var run Task
	if json.Unmarshal(data, &run) != nil {
		return ErrRunStorage
	}
	if run.DeletedAt != nil || run.WorkID != work.ID || run.ParentID != "" || run.Origin == nil || run.Origin.Workspace != scope.Workspace {
		return ErrControlNotFound
	}
	if c.Kind == "link_work" {
		if _, err = tx.Exec(`INSERT INTO agent_control_links(workspace,actor,conversation_id,work_id) VALUES(?,?,?,?) ON CONFLICT DO NOTHING`, scope.Workspace, scope.Actor, conversation.ID, work.ID); err != nil {
			return err
		}
	} else {
		var found int
		if err = tx.QueryRow(`SELECT 1 FROM agent_control_links WHERE workspace=? AND actor=? AND conversation_id=? AND work_id=?`, scope.Workspace, scope.Actor, conversation.ID, work.ID).Scan(&found); errors.Is(err, sql.ErrNoRows) {
			return ErrControlNotFound
		} else if err != nil {
			return err
		}
	}
	conversation.FocusedWorkID = work.ID
	return nil
}

// ConversationContainsRun validates a durable link, not caller-supplied history.
// Source permission checks remain the service's responsibility at retrieval.
func (m *Manager) ConversationContainsRun(scope ControlScope, conversation, workID, runID string) (bool, error) {
	db, err := m.controlDatabase()
	if err != nil {
		return false, err
	}
	defer db.Close()
	if !validProfileOwner(scope.Workspace, scope.Actor) {
		return false, ErrControlNotFound
	}
	var raw []byte
	err = db.QueryRow(`SELECT w.snapshot FROM agent_control_links l JOIN agent_control_records c ON c.workspace=l.workspace AND c.actor=l.actor AND c.id=l.conversation_id AND c.kind='conversation' JOIN agent_control_records w ON w.workspace=l.workspace AND w.actor=l.actor AND w.id=l.work_id AND w.kind='work' WHERE l.workspace=? AND l.actor=? AND l.conversation_id=? AND l.work_id=?`, scope.Workspace, scope.Actor, conversation, workID).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var work WorkItem
	if err = json.Unmarshal(raw, &work); err != nil {
		return false, err
	}
	return work.RunID == runID, nil
}
