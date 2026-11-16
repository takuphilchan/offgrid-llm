package agents

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

var ErrConversationHasWork = errors.New("conversation has work requiring an explicit retention choice")
var ErrConversationInputPending = errors.New("conversation has unprocessed accepted input")

// DeleteImportedConversation fences future commands and purges conversation
// bodies in one write transaction. Work records remain independently available.
// Its tombstone makes retries safe and prevents retained import evidence from
// recreating a conversation after a crash during filesystem cleanup.
func (m *Manager) DeleteImportedConversation(scope ControlScope, name, id string, revision int64, retain bool) error {
	if !validProfileOwner(scope.Workspace, scope.Actor) || revision < 1 {
		return ErrControlCommand
	}
	db, err := m.controlDatabase()
	if err != nil {
		return err
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("UPDATE agent_schema SET version=version WHERE singleton=1"); err != nil {
		return err
	}
	manifest, err := readConversationImport(tx, scope, name)
	if err != nil {
		return err
	}
	if manifest == nil || manifest.ConversationID != id {
		return ErrControlNotFound
	}
	var deleted int
	if err = tx.QueryRow("SELECT count(*) FROM agent_conversation_deletions WHERE workspace=? AND actor=? AND conversation_id=?", scope.Workspace, scope.Actor, id).Scan(&deleted); err != nil {
		return err
	}
	if deleted != 0 {
		return tx.Commit()
	}
	record, err := readControlRecord(tx, scope, id, "conversation")
	if err != nil {
		return err
	}
	if record.Revision != revision {
		return ErrRunConflict
	}
	var pending int
	if err = tx.QueryRow("SELECT count(*) FROM agent_conversation_inbox WHERE workspace=? AND actor=? AND conversation_id=? AND (state IN ('accepted','processing','stopping') OR (state='responded' AND json_extract(payload,'$.decision.kind') IS NOT NULL AND json_extract(payload,'$.applied_at') IS NULL))", scope.Workspace, scope.Actor, id).Scan(&pending); err != nil {
		return err
	}
	if pending != 0 {
		return ErrConversationInputPending
	}
	rows, err := tx.Query("SELECT r.snapshot FROM agent_control_links l JOIN agent_control_records r ON r.workspace=l.workspace AND r.actor=l.actor AND r.id=l.work_id WHERE l.workspace=? AND l.actor=? AND l.conversation_id=?", scope.Workspace, scope.Actor, id)
	if err != nil {
		return err
	}
	ids := append([]string{}, manifest.LegacyRunIDs...)
	for rows.Next() {
		var data []byte
		if err = rows.Scan(&data); err != nil {
			rows.Close()
			return err
		}
		var work WorkItem
		if err = json.Unmarshal(data, &work); err != nil {
			rows.Close()
			return err
		}
		if len(work.RunIDs) > 0 {
			ids = append(ids, work.RunIDs...)
		} else {
			ids = append(ids, work.RunID)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, run := range ids {
		var data []byte
		err = tx.QueryRow("SELECT snapshot FROM agent_tasks WHERE id=? AND actor=?", run, scope.Actor).Scan(&data)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return err
		}
		var task Task
		if err = json.Unmarshal(data, &task); err != nil {
			return err
		}
		// A retained task must not lose the inbox entry needed to release its
		// hold, including a completed side answer awaiting incorporation.
		if hasConversationHold(&task) {
			return ErrConversationInputPending
		}
		if !retain && task.Status != TaskCompleted && task.Status != TaskFailed && task.Status != TaskCancelled {
			return ErrConversationHasWork
		}
	}
	if _, err = tx.Exec("INSERT INTO agent_conversation_deletions(workspace,actor,conversation_id,deleted_at) VALUES(?,?,?,?)", scope.Workspace, scope.Actor, id, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return err
	}
	for _, table := range []string{"agent_conversation_messages", "agent_conversation_inbox", "agent_conversation_folders", "agent_control_links"} {
		if _, err = tx.Exec("DELETE FROM "+table+" WHERE workspace=? AND actor=? AND conversation_id=?", scope.Workspace, scope.Actor, id); err != nil {
			return err
		}
	}
	if _, err = tx.Exec("DELETE FROM agent_control_outbox WHERE event_id IN (SELECT sequence FROM agent_control_events WHERE workspace=? AND actor=? AND target_id=?)", scope.Workspace, scope.Actor, id); err != nil {
		return err
	}
	if _, err = tx.Exec("DELETE FROM agent_control_events WHERE workspace=? AND actor=? AND target_id=?", scope.Workspace, scope.Actor, id); err != nil {
		return err
	}
	if _, err = tx.Exec("DELETE FROM agent_control_commands WHERE workspace=? AND actor=? AND json_extract(receipt,'$.target_id')=?", scope.Workspace, scope.Actor, id); err != nil {
		return err
	}
	if _, err = tx.Exec("UPDATE agent_conversation_imports SET source=X'7B7D' WHERE workspace=? AND actor=? AND conversation_id=?", scope.Workspace, scope.Actor, id); err != nil {
		return err
	}
	if _, err = tx.Exec("UPDATE agent_control_records SET snapshot=X'7B7D',revision=revision+1 WHERE workspace=? AND actor=? AND id=? AND kind='conversation'", scope.Workspace, scope.Actor, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (m *Manager) ConversationDeleted(scope ControlScope, id string) (bool, error) {
	if !validProfileOwner(scope.Workspace, scope.Actor) {
		return false, ErrControlNotFound
	}
	db, err := m.controlDatabase()
	if err != nil {
		return false, err
	}
	defer db.Close()
	var n int
	err = db.QueryRow("SELECT count(*) FROM agent_conversation_deletions WHERE workspace=? AND actor=? AND conversation_id=?", scope.Workspace, scope.Actor, id).Scan(&n)
	return n > 0, err
}
