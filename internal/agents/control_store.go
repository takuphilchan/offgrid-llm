package agents

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// This schema shares the task database and transaction boundary. Records are
// current state; events are audit/observation, not a replay-based state machine.
const controlRecordColumns = `(
workspace TEXT NOT NULL, actor TEXT NOT NULL, id TEXT NOT NULL,
kind TEXT NOT NULL CHECK(kind IN ('project','conversation','work','worker','result','change_set','user_preferences')),
revision INTEGER NOT NULL CHECK(revision>0), event_floor INTEGER NOT NULL DEFAULT 0,
snapshot BLOB NOT NULL, PRIMARY KEY(workspace,actor,id))`

const controlSchema = `CREATE TABLE IF NOT EXISTS agent_conversation_deletions (workspace TEXT NOT NULL, actor TEXT NOT NULL, conversation_id TEXT NOT NULL, deleted_at TEXT NOT NULL, PRIMARY KEY(workspace,actor,conversation_id));
CREATE TABLE IF NOT EXISTS agent_control_records ` + controlRecordColumns + `;
CREATE TABLE IF NOT EXISTS agent_conversation_folders (
workspace TEXT NOT NULL, actor TEXT NOT NULL, conversation_id TEXT NOT NULL,
project_id TEXT NOT NULL, reference BLOB NOT NULL,
PRIMARY KEY(workspace,actor,conversation_id),
FOREIGN KEY(workspace,actor,conversation_id) REFERENCES agent_control_records(workspace,actor,id),
FOREIGN KEY(workspace,actor,project_id) REFERENCES agent_control_records(workspace,actor,id));
CREATE TABLE IF NOT EXISTS agent_project_revisions (
workspace TEXT NOT NULL, actor TEXT NOT NULL, project_id TEXT NOT NULL, revision INTEGER NOT NULL,
snapshot BLOB NOT NULL, PRIMARY KEY(workspace,actor,project_id,revision),
FOREIGN KEY(workspace,actor,project_id) REFERENCES agent_control_records(workspace,actor,id));
CREATE TABLE IF NOT EXISTS agent_control_commands (
workspace TEXT NOT NULL, actor TEXT NOT NULL, request_id TEXT NOT NULL,
digest TEXT NOT NULL, receipt BLOB NOT NULL, PRIMARY KEY(workspace,actor,request_id));
CREATE TABLE IF NOT EXISTS agent_control_events (
sequence INTEGER PRIMARY KEY AUTOINCREMENT, workspace TEXT NOT NULL, actor TEXT NOT NULL,
target_id TEXT NOT NULL, revision INTEGER NOT NULL, kind TEXT NOT NULL,
recorded_at TEXT NOT NULL, payload BLOB NOT NULL,
FOREIGN KEY(workspace,actor,target_id) REFERENCES agent_control_records(workspace,actor,id));
CREATE INDEX IF NOT EXISTS agent_control_event_order ON agent_control_events(workspace,actor,target_id,sequence);
CREATE TABLE IF NOT EXISTS agent_control_outbox (
event_id INTEGER PRIMARY KEY REFERENCES agent_control_events(sequence),
run_id TEXT REFERENCES agent_tasks(id), disposition TEXT NOT NULL DEFAULT 'pending');
CREATE TABLE IF NOT EXISTS agent_control_links (
workspace TEXT NOT NULL, actor TEXT NOT NULL, conversation_id TEXT NOT NULL, work_id TEXT NOT NULL,
PRIMARY KEY(workspace,actor,conversation_id,work_id),
FOREIGN KEY(workspace,actor,conversation_id) REFERENCES agent_control_records(workspace,actor,id),
FOREIGN KEY(workspace,actor,work_id) REFERENCES agent_control_records(workspace,actor,id));
CREATE TABLE IF NOT EXISTS agent_conversation_imports (
workspace TEXT NOT NULL, actor TEXT NOT NULL, legacy_name TEXT NOT NULL,
conversation_id TEXT NOT NULL, source_digest TEXT NOT NULL, manifest BLOB NOT NULL, source BLOB NOT NULL,
PRIMARY KEY(workspace,actor,legacy_name),
FOREIGN KEY(workspace,actor,conversation_id) REFERENCES agent_control_records(workspace,actor,id));
CREATE TABLE IF NOT EXISTS agent_conversation_messages (
sequence INTEGER PRIMARY KEY AUTOINCREMENT, workspace TEXT NOT NULL, actor TEXT NOT NULL,
conversation_id TEXT NOT NULL, message_id TEXT NOT NULL, payload BLOB NOT NULL,
UNIQUE(workspace,actor,conversation_id,message_id),
FOREIGN KEY(workspace,actor,conversation_id) REFERENCES agent_control_records(workspace,actor,id));
CREATE TABLE IF NOT EXISTS agent_worker_messages (
sequence INTEGER PRIMARY KEY AUTOINCREMENT, workspace TEXT NOT NULL, actor TEXT NOT NULL,
worker_id TEXT NOT NULL, message_id TEXT NOT NULL, payload BLOB NOT NULL,
UNIQUE(workspace,actor,worker_id,message_id),
FOREIGN KEY(workspace,actor,worker_id) REFERENCES agent_control_records(workspace,actor,id));
CREATE TABLE IF NOT EXISTS agent_conversation_inbox (
sequence INTEGER PRIMARY KEY AUTOINCREMENT, workspace TEXT NOT NULL, actor TEXT NOT NULL,
conversation_id TEXT NOT NULL, input_id TEXT NOT NULL, state TEXT NOT NULL,
payload BLOB NOT NULL, UNIQUE(workspace,actor,input_id),
FOREIGN KEY(workspace,actor,conversation_id) REFERENCES agent_control_records(workspace,actor,id));
CREATE INDEX IF NOT EXISTS agent_conversation_inbox_order ON agent_conversation_inbox(workspace,actor,conversation_id,state,sequence);`

