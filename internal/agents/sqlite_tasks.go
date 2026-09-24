package agents

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/storage"
)

const taskDatabaseName = "agent-state.sqlite"
const taskSchemaVersion = 4

// The service owns the workspace lock. Connections are operation-scoped so
// stopped-workspace backup/restore never races an idle connection or leaked WAL.
func openTaskDatabase(directory string) (*sql.DB, error) {
	if err := os.MkdirAll(directory, 0700); err != nil {
		return nil, err
	}
	path := filepath.Join(directory, taskDatabaseName)
	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		return nil, fmt.Errorf("agent database must be a regular file")
	} else if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	// Set permissions before SQLite creates a WAL using the database's mode.
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	modeErr := file.Chmod(0600)
	closeErr := file.Close()
	if modeErr != nil {
		return nil, modeErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	return storage.OpenSQLite(path)
}

type taskImportFile struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	ID     string `json:"id"`
	Actor  string `json:"actor,omitempty"`
}

func validateTaskIdentityStatus(task *Task) error {
	if !validTaskID(task.ID) {
		return fmt.Errorf("invalid task identity")
	}
	switch task.Status {
	case TaskPending, TaskRunning, TaskWaiting, TaskInput, TaskChildren, TaskCompleted, TaskFailed, TaskCancelled, TaskInterrupted, TaskUncertain:
	default:
		return fmt.Errorf("invalid task status for %s", task.ID)
	}
	return nil
}

func validateStoredTask(task *Task) error {
	if err := validateTaskIdentityStatus(task); err != nil {
		return err
	}
	if task.Config.ComputerStepOffset < 0 || task.Config.ComputerStepOffset > len(task.Steps) {
		return fmt.Errorf("invalid computer step boundary")
	}
	if cp := task.Checkpoint; cp != nil && cp.ReadOnlyCall != "" {
		if cp.ExecutingCall != cp.ReadOnlyCall || len(cp.Calls) == 0 || cp.Calls[0].ID != cp.ReadOnlyCall {
			return fmt.Errorf("read-only intent does not match executing call")
		}
		switch cp.Calls[0].Function.Name {
		case "read_file", "list_files", "calculator", "current_time":
		default:
			return fmt.Errorf("invalid read-only intent tool")
		}
	}
	for _, archive := range task.ContextArchive {
		data, err := json.Marshal(archive.Messages)
		if err != nil {
			return err
		}
		digest := sha256.Sum256(data)
		if archive.ID != hex.EncodeToString(digest[:]) {
			return fmt.Errorf("archived task context digest mismatch")
		}
	}
	if task.Status == TaskChildren {
		if task.Delegation == nil || task.ParentID != "" || task.Checkpoint == nil || task.Checkpoint.ExecutingCall != "" || len(task.Checkpoint.Calls) != 1 || task.Checkpoint.Calls[0].ID != task.Delegation.CallID {
			return fmt.Errorf("invalid delegated checkpoint")
		}
		specs := []ChildSpec{}
		for _, child := range task.Delegation.Children {
			if !validTaskID(child.ID) {
				return fmt.Errorf("invalid child identity")
			}
			specs = append(specs, child.Spec)
		}
		if err := validateChildSpecs(specs); err != nil {
			return err
		}
	}
	if task.Status == TaskInput {
		input, cp := task.PendingInput, task.Checkpoint
		if input == nil || input.ID == "" || input.CallID == "" || input.Kind != "computer" || (input.Mode != "app" && input.Mode != "browser") || input.Target == "" || cp == nil || cp.ExecutingCall != "" || len(cp.Calls) != 1 || cp.Calls[0].ID != input.CallID || task.Config.ComputerSession != "" || !task.Config.TaskFirst {
			return fmt.Errorf("invalid task input checkpoint for %s", task.ID)
		}
	}
	if a := task.PendingApproval; a != nil {
		if a.ID == "" || a.RunID != task.ID || a.Actor != task.Actor || a.CallID == "" || a.Tool == "" {
			return fmt.Errorf("invalid approval ownership or identity for %s", task.ID)
		}
		canonical, err := CanonicalArguments(a.Arguments)
		if err != nil || string(canonical) != a.ArgumentsJSON {
			return fmt.Errorf("invalid approval arguments for %s", task.ID)
		}
	}
	return nil
}

func readLegacyTasks(directory string) ([]*Task, []taskImportFile, error) {
	root := filepath.Join(directory, "agent_tasks")
	if info, err := os.Lstat(root); err == nil && !info.IsDir() {
		return nil, nil, fmt.Errorf("legacy agent_tasks must be a directory, not a link or file")
	}
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return []*Task{}, []taskImportFile{}, nil
	}
	if err != nil {
		return nil, nil, err
	}
	tasks, manifest := []*Task{}, []taskImportFile{}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, nil, err
		}
		if !info.Mode().IsRegular() || info.Size() > 32<<20 {
			return nil, nil, fmt.Errorf("invalid legacy task file %s", entry.Name())
		}
		data, err := os.ReadFile(filepath.Join(root, entry.Name()))
		if err != nil {
			return nil, nil, err
		}
		var task Task
		if err = json.Unmarshal(data, &task); err != nil {
			return nil, nil, fmt.Errorf("invalid legacy task %s: repair or restore the original snapshot before retrying", entry.Name())
		}
		if err = validateStoredTask(&task); err != nil {
			return nil, nil, fmt.Errorf("%s: %w", entry.Name(), err)
		}
		if entry.Name() != task.ID+".json" {
			return nil, nil, fmt.Errorf("legacy task filename/identity mismatch: %s", entry.Name())
		}
		digest := sha256.Sum256(data)
		manifest = append(manifest, taskImportFile{Name: entry.Name(), SHA256: hex.EncodeToString(digest[:]), ID: task.ID, Actor: task.Actor})
		tasks = append(tasks, &task)
	}
	return tasks, manifest, nil
}

