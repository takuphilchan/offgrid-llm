package agents

import (
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
const taskSchemaVersion = 1

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

func validateStoredTask(task *Task) error {
	if !validTaskID(task.ID) {
		return fmt.Errorf("invalid task identity")
	}
	switch task.Status {
	case TaskPending, TaskRunning, TaskWaiting, TaskCompleted, TaskFailed, TaskCancelled, TaskInterrupted, TaskUncertain:
	default:
		return fmt.Errorf("invalid task status for %s", task.ID)
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
	_, err = tx.Exec(`CREATE TABLE agent_tasks (id TEXT PRIMARY KEY, actor TEXT NOT NULL, snapshot BLOB NOT NULL);
CREATE TABLE agent_task_events (sequence INTEGER PRIMARY KEY AUTOINCREMENT, task_id TEXT NOT NULL REFERENCES agent_tasks(id), status TEXT NOT NULL, recorded_at TEXT NOT NULL)`)
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
	var previous string
	err = tx.QueryRow(`SELECT json_extract(snapshot, '$.status') FROM agent_tasks WHERE id=?`, task.ID).Scan(&previous)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if _, err = tx.Exec(`INSERT INTO agent_tasks(id,actor,snapshot) VALUES(?,?,?) ON CONFLICT(id) DO UPDATE SET actor=excluded.actor,snapshot=excluded.snapshot`, task.ID, task.Actor, data); err != nil {
		return err
	}
	// Progress updates do not duplicate content into an unbounded event journal.
	if previous != string(task.Status) {
		_, err = tx.Exec(`INSERT INTO agent_task_events(task_id,status,recorded_at) VALUES(?,?,?)`, task.ID, task.Status, time.Now().UTC().Format(time.RFC3339Nano))
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
