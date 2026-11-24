package execution

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func environmentRequest(w *Worker, b Binding, id string) Request {
	return request(w, b, "process_start", id, ProcessSpec{Program: os.Args[0], Arguments: []string{"-test.run=^TestProcessEnvironmentFixture$", "--", "offgrid-environment-fixture"}})
}

func TestProcessManifestPrecedesNativeDispatchAndOmitsValues(t *testing.T) {
	t.Setenv("LANG", "private-environment-value")
	w, binding, _ := workerFixture(t)
	w.Session.config.Permissions.Processes = true
	w.AuthorizeProcess = func(context.Context, Binding, ProcessSpec, bool) error { return nil }
	r := environmentRequest(w, binding, "manifest")
	response := w.Handle(context.Background(), r)
	if response.Error != "" || response.Receipt == nil || response.Receipt.State != "dispatched" {
		t.Fatal(response)
	}
	w.finishing.Wait()
	var raw, hash string
	if err := w.Journal.db.QueryRow(`SELECT manifest,digest FROM execution_process_manifests WHERE operation=?`, r.OperationID).Scan(&raw, &hash); err != nil {
		t.Fatal(err)
	}
	var manifest processManifest
	if err := json.Unmarshal([]byte(raw), &manifest); err != nil || hash != digest([]byte(raw)) || manifest.Binding != binding || manifest.RequestDigest != digest(r.Payload()) || manifest.Program != os.Args[0] || !reflect.DeepEqual(manifest.Environment, w.Session.environment.descriptor()) {
		t.Fatal(manifest, err)
	}
	if strings.Contains(raw, "private-environment-value") || strings.Contains(raw, w.Token) {
		t.Fatal("private environment/token was stored in manifest")
	}
}

func TestProcessManifestPublicationFailuresPreventDispatch(t *testing.T) {
	for name, trigger := range map[string]string{
		"ignore":      `CREATE TRIGGER fault BEFORE INSERT ON execution_process_manifests BEGIN SELECT RAISE(IGNORE); END`,
		"replace":     `CREATE TRIGGER fault AFTER INSERT ON execution_process_manifests BEGIN UPDATE execution_process_manifests SET manifest='{}' WHERE operation=NEW.operation; END`,
		"environment": `CREATE TRIGGER fault AFTER INSERT ON execution_process_manifests BEGIN UPDATE execution_environments SET descriptor='{}'; END`,
		"claim":       `CREATE TRIGGER fault AFTER INSERT ON execution_process_manifests BEGIN UPDATE execution_operations SET state='complete' WHERE id=NEW.operation; END`,
	} {
		t.Run(name, func(t *testing.T) {
			w, binding, _ := workerFixture(t)
			w.Session.config.Permissions.Processes = true
			w.AuthorizeProcess = func(context.Context, Binding, ProcessSpec, bool) error { return nil }
			if _, err := w.Journal.db.Exec(trigger); err != nil {
				t.Fatal(err)
			}
			response := w.Handle(context.Background(), environmentRequest(w, binding, "failed-manifest"))
			if response.Error != "" || response.Receipt == nil || response.Receipt.State != "rejected" || len(w.Processes.processes) != 0 {
				t.Fatal("unpublished manifest dispatched native work", response)
			}
			var count int
			if err := w.Journal.db.QueryRow(`SELECT COUNT(*) FROM execution_process_manifests`).Scan(&count); err != nil || count != 0 {
				t.Fatal("failed transaction retained partial evidence", count, err)
			}
		})
	}
}

func TestProcessManifestRejectsChangedSessionEnvironment(t *testing.T) {
	w, binding, root := workerFixture(t)
	r := environmentRequest(w, binding, "original-environment")
	if _, fresh, err := w.Journal.Claim(context.Background(), r.OperationID, r.Payload()); err != nil || !fresh {
		t.Fatal(fresh, err)
	}
	environment := w.Session.environment.descriptor()
	if err := w.Journal.recordProcessManifest(context.Background(), r, root, os.Args[0], environment); err != nil {
		t.Fatal(err)
	}
	r = environmentRequest(w, binding, "changed-environment")
	if _, fresh, err := w.Journal.Claim(context.Background(), r.OperationID, r.Payload()); err != nil || !fresh {
		t.Fatal(fresh, err)
	}
	environment.Digest = strings.Repeat("a", 64)
	if err := w.Journal.recordProcessManifest(context.Background(), r, root, os.Args[0], environment); !errors.Is(err, ErrRevision) {
		t.Fatal("same session adopted different environment", err)
	}
}

func TestProcessManifestJournalV3UpgradeRetainsUncertainReceipt(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "journal")
	journal, err := OpenJournal(directory)
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte("legacy pending work")
	if _, fresh, err := journal.Claim(context.Background(), "legacy", payload); err != nil || !fresh {
		t.Fatal(fresh, err)
	}
	if _, err := journal.db.Exec(`BEGIN; DROP TABLE execution_process_manifests; DROP TABLE execution_environments; PRAGMA user_version=3; COMMIT;`); err != nil {
		t.Fatal(err)
	}
	if err := journal.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenJournal(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if receipt, fresh, err := reopened.Claim(context.Background(), "legacy", payload); err != nil || fresh || receipt.State != "uncertain" {
		t.Fatal("migration replayed pending work", receipt, fresh, err)
	}
	var version, count int
	if err := reopened.db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil || version != 4 {
		t.Fatal(version, err)
	}
	if err := reopened.db.QueryRow(`SELECT COUNT(*) FROM execution_process_manifests`).Scan(&count); err != nil || count != 0 {
		t.Fatal("migration invented legacy command evidence", count, err)
	}
}
