package agents

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const retainedTaskEvents = 256

var ErrEventCursor = errors.New("task event cursor requires snapshot recovery")

// TaskActivity deliberately excludes prompts, page contents, arguments and
// credentials. The owner-only snapshot holds the actual results and approvals.
type TaskActivity struct {
	Sequence       int64      `json:"sequence"`
	TaskID         string     `json:"task_id"`
	Status         TaskStatus `json:"status"`
	Phase          string     `json:"phase,omitempty"`
	Tool           string     `json:"tool,omitempty"`
	Iteration      int        `json:"iteration,omitempty"`
	CompletedSteps int        `json:"completed_steps"`
	ApprovalID     string     `json:"approval_id,omitempty"`
	ExecutingCall  string     `json:"executing_call,omitempty"`
	SnapshotSHA256 string     `json:"snapshot_sha256"`
	RecordedAt     time.Time  `json:"recorded_at"`
}

type TaskReplay struct {
	Events   []TaskActivity
	Snapshot *Task
	Cursor   int64
	Recovery bool
}

func (m *Manager) ActivityReplayAvailable() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.dataDir != "" && m.storageErr == nil
}

func appendTaskActivity(tx *sql.Tx, task *Task, snapshot []byte) error {
	digest := sha256.Sum256(snapshot)
	event := TaskActivity{TaskID: task.ID, Status: task.Status, CompletedSteps: len(task.Steps), SnapshotSHA256: hex.EncodeToString(digest[:]), RecordedAt: time.Now().UTC()}
	if task.Progress != nil {
		event.Phase, event.Tool, event.Iteration = task.Progress.Phase, task.Progress.Tool, task.Progress.Iteration
	}
	if task.PendingApproval != nil {
		event.ApprovalID = task.PendingApproval.ID
	}
	if task.Checkpoint != nil {
		event.ExecutingCall = task.Checkpoint.ExecutingCall
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(`INSERT INTO agent_task_events(task_id,status,recorded_at,payload) VALUES(?,?,?,?)`, task.ID, task.Status, event.RecordedAt.Format(time.RFC3339Nano), payload); err != nil {
		return err
	}
	// Compact only metadata. Completed snapshots and their result history remain.
	var floor sql.NullInt64
	if err = tx.QueryRow(`SELECT sequence FROM agent_task_events WHERE task_id=? ORDER BY sequence DESC LIMIT 1 OFFSET ?`, task.ID, retainedTaskEvents).Scan(&floor); err != nil && err != sql.ErrNoRows {
		return err
	}
	if floor.Valid {
		if _, err = tx.Exec(`UPDATE agent_tasks SET event_floor=MAX(event_floor,?) WHERE id=?`, floor.Int64, task.ID); err != nil {
			return err
		}
		_, err = tx.Exec(`DELETE FROM agent_task_events WHERE task_id=? AND sequence<=?`, task.ID, floor.Int64)
		return err
	}
	return nil
}

// ReplayTask reads one consistent, actor-scoped snapshot and event window. A
// subscriber never holds a manager mutex or an execution channel while writing
// to the network. Negative/ahead/compacted cursors require explicit recovery.
func (m *Manager) ReplayTask(ctx context.Context, id, actor string, after int64) (*TaskReplay, error) {
	if !validTaskID(id) || actor == "" {
		return nil, ErrTaskNotFound
	}
	m.mu.RLock()
	directory, storageErr := m.dataDir, m.storageErr
	m.mu.RUnlock()
	if storageErr != nil || directory == "" {
		return nil, ErrRunStorage
	}
	db, err := openTaskDatabase(directory)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRunStorage, err)
	}
	defer db.Close()
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var data []byte
	var floor int64
	if err = tx.QueryRowContext(ctx, `SELECT snapshot,event_floor FROM agent_tasks WHERE id=? AND actor=?`, id, actor).Scan(&data, &floor); err == sql.ErrNoRows {
		return nil, ErrTaskNotFound
	} else if err != nil {
		return nil, err
	}
	var task Task
	if err = json.Unmarshal(data, &task); err != nil {
		return nil, ErrRunStorage
	}
	if task.ID != id || task.Actor != actor || validateStoredTask(&task) != nil {
		return nil, ErrRunStorage
	}
	if task.DeletedAt != nil {
		return nil, ErrTaskNotFound
	}
	replay := &TaskReplay{Snapshot: &task, Events: []TaskActivity{}}
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(sequence),?) FROM agent_task_events WHERE task_id=?`, floor, id).Scan(&replay.Cursor); err != nil {
		return nil, err
	}
	if after < floor || after > replay.Cursor || after < 0 {
		replay.Recovery = true
		return replay, ErrEventCursor
	}
	rows, err := tx.QueryContext(ctx, `SELECT sequence,payload FROM agent_task_events WHERE task_id=? AND sequence>? ORDER BY sequence`, id, after)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var sequence int64
		var payload []byte
		if err = rows.Scan(&sequence, &payload); err != nil {
			return nil, err
		}
		var event TaskActivity
		if err = json.Unmarshal(payload, &event); err != nil || event.TaskID != id {
			return nil, ErrRunStorage
		}
		event.Sequence = sequence
		replay.Events = append(replay.Events, event)
	}
	return replay, rows.Err()
}

// Startup runs under exclusive workspace ownership, before accepting jobs.
// VACUUM INTO captures committed WAL content; copying the main file would not.
func migrateTaskActivity(db *sql.DB, directory string) error {
	if err := validateTaskDatabaseRows(db); err != nil {
		return err
	}
	if err := validateLegacyTaskEvents(db); err != nil {
		return err
	}
	backupDir, err := os.MkdirTemp(directory, "agent-schema-1-backup-")
	if err != nil {
		return err
	}
	backup := filepath.Join(backupDir, taskDatabaseName)
	if _, err = db.Exec(`VACUUM INTO ?`, backup); err != nil {
		return fmt.Errorf("agent schema backup failed; original unchanged: %w", err)
	}
	if err = os.Chmod(backup, 0600); err != nil {
		return err
	}
	check, err := sql.Open("sqlite", backup)
	if err != nil {
		return err
	}
	verifyErr := validateTaskDatabaseRows(check)
	if verifyErr == nil {
		for _, query := range []string{`SELECT COUNT(*) FROM agent_tasks`, `SELECT COUNT(*) FROM agent_task_events`} {
			var sourceCount, backupCount int
			if verifyErr = db.QueryRow(query).Scan(&sourceCount); verifyErr != nil {
				break
			}
			if verifyErr = check.QueryRow(query).Scan(&backupCount); verifyErr != nil {
				break
			}
			if sourceCount != backupCount {
				verifyErr = fmt.Errorf("agent migration backup count mismatch")
				break
			}
		}
	}
	closeErr := check.Close()
	if verifyErr != nil {
		return verifyErr
	}
	if closeErr != nil {
		return closeErr
	}
	file, err := os.Open(backup)
	if err != nil {
		return err
	}
	digest := sha256.New()
	_, hashErr := io.Copy(digest, file)
	closeErr = file.Close()
	if hashErr != nil {
		return hashErr
	}
	if closeErr != nil {
		return closeErr
	}
	manifest, err := json.Marshal(map[string]any{"from": 1, "to": 2, "backup": filepath.Join(filepath.Base(backupDir), taskDatabaseName), "sha256": hex.EncodeToString(digest.Sum(nil)), "created_at": time.Now().UTC()})
	if err != nil {
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.Exec(`ALTER TABLE agent_tasks ADD COLUMN event_floor INTEGER NOT NULL DEFAULT 0;
ALTER TABLE agent_task_events ADD COLUMN payload BLOB;
CREATE INDEX agent_task_event_order ON agent_task_events(task_id,sequence);
CREATE TABLE agent_migrations(version INTEGER PRIMARY KEY,manifest BLOB NOT NULL);
UPDATE agent_tasks SET event_floor=COALESCE((SELECT MAX(sequence) FROM agent_task_events WHERE task_id=agent_tasks.id),0);
DELETE FROM agent_task_events;
UPDATE agent_schema SET version=2 WHERE singleton=1 AND version=1`)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(`INSERT INTO agent_migrations VALUES(2,?)`, manifest); err != nil {
		return err
	}
	return tx.Commit()
}

func validateLegacyTaskEvents(db *sql.DB) error {
	rows, err := db.Query(`SELECT sequence,task_id,status,recorded_at FROM agent_task_events ORDER BY sequence`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var sequence int64
		var id, status, recorded string
		if err := rows.Scan(&sequence, &id, &status, &recorded); err != nil {
			return err
		}
		timestamp, err := time.Parse(time.RFC3339Nano, recorded)
		if err != nil || timestamp.IsZero() || sequence <= 0 || validateStoredTask(&Task{ID: id, Status: TaskStatus(status)}) != nil {
			return fmt.Errorf("invalid legacy agent event %d; repair or restore before migration", sequence)
		}
	}
	return rows.Err()
}

func validateTaskDatabaseRows(db *sql.DB) error {
	var integrity string
	if err := db.QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil {
		return err
	} else if integrity != "ok" {
		return fmt.Errorf("agent database integrity check failed")
	}
	foreign, err := db.Query(`PRAGMA foreign_key_check`)
	if err != nil {
		return err
	}
	badForeignKey := foreign.Next()
	foreignErr := foreign.Err()
	foreign.Close()
	if foreignErr != nil {
		return foreignErr
	}
	if badForeignKey {
		return fmt.Errorf("agent database has orphaned event records")
	}
	rows, err := db.Query(`SELECT id,actor,snapshot FROM agent_tasks`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id, actor string
		var data []byte
		if err := rows.Scan(&id, &actor, &data); err != nil {
			return err
		}
		var task Task
		if json.Unmarshal(data, &task) != nil || validateStoredTask(&task) != nil || task.ID != id || task.Actor != actor {
			return fmt.Errorf("agent snapshot invalid; repair or restore before migration: %s", id)
		}
	}
	return rows.Err()
}

func validateTaskActivityDatabase(db *sql.DB) error {
	if err := validateTaskDatabaseRows(db); err != nil {
		return err
	}
	rows, err := db.Query(`SELECT sequence,task_id,status,recorded_at,payload FROM agent_task_events ORDER BY sequence`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var sequence int64
		var taskID, status, recorded string
		var payload []byte
		if err := rows.Scan(&sequence, &taskID, &status, &recorded, &payload); err != nil {
			return err
		}
		var event TaskActivity
		if json.Unmarshal(payload, &event) != nil || sequence <= 0 || event.TaskID != taskID || string(event.Status) != status || event.RecordedAt.IsZero() || event.RecordedAt.Format(time.RFC3339Nano) != recorded || event.CompletedSteps < 0 || validateStoredTask(&Task{ID: taskID, Status: event.Status}) != nil {
			return fmt.Errorf("invalid agent activity record %d; restore or repair before startup", sequence)
		}
		digest, err := hex.DecodeString(event.SnapshotSHA256)
		if err != nil || len(digest) != sha256.Size {
			return fmt.Errorf("invalid agent activity digest %d", sequence)
		}
	}
	return rows.Err()
}
