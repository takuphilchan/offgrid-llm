package agents

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTaskActivityAtomicReplayIsolationAndCompaction(t *testing.T) {
	dir := t.TempDir()
	m := NewManagerWithPersistence(nil, nil, nil, dir)
	if _, err := m.CreateTask("task", "private prompt", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := m.updateTask("task", func(task *Task) error { task.Actor = "alice"; return nil }); err != nil {
		t.Fatal(err)
	}
	replay, err := m.ReplayTask(context.Background(), "task", "alice", 0)
	if err != nil || len(replay.Events) != 2 || replay.Cursor == 0 {
		t.Fatalf("replay: %+v %v", replay, err)
	}
	cursor := replay.Cursor
	for _, actor := range []string{"bob", "", "local-admin"} {
		if _, err := m.ReplayTask(context.Background(), "task", actor, 0); !errors.Is(err, ErrTaskNotFound) {
			t.Fatalf("actor isolation %q: %v", actor, err)
		}
	}
	for i := 0; i < retainedTaskEvents+2; i++ {
		if _, err := m.updateTask("task", func(task *Task) error {
			task.Progress = &RunProgress{Phase: "generating", Iteration: i, Preview: "private generated text"}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	replay, err = m.ReplayTask(context.Background(), "task", "alice", cursor)
	if !errors.Is(err, ErrEventCursor) || !replay.Recovery || replay.Snapshot.Prompt != "private prompt" {
		t.Fatalf("missing recovery: %+v %v", replay, err)
	}
	last := replay.Cursor
	replay, err = m.ReplayTask(context.Background(), "task", "alice", last-2)
	if err != nil || len(replay.Events) != 2 || replay.Events[0].Sequence >= replay.Events[1].Sequence {
		t.Fatalf("ordered replay: %+v %v", replay, err)
	}
	payload, _ := json.Marshal(replay.Events)
	if strings.Contains(string(payload), "private") {
		t.Fatal("private content duplicated into activity metadata")
	}
	if _, err = m.ReplayTask(context.Background(), "task", "alice", last+1); !errors.Is(err, ErrEventCursor) {
		t.Fatalf("future cursor accepted: %v", err)
	}
	db, err := openTaskDatabase(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var count int
	if err = db.QueryRow(`SELECT COUNT(*) FROM agent_task_events`).Scan(&count); err != nil || count != retainedTaskEvents {
		t.Fatalf("unbounded history: %d %v", count, err)
	}
}

func createV1ActivityFixture(t *testing.T, dir string) {
	t.Helper()
	db, err := openTaskDatabase(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE agent_schema(singleton INTEGER PRIMARY KEY,version INTEGER NOT NULL,recovery_manifest BLOB NOT NULL);
INSERT INTO agent_schema VALUES(1,1,'{"original":"preserved"}');
CREATE TABLE agent_tasks(id TEXT PRIMARY KEY,actor TEXT NOT NULL,snapshot BLOB NOT NULL);
CREATE TABLE agent_task_events(sequence INTEGER PRIMARY KEY AUTOINCREMENT,task_id TEXT NOT NULL REFERENCES agent_tasks(id),status TEXT NOT NULL,recorded_at TEXT NOT NULL);
INSERT INTO agent_tasks VALUES('old','alice','{"id":"old","actor":"alice","status":"completed","result":"saved"}');
INSERT INTO agent_task_events(task_id,status,recorded_at) VALUES('old','completed','2026-09-21T00:00:00Z')`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestTaskActivityMigrationBacksUpAndPreservesCompletedWork(t *testing.T) {
	dir := t.TempDir()
	createV1ActivityFixture(t, dir)
	m := NewManagerWithPersistence(nil, nil, nil, dir)
	if err := m.StorageError(); err != nil {
		t.Fatal(err)
	}
	replay, err := m.ReplayTask(context.Background(), "old", "alice", 0)
	if !errors.Is(err, ErrEventCursor) || replay.Snapshot.Result != "saved" || replay.Cursor != 1 {
		t.Fatalf("migration recovery: %+v %v", replay, err)
	}
	db, err := openTaskDatabase(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var raw, original []byte
	if err = db.QueryRow(`SELECT manifest FROM agent_migrations WHERE version=2`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(`SELECT recovery_manifest FROM agent_schema`).Scan(&original); err != nil || string(original) != `{"original":"preserved"}` {
		t.Fatal("original manifest changed")
	}
	var manifest struct {
		Backup string `json:"backup"`
		SHA256 string `json:"sha256"`
	}
	if err = json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, manifest.Backup))
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	if hex.EncodeToString(digest[:]) != manifest.SHA256 {
		t.Fatal("backup digest mismatch")
	}
	// Original snapshot and status history remain available in the backup.
	backup, err := openTaskDatabase(filepath.Dir(filepath.Join(dir, manifest.Backup)))
	if err != nil {
		t.Fatal(err)
	}
	defer backup.Close()
	var version, count int
	if err = backup.QueryRow(`SELECT version FROM agent_schema`).Scan(&version); err != nil || version != 1 {
		t.Fatal("backup not original schema")
	}
	if err = backup.QueryRow(`SELECT COUNT(*) FROM agent_task_events`).Scan(&count); err != nil || count != 1 {
		t.Fatal("backup lost history")
	}
	if NewManagerWithPersistence(nil, nil, nil, dir).StorageError() != nil {
		t.Fatal("second activation failed")
	}
}

func TestTaskActivityMigrationBlocksCorruptOwnershipWithoutActivating(t *testing.T) {
	dir := t.TempDir()
	createV1ActivityFixture(t, dir)
	db, err := openTaskDatabase(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE agent_tasks SET actor='bob'`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if NewManagerWithPersistence(nil, nil, nil, dir).StorageError() == nil {
		t.Fatal("corruption activated")
	}
	db, err = openTaskDatabase(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var version int
	if err = db.QueryRow(`SELECT version FROM agent_schema`).Scan(&version); err != nil || version != 1 {
		t.Fatal("failed migration changed schema")
	}
}

func TestTaskActivityMigrationFailureRollsBackSchemaAndRetainsBackup(t *testing.T) {
	dir := t.TempDir()
	createV1ActivityFixture(t, dir)
	db, err := openTaskDatabase(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`CREATE TRIGGER fail_migration BEFORE DELETE ON agent_task_events BEGIN SELECT RAISE(ABORT,'injected migration write failure'); END`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if NewManagerWithPersistence(nil, nil, nil, dir).StorageError() == nil {
		t.Fatal("failed migration activated")
	}
	db, err = openTaskDatabase(dir)
	if err != nil {
		t.Fatal(err)
	}
	var version, count int
	if err = db.QueryRow(`SELECT version FROM agent_schema`).Scan(&version); err != nil || version != 1 {
		t.Fatal("schema advanced after failure")
	}
	if err = db.QueryRow(`SELECT COUNT(*) FROM agent_task_events`).Scan(&count); err != nil || count != 1 {
		t.Fatal("original events lost")
	}
	backups, err := filepath.Glob(filepath.Join(dir, "agent-schema-1-backup-*", taskDatabaseName))
	if err != nil || len(backups) != 1 {
		t.Fatal("recovery copy missing")
	}
	if _, err = db.Exec(`DROP TRIGGER fail_migration`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if err := NewManagerWithPersistence(nil, nil, nil, dir).StorageError(); err != nil {
		t.Fatalf("repair retry: %v", err)
	}
}

func TestCorruptActivityBlocksStartupAndAdmission(t *testing.T) {
	dir := t.TempDir()
	m := NewManagerWithPersistence(nil, nil, nil, dir)
	if _, err := m.CreateTask("task", "saved", nil); err != nil {
		t.Fatal(err)
	}
	db, err := openTaskDatabase(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE agent_task_events SET payload='{}'`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	m = NewManagerWithPersistence(nil, nil, nil, dir)
	if m.StorageError() == nil || len(m.ListTasks()) != 0 {
		t.Fatal("corrupt history silently accepted")
	}
	if _, err = m.CreateTask("another", "do not run", nil); err == nil {
		t.Fatal("admitted after corruption")
	}
}
