package agents

import (
	"encoding/json"
	"testing"
)

func TestConversationInboxSchemaUpgradePreservesExistingWork(t *testing.T) {
	m, r, scope, c := controlFixture(t)
	work := admitControlTestWork(t, r, scope, c)
	db, err := m.controlDatabase()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`DROP TABLE agent_conversation_inbox`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE agent_schema SET version=7`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	loaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err = loaded.StorageError(); err != nil {
		t.Fatal(err)
	}
	saved, err := loaded.WorkItem(scope, work.ID)
	if err != nil || saved.RunID != work.RunID || saved.Status != TaskInterrupted {
		t.Fatal(saved, err)
	}
	page, err := loaded.ConversationInbox(scope, c.ID, 0, 10)
	if err != nil || len(page.Items) != 0 {
		t.Fatal(page, err)
	}
	db, err = loaded.controlDatabase()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var version int
	if err = db.QueryRow(`SELECT version FROM agent_schema`).Scan(&version); err != nil || version != taskSchemaVersion {
		t.Fatal("older-runner fence missing", version, err)
	}
	var raw []byte
	if err = db.QueryRow(`SELECT manifest FROM agent_migrations WHERE version=?`, taskSchemaVersion).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		From   int    `json:"from"`
		To     int    `json:"to"`
		Backup string `json:"backup"`
	}
	if err = json.Unmarshal(raw, &manifest); err != nil || manifest.From != 7 || manifest.To != taskSchemaVersion || manifest.Backup == "" {
		t.Fatal(manifest, err)
	}
}