// Migration is staged inside one transaction. No task becomes active unless
// every source and ownership record validates. Original JSON is never modified.
func initializeTaskDatabase(db *sql.DB, directory string) error {
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`CREATE TABLE IF NOT EXISTS agent_schema (singleton INTEGER PRIMARY KEY CHECK(singleton=1), version INTEGER NOT NULL, recovery_manifest BLOB NOT NULL)`); err != nil {
		return err
	}
	var version int
	err = tx.QueryRow(`SELECT version FROM agent_schema WHERE singleton=1`).Scan(&version)
	if err == nil {
		if version == 1 {
			if err := tx.Rollback(); err != nil {
				return err
			}
			if err := migrateTaskActivity(db, directory); err != nil {
				return err
			}
			return migrateTaskInputs(db, directory)
		}
		if version == 2 {
			if err := tx.Rollback(); err != nil {
				return err
			}
			return migrateTaskInputs(db, directory)
		}
		if version == 3 {
			if err := tx.Rollback(); err != nil {
				return err
			}
			return migrateTaskVersion(db, directory, 3, taskSchemaVersion)
		}
		if version != taskSchemaVersion {
			return fmt.Errorf("agent schema %d is incompatible; use a matching OffGrid version", version)
		}
		return tx.Commit()
	}
	if err != sql.ErrNoRows {
		return err
	}
	tasks, manifest, err := readLegacyTasks(directory)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`CREATE TABLE agent_tasks (id TEXT PRIMARY KEY, actor TEXT NOT NULL, snapshot BLOB NOT NULL, event_floor INTEGER NOT NULL DEFAULT 0);
CREATE TABLE agent_task_events (sequence INTEGER PRIMARY KEY AUTOINCREMENT, task_id TEXT NOT NULL REFERENCES agent_tasks(id), status TEXT NOT NULL, recorded_at TEXT NOT NULL, payload BLOB);
CREATE INDEX agent_task_event_order ON agent_task_events(task_id,sequence);
CREATE TABLE agent_migrations(version INTEGER PRIMARY KEY, manifest BLOB NOT NULL)`)
	if err != nil {
		return err
	}
	for _, task := range tasks {
		if err = putTaskTransaction(tx, task); err != nil {
			return err
		}
	}
	var count int
	if err = tx.QueryRow(`SELECT COUNT(*) FROM agent_tasks`).Scan(&count); err != nil {
		return err
	}
	if count != len(tasks) {
		return fmt.Errorf("agent migration count mismatch")
	}
	var integrity string
	if err = tx.QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil {
		return err
	}
	if integrity != "ok" {
		return fmt.Errorf("agent migration integrity check failed")
	}
	recovery, err := json.Marshal(struct {
		Format          int              `json:"format"`
		Created         time.Time        `json:"created_at"`
		Sources         []taskImportFile `json:"sources"`
		LegacyOwnership string           `json:"legacy_ownership"`
	}{1, time.Now().UTC(), manifest, "Unowned records remain local-admin only; originals retained in agent_tasks."})
	if err != nil {
		return err
	}
	if _, err = tx.Exec(`INSERT INTO agent_schema VALUES(1, ?, ?)`, taskSchemaVersion, recovery); err != nil {
		return err
	}
	return tx.Commit()
}

func putTaskTransaction(tx *sql.Tx, task *Task) error {
	if err := validateStoredTask(task); err != nil {
		return err
	}
	data, err := json.Marshal(task)
	if err != nil {
		return err
	}
	var previous []byte
	err = tx.QueryRow(`SELECT snapshot FROM agent_tasks WHERE id=?`, task.ID).Scan(&previous)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if _, err = tx.Exec(`INSERT INTO agent_tasks(id,actor,snapshot) VALUES(?,?,?) ON CONFLICT(id) DO UPDATE SET actor=excluded.actor,snapshot=excluded.snapshot`, task.ID, task.Actor, data); err != nil {
		return err
	}
	if !bytes.Equal(previous, data) {
		return appendTaskActivity(tx, task, data)
	}
	return err
}

func persistTask(directory string, task *Task) error {
	db, err := openTaskDatabase(directory)
	if err != nil {
		return err
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = putTaskTransaction(tx, task); err != nil {
		return err
	}
	return tx.Commit()
}

func loadTaskDatabase(directory string) ([]*Task, error) {
	db, err := openTaskDatabase(directory)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	if err = initializeTaskDatabase(db, directory); err != nil {
		return nil, err
	}
	if err = validateTaskActivityDatabase(db); err != nil {
		return nil, err
	}
	rows, err := db.Query(`SELECT id, actor, snapshot FROM agent_tasks ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tasks := []*Task{}
	for rows.Next() {
		var id, actor string
		var data []byte
		if err = rows.Scan(&id, &actor, &data); err != nil {
			return nil, err
		}
		var task Task
		if err = json.Unmarshal(data, &task); err != nil {
			return nil, fmt.Errorf("invalid agent snapshot %s", id)
		}
		if err = validateStoredTask(&task); err != nil {
			return nil, err
		}
		if task.ID != id || task.Actor != actor {
			return nil, fmt.Errorf("agent snapshot ownership mismatch: %s", id)
		}
		tasks = append(tasks, &task)
	}
	return tasks, rows.Err()
}