func (m *Manager) controlDatabase() (*sql.DB, error) {
	if m.dataDir == "" {
		return nil, ErrRunStorage
	}
	if err := m.StorageError(); err != nil {
		return nil, err
	}
	db, err := openTaskDatabase(m.dataDir)
	if err != nil {
		return nil, err
	}
	var version int
	if err = db.QueryRow(`SELECT version FROM agent_schema WHERE singleton=1`).Scan(&version); err == nil && version != taskSchemaVersion {
		err = ErrRunStorage
	}
	if err == nil {
		_, err = db.Exec(controlSchema)
	}
	if err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

type controlReader interface{ QueryRow(string, ...any) *sql.Row }

func readControlRecord(q controlReader, scope ControlScope, id, kind string) (*controlRecord, error) {
	if !validProfileOwner(scope.Workspace, scope.Actor) {
		return nil, ErrControlNotFound
	}
	r := &controlRecord{ID: id, Kind: kind}
	err := q.QueryRow(`SELECT revision,event_floor,snapshot FROM agent_control_records WHERE workspace=? AND actor=? AND id=? AND kind=? AND NOT EXISTS (SELECT 1 FROM agent_conversation_deletions d WHERE d.workspace=agent_control_records.workspace AND d.actor=agent_control_records.actor AND d.conversation_id=agent_control_records.id)`, scope.Workspace, scope.Actor, id, kind).Scan(&r.Revision, &r.Floor, &r.Snapshot)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrControlNotFound
	}
	return r, err
}

