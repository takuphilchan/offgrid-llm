package agents

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRestartRevokesComputerApprovalWithoutDiscardingCheckpoint(t *testing.T) {
	dir := t.TempDir()
	m := NewManagerWithPersistence(nil, nil, nil, dir)
	config := DefaultAgentConfig()
	config.ComputerSession = "old-companion-session"
	task, err := m.CreateTask("computer-restart", "edit a draft", &config)
	if err != nil {
		t.Fatal(err)
	}
	_, err = m.updateTask(task.ID, func(task *Task) error {
		task.Actor = "alice"
		task.Status = TaskWaiting
		task.Checkpoint = &Checkpoint{Iteration: 3}
		task.PendingApproval = &Approval{ID: "old-approval", RunID: task.ID, Actor: task.Actor, CallID: "call", Tool: "browser_fill", Arguments: json.RawMessage(`{}`), ArgumentsJSON: `{}`, ExpiresAt: time.Now().Add(time.Hour)}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	m = NewManagerWithPersistence(nil, nil, nil, dir)
	if err := m.StorageError(); err != nil {
		t.Fatal(err)
	}
	got, ok := m.GetTask(task.ID)
	if !ok || got.Status != TaskInterrupted || got.PendingApproval != nil || got.Checkpoint == nil || got.Checkpoint.Iteration != 3 {
		t.Fatalf("restart retained control authority or lost history: %+v", got)
	}
	if _, err := NewRunner(m, nil, nil).Continue(context.Background(), task.ID, "alice", "approve", "old-approval", false); err != ErrRunConflict {
		t.Fatalf("old approval accepted: %v", err)
	}
	if _, err := NewRunner(m, nil, nil).Continue(context.Background(), task.ID, "alice", "resume", "", false); err != ErrRunConflict {
		t.Fatalf("expired computer session resumed: %v", err)
	}
}

func legacyTaskFixture(t *testing.T, directory, name string, data []byte) string {
	t.Helper()
	root := filepath.Join(directory, "agent_tasks")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, name)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestTaskMigrationPreservesOriginalsAndLegacyOwnership(t *testing.T) {
	dir := t.TempDir()
	source := []byte(`{"id":"legacy","prompt":"Résumé 日本語","status":"completed","result":"saved"}`)
	path := legacyTaskFixture(t, dir, "legacy.json", source)
	m := NewManagerWithPersistence(nil, nil, nil, dir)
	if err := m.StorageError(); err != nil {
		t.Fatal(err)
	}
	task, ok := m.GetTask("legacy")
	if !ok || task.Actor != "" || task.Result != "saved" {
		t.Fatalf("changed legacy ownership/content: %#v", task)
	}
	if err := NewRunner(m, nil, nil).Delete("legacy", "alice"); err != ErrTaskNotFound {
		t.Fatalf("legacy access: %v", err)
	}
	if err := NewRunner(m, nil, nil).Delete("legacy", "local-admin"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(source, data) {
		t.Fatal("original migration source changed")
	}
	db, err := openTaskDatabase(dir)
	if err != nil {
		t.Fatal(err)
	}
	var recovery []byte
	err = db.QueryRow(`SELECT recovery_manifest FROM agent_schema`).Scan(&recovery)
	db.Close()
	var manifest struct {
		Sources []taskImportFile `json:"sources"`
	}
	if err != nil || json.Unmarshal(recovery, &manifest) != nil || len(manifest.Sources) != 1 || manifest.Sources[0].SHA256 == "" {
		t.Fatalf("missing recovery evidence: %s %v", recovery, err)
	}
	// An active database is authoritative; old snapshots must never resurrect work.
	m = NewManagerWithPersistence(nil, nil, nil, dir)
	if err := m.StorageError(); err != nil {
		t.Fatal(err)
	}
	if !m.IsDeleted("legacy") {
		t.Fatal("legacy source reimported after activation")
	}
}

func TestTaskMigrationRejectsEntireCorruptImport(t *testing.T) {
	for _, broken := range []string{
		`{`,
		`{"id":"different","status":"completed"}`,
		`{"id":"z-broken","status":"made_up"}`,
		`{"id":"z-broken","status":"waiting_for_approval","actor":"alice","pending_approval":{"id":"x","run_id":"z-broken","actor":"bob"}}`,
	} {
		t.Run(broken, func(t *testing.T) {
			dir := t.TempDir()
			legacyTaskFixture(t, dir, "a-valid.json", []byte(`{"id":"a-valid","status":"completed"}`))
			path := legacyTaskFixture(t, dir, "z-broken.json", []byte(broken))
			m := NewManagerWithPersistence(nil, nil, nil, dir)
			if m.StorageError() == nil || len(m.ListTasks()) != 0 {
				t.Fatal("partial migration was exposed")
			}
			if _, err := m.CreateTask("new", "must not run", nil); err == nil {
				t.Fatal("admitted work after migration failure")
			}
			if err := os.WriteFile(path, []byte(`{"id":"z-broken","status":"completed"}`), 0600); err != nil {
				t.Fatal(err)
			}
			m = NewManagerWithPersistence(nil, nil, nil, dir)
			if m.StorageError() != nil || len(m.ListTasks()) != 2 {
				t.Fatalf("repair/retry failed: %v", m.StorageError())
			}
		})
	}
}

func TestTaskTransactionRollsBackSnapshotWhenEventWriteFails(t *testing.T) {
	dir := t.TempDir()
	m := NewManagerWithPersistence(nil, nil, nil, dir)
	if _, err := m.CreateTask("task", "hello", nil); err != nil {
		t.Fatal(err)
	}
	db, err := openTaskDatabase(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TRIGGER reject_event BEFORE INSERT ON agent_task_events BEGIN SELECT RAISE(ABORT,'injected disk failure'); END`)
	if err != nil {
		t.Fatal(err)
	}
	if err = m.StartTask("task"); err == nil {
		t.Fatal("reported success without durable event")
	}
	task, _ := m.GetTask("task")
	if task.Status != TaskPending {
		t.Fatal("failed transaction published in memory")
	}
	var status string
	if err = db.QueryRow(`SELECT json_extract(snapshot,'$.status') FROM agent_tasks WHERE id='task'`).Scan(&status); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if status != "pending" {
		t.Fatal("failed event left a changed snapshot")
	}
}

func TestTaskDatabaseRejectsNewerSchemaAndOwnershipMismatch(t *testing.T) {
	for _, statement := range []string{
		`UPDATE agent_schema SET version=999`,
		`UPDATE agent_tasks SET actor='someone-else'`,
	} {
		t.Run(statement, func(t *testing.T) {
			dir := t.TempDir()
			m := NewManagerWithPersistence(nil, nil, nil, dir)
			if _, err := m.CreateTask("task", "private", nil); err != nil {
				t.Fatal(err)
			}
			db, err := openTaskDatabase(dir)
			if err != nil {
				t.Fatal(err)
			}
			_, err = db.Exec(statement)
			db.Close()
			if err != nil {
				t.Fatal(err)
			}
			m = NewManagerWithPersistence(nil, nil, nil, dir)
			if m.StorageError() == nil || len(m.ListTasks()) != 0 {
				t.Fatal("incompatible/corrupt store exposed")
			}
		})
	}
}
