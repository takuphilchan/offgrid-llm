package agents

import (
	"encoding/json"
)

func (m *Manager) ConversationRuns(scope ControlScope, id string) ([]string, error) {
	db, err := m.controlDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	if _, err = readControlRecord(db, scope, id, "conversation"); err != nil {
		return nil, err
	}
	rows, err := db.Query(`SELECT r.snapshot FROM agent_control_links l JOIN agent_control_records r ON r.workspace=l.workspace AND r.actor=l.actor AND r.id=l.work_id AND r.kind='work' WHERE l.workspace=? AND l.actor=? AND l.conversation_id=? ORDER BY l.work_id`, scope.Workspace, scope.Actor, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var data []byte
		if err = rows.Scan(&data); err != nil {
			return nil, err
		}
		var work WorkItem
		if err = json.Unmarshal(data, &work); err != nil {
			return nil, err
		}
		if len(work.RunIDs) > 0 {
			ids = append(ids, work.RunIDs...)
		} else {
			ids = append(ids, work.RunID)
		}
	}
	return ids, rows.Err()
}

type ControlRecordPage struct {
	Items      []json.RawMessage `json:"items"`
	NextCursor string            `json:"next_cursor,omitempty"`
	HasMore    bool              `json:"has_more"`
}

func (m *Manager) ControlRecords(scope ControlScope, kind, after string, limit int) (*ControlRecordPage, error) {
	if !validProfileOwner(scope.Workspace, scope.Actor) {
		return nil, ErrControlNotFound
	}
	if limit < 1 || limit > 200 || len(after) > 128 {
		return nil, ErrControlCommand
	}
	switch kind {
	case "project", "conversation", "work", "worker", "result", "change_set":
	default:
		return nil, ErrControlCommand
	}
	db, err := m.controlDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(`SELECT id,snapshot FROM agent_control_records WHERE workspace=? AND actor=? AND kind=? AND id>? AND NOT EXISTS (SELECT 1 FROM agent_conversation_deletions d WHERE d.workspace=agent_control_records.workspace AND d.actor=agent_control_records.actor AND d.conversation_id=agent_control_records.id) ORDER BY id LIMIT ?`, scope.Workspace, scope.Actor, kind, after, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	page := &ControlRecordPage{Items: []json.RawMessage{}, NextCursor: after}
	for rows.Next() {
		if len(page.Items) == limit {
			page.HasMore = true
			break
		}
		var id string
		var data []byte
		if err = rows.Scan(&id, &data); err != nil {
			return nil, err
		}
		page.Items = append(page.Items, json.RawMessage(data))
		page.NextCursor = id
	}
	return page, rows.Err()
}

// RetainControlEvents is service maintenance, never a client command. History
// and current state remain authoritative after observation events expire.
func (m *Manager) RetainControlEvents(scope ControlScope, id, kind string, through int64) error {
	if through < 0 {
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
	if _, err = tx.Exec(`UPDATE agent_schema SET version=version WHERE singleton=1`); err != nil {
		return err
	}
	r, err := readControlRecord(tx, scope, id, kind)
	if err != nil {
		return err
	}
	var max int64
	if err = tx.QueryRow(`SELECT COALESCE(MAX(sequence),?) FROM agent_control_events WHERE workspace=? AND actor=? AND target_id=?`, r.Floor, scope.Workspace, scope.Actor, id).Scan(&max); err != nil {
		return err
	}
	if through > max {
		return ErrControlCommand
	}
	if through <= r.Floor {
		return nil
	}
	var pending int
	if err = tx.QueryRow(`SELECT COUNT(*) FROM agent_control_outbox o JOIN agent_control_events e ON e.sequence=o.event_id WHERE e.workspace=? AND e.actor=? AND e.target_id=? AND e.sequence<=? AND o.run_id IS NOT NULL AND o.disposition='pending'`, scope.Workspace, scope.Actor, id, through).Scan(&pending); err != nil {
		return err
	}
	if pending > 0 {
		return ErrRunConflict
	}
	if _, err = tx.Exec(`DELETE FROM agent_control_outbox WHERE event_id IN (SELECT sequence FROM agent_control_events WHERE workspace=? AND actor=? AND target_id=? AND sequence<=?)`, scope.Workspace, scope.Actor, id, through); err != nil {
		return err
	}
	if _, err = tx.Exec(`DELETE FROM agent_control_events WHERE workspace=? AND actor=? AND target_id=? AND sequence<=?`, scope.Workspace, scope.Actor, id, through); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE agent_control_records SET event_floor=? WHERE workspace=? AND actor=? AND id=?`, through, scope.Workspace, scope.Actor, id); err != nil {
		return err
	}
	return tx.Commit()
}