func putControlRecord(tx *sql.Tx, scope ControlScope, id, kind string, revision int64, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	result, err := tx.Exec(`INSERT INTO agent_control_records(workspace,actor,id,kind,revision,snapshot) VALUES(?,?,?,?,?,?) ON CONFLICT(workspace,actor,id) DO UPDATE SET revision=excluded.revision,snapshot=excluded.snapshot WHERE kind=excluded.kind`, scope.Workspace, scope.Actor, id, kind, revision, data)
	if err != nil {
		return err
	}
	if count, err := result.RowsAffected(); err != nil {
		return err
	} else if count != 1 {
		return ErrRunConflict
	}
	if kind == "project" {
		if _, err = tx.Exec(`INSERT INTO agent_project_revisions(workspace,actor,project_id,revision,snapshot) VALUES(?,?,?,?,?) ON CONFLICT DO NOTHING`, scope.Workspace, scope.Actor, id, revision, data); err != nil {
			return err
		}
		var saved []byte
		if err = tx.QueryRow(`SELECT snapshot FROM agent_project_revisions WHERE workspace=? AND actor=? AND project_id=? AND revision=?`, scope.Workspace, scope.Actor, id, revision).Scan(&saved); err != nil {
			return err
		}
		if !bytes.Equal(data, saved) {
			return ErrRunConflict
		}
	}
	return nil
}

func appendControlEvent(tx *sql.Tx, scope ControlScope, id string, revision int64, kind string, payload any, runID string) (int64, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}
	res, err := tx.Exec(`INSERT INTO agent_control_events(workspace,actor,target_id,revision,kind,recorded_at,payload) VALUES(?,?,?,?,?,?,?)`, scope.Workspace, scope.Actor, id, revision, kind, time.Now().UTC().Format(time.RFC3339Nano), data)
	if err != nil {
		return 0, err
	}
	cursor, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	var dispatch any
	if runID != "" {
		dispatch = runID
	}
	_, err = tx.Exec(`INSERT INTO agent_control_outbox(event_id,run_id) VALUES(?,?)`, cursor, dispatch)
	return cursor, err
}

func (m *Manager) Conversation(scope ControlScope, id string) (*Conversation, error) {
	db, err := m.controlDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	r, err := readControlRecord(db, scope, id, "conversation")
	if err != nil {
		return nil, err
	}
	var c Conversation
	err = json.Unmarshal(r.Snapshot, &c)
	return &c, err
}

func (m *Manager) WorkItem(scope ControlScope, id string) (*WorkItem, error) {
	db, err := m.controlDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	r, err := readControlRecord(db, scope, id, "work")
	if err != nil {
		return nil, err
	}
	var w WorkItem
	err = json.Unmarshal(r.Snapshot, &w)
	return &w, err
}

func (m *Manager) Project(scope ControlScope, id string) (*Project, error) {
	db, err := m.controlDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	r, err := readControlRecord(db, scope, id, "project")
	if err != nil {
		return nil, err
	}
	var p Project
	err = json.Unmarshal(r.Snapshot, &p)
	return &p, err
}

// Each page and history-gap snapshot is read from one SQLite snapshot.
// Cursors are global opaque offsets; gaps caused by other actors disclose no data.
func (m *Manager) ControlEvents(scope ControlScope, id, kind string, after int64, limit int) (*ControlEventPage, error) {
	if after < 0 || limit < 1 || limit > 200 {
		return nil, ErrControlCommand
	}
	db, err := m.controlDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	r, err := readControlRecord(tx, scope, id, kind)
	if err != nil {
		return nil, err
	}
	page := &ControlEventPage{Events: []ControlEvent{}, NextCursor: after}
	if after < r.Floor {
		page.HistoryGap = true
		page.Snapshot = r.Snapshot
		after = r.Floor
		page.NextCursor = after
	}
	rows, err := tx.Query(`SELECT sequence,revision,kind,recorded_at,payload FROM agent_control_events WHERE workspace=? AND actor=? AND target_id=? AND sequence>? ORDER BY sequence LIMIT ?`, scope.Workspace, scope.Actor, id, after, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		if len(page.Events) == limit {
			page.HasMore = true
			break
		}
		var e ControlEvent
		var stamp string
		e.TargetID = id
		if err = rows.Scan(&e.Cursor, &e.Revision, &e.Kind, &stamp, &e.Payload); err != nil {
			return nil, err
		}
		if e.RecordedAt, err = time.Parse(time.RFC3339Nano, stamp); err != nil {
			return nil, err
		}
		page.Events = append(page.Events, e)
		page.NextCursor = e.Cursor
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return page, nil
}
