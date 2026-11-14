package agents

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCorrectionSchemaUpgradeRetainsBackupAndVersionFence(t *testing.T) {
	directory := t.TempDir()
	m := NewManagerWithPersistence(nil, nil, nil, directory)
	r := NewRunner(m, NewToolRegistry(), nil)
	task, err := r.Create("Retain this history", "model", "alice", taskConfig())
	if err != nil {
		t.Fatal(err)
	}
	db, err := openTaskDatabase(directory)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE agent_schema SET version=4`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	reloaded := NewManagerWithPersistence(nil, nil, nil, directory)
	if err := reloaded.StorageError(); err != nil {
		t.Fatal(err)
	}
	if restored, exists := reloaded.GetTask(task.ID); !exists || restored.Prompt != task.Prompt {
		t.Fatal("upgrade lost existing task")
	}
	db, err = openTaskDatabase(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var version int
	var manifest []byte
	if err = db.QueryRow(`SELECT version FROM agent_schema`).Scan(&version); err != nil || version != taskSchemaVersion {
		t.Fatal("missing older-binary fence", version, err)
	}
	if err = db.QueryRow(`SELECT manifest FROM agent_migrations WHERE version=?`, taskSchemaVersion).Scan(&manifest); err != nil {
		t.Fatal(err)
	}
	var record struct {
		Backup string `json:"backup"`
		From   int    `json:"from"`
		To     int    `json:"to"`
	}
	if err = json.Unmarshal(manifest, &record); err != nil {
		t.Fatal(err)
	}
	if record.From != 4 || record.To != taskSchemaVersion {
		t.Fatal("incorrect migration provenance")
	}
	if _, err = os.Stat(filepath.Join(directory, record.Backup)); err != nil {
		t.Fatal("missing retained migration backup", err)
	}
}
