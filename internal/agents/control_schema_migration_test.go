package agents

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Build an actual v6 constraint around populated control records, not just a
// version-number change. All original child-table references remain in place.
func restoreV6RecordConstraint(t *testing.T, db *sql.DB) {
	t.Helper()
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	for _, statement := range []string{
		`PRAGMA foreign_keys=OFF`, `BEGIN`,
		`CREATE TABLE agent_control_records_v6 ` + strings.Replace(controlRecordColumns, ",'user_preferences'", "", 1),
		`INSERT INTO agent_control_records_v6 SELECT * FROM agent_control_records`,
		`DROP TABLE agent_control_records`,
		`ALTER TABLE agent_control_records_v6 RENAME TO agent_control_records`,
		`UPDATE agent_schema SET version=6`, `COMMIT`, `PRAGMA foreign_keys=ON`,
	} {
		if _, err = conn.ExecContext(context.Background(), statement); err != nil {
			t.Fatal(err)
		}
	}
}

func TestUserPreferencesSchemaUpgradeRetainsCanonicalGraph(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "upgrade", true: "rollback"}[fail], func(t *testing.T) {
			m, r, scope, root, child := workerFixture(t)
			worker, _ := m.WorkerSession(scope, child.Worker.SessionID)
			message := api.WorkerCommand{RequestID: "before-upgrade", TargetID: worker.ID, ExpectedRevision: worker.Revision, Kind: "message", Payload: api.WorkerCommandPayload{Content: "Retain pending input"}}
			if _, err := r.CommandWorker(scope, message); err != nil {
				t.Fatal(err)
			}
			db, err := m.controlDatabase()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			restoreV6RecordConstraint(t, db)
			if _, err = db.Exec(`CREATE INDEX retained_control_index ON agent_control_records(kind)`); err != nil {
				t.Fatal(err)
			}
			if fail {
				if _, err = db.Exec(`CREATE TRIGGER reject_upgrade BEFORE INSERT ON agent_migrations BEGIN SELECT RAISE(ABORT,'injected'); END`); err != nil {
					t.Fatal(err)
				}
			}
			err = migrateTaskVersion(db, m.dataDir, 6, taskSchemaVersion)
			if (err != nil) != fail {
				t.Fatal("wrong migration outcome", err)
			}
			var version, enabled, indexes int
			if err = db.QueryRow(`SELECT version FROM agent_schema`).Scan(&version); err != nil {
				t.Fatal(err)
			}
			if fail && version != 6 || !fail && version != taskSchemaVersion {
				t.Fatal("wrong version", version)
			}
			if err = db.QueryRow(`PRAGMA foreign_keys`).Scan(&enabled); err != nil || enabled != 1 {
				t.Fatal("foreign keys left disabled", enabled, err)
			}
			if err = db.QueryRow(`SELECT count(*) FROM sqlite_schema WHERE name='retained_control_index'`).Scan(&indexes); err != nil || indexes != 1 {
				t.Fatal("index lost", indexes, err)
			}
			var definition string
			if err = db.QueryRow(`SELECT sql FROM sqlite_schema WHERE name='agent_control_records'`).Scan(&definition); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(definition, "'user_preferences'") == fail {
				t.Fatal("wrong constraint after migration", definition)
			}
			if fail {
				return
			}
			var raw []byte
			if err = db.QueryRow(`SELECT manifest FROM agent_migrations WHERE version=?`, taskSchemaVersion).Scan(&raw); err != nil {
				t.Fatal(err)
			}
			var manifest struct {
				Backup string `json:"backup"`
			}
			if err = json.Unmarshal(raw, &manifest); err != nil {
				t.Fatal(err)
			}
			if _, err = os.Stat(filepath.Join(m.dataDir, manifest.Backup)); err != nil {
				t.Fatal(err)
			}
			loaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
			if err = loaded.StorageError(); err != nil {
				t.Fatal(err)
			}
			work, err := loaded.WorkItem(scope, root.WorkID)
			if err != nil || work.RunID != root.ID {
				t.Fatal(work, err)
			}
			restored, err := loaded.WorkerSession(scope, worker.ID)
			if err != nil || restored.CurrentRunID != child.ID {
				t.Fatal(restored, err)
			}
			mail, err := loaded.WorkerMessages(scope, worker.ID, 0, 10)
			if err != nil || len(mail.Items) != 1 || mail.Items[0].Content != message.Payload.Content {
				t.Fatal(mail, err)
			}
			if receipt, err := loaded.WorkerCommandReceipt(scope, message); err != nil || receipt.Message == nil {
				t.Fatal(receipt, err)
			}
			if _, err = loaded.ConfigureUserPreferences(scope, api.UserPreferencesCommand{RequestID: "after-upgrade", TargetID: UserPreferencesID, Kind: "configure_user_preferences"}); err != nil {
				t.Fatal(err)
			}
			if _, err = db.Exec(`INSERT INTO agent_control_links VALUES('missing','missing','missing','missing')`); err == nil {
				t.Fatal("foreign-key enforcement lost")
			}
		})
	}
}
