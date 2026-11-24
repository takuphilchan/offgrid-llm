package execution

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteRetentionReservationSurvivesRestart(t *testing.T) {
	s, b, root := fixture(t)
	put(t, root, "input.txt", "before")
	private := t.TempDir()
	j, err := OpenJournal(private)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, fresh, err := j.Claim(ctx, "unsettled", []byte("unsettled")); err != nil || !fresh {
		t.Fatal(err)
	}
	if _, err = j.db.Exec(`INSERT INTO execution_replacements(operation,directory,target,expected,proposed,reserved_bytes,state) VALUES('unsettled','old','input.txt','old','new',?,'dispatching')`, maxReplacementStoreBytes); err != nil {
		t.Fatal(err)
	}
	if err = j.Close(); err != nil {
		t.Fatal(err)
	}
	j, err = OpenJournal(private)
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	if _, fresh, err := j.Claim(ctx, "new", []byte("new")); err != nil || !fresh {
		t.Fatal(err)
	}
	if _, err = s.write(ctx, b, "input.txt", []byte("proposed"), digest([]byte("before")), j, "new"); !errors.Is(err, ErrLimit) {
		t.Fatal("unknown reservation refunded", err)
	}
	data, _ := os.ReadFile(filepath.Join(root, "input.txt"))
	if string(data) != "before" {
		t.Fatal("quota rejection mutated source")
	}
	if _, err = os.Stat(filepath.Join(private, "replacements")); !os.IsNotExist(err) {
		t.Fatal("copied before storage admission", err)
	}
}

func TestWriteJournalUpgradePreservesReceiptsAndRejectsFutureSchema(t *testing.T) {
	directory := t.TempDir()
	j, err := OpenJournal(directory)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, fresh, err := j.Claim(ctx, "legacy", []byte("payload")); err != nil || !fresh {
		t.Fatal(err)
	}
	if err = j.Finish(ctx, "legacy", []byte("payload"), Receipt{State: "complete", Result: []byte(`{"saved":true}`)}); err != nil {
		t.Fatal(err)
	}
	if _, err = j.db.Exec(`DROP TABLE execution_replacements; PRAGMA user_version=2;`); err != nil {
		t.Fatal(err)
	}
	if err = j.Close(); err != nil {
		t.Fatal(err)
	}
	j, err = OpenJournal(directory)
	if err != nil {
		t.Fatal(err)
	}
	receipt, fresh, err := j.Claim(ctx, "legacy", []byte("payload"))
	if err != nil || fresh || receipt.State != "complete" || string(receipt.Result) != `{"saved":true}` {
		t.Fatal(receipt, fresh, err)
	}
	if _, err = j.db.Exec(`PRAGMA user_version=5`); err != nil {
		t.Fatal(err)
	}
	if err = j.Close(); err != nil {
		t.Fatal(err)
	}
	if future, err := OpenJournal(directory); !errors.Is(err, ErrUnsupported) {
		if future != nil {
			future.Close()
		}
		t.Fatal("future schema was accepted", err)
	}
}

func TestReplacementRecoveryObservationDoesNotDispatch(t *testing.T) {
	w, b, root := workerFixture(t)
	put(t, root, "input.txt", "before")
	r := request(w, b, "file_write", "unknown-write", map[string]any{"path": "input.txt", "data": []byte("proposed"), "revision": digest([]byte("before"))})
	ctx := context.Background()
	if _, fresh, err := w.Journal.Claim(ctx, r.OperationID, r.Payload()); err != nil || !fresh {
		t.Fatal(err)
	}
	id := digest([]byte("execution-replacement-v1\x00" + r.OperationID))
	if _, err := w.Journal.db.Exec(`INSERT INTO execution_replacements(operation,directory,target,expected,proposed,reserved_bytes,state) VALUES(?,?,?,?,?,100,'dispatching')`, r.OperationID, id, "input.txt", digest([]byte("before")), digest([]byte("proposed"))); err != nil {
		t.Fatal(err)
	}
	receipt, err := w.Journal.Inspect(ctx, r)
	if err != nil || receipt.State != "uncertain" || len(receipt.Result) == 0 {
		t.Fatal(receipt, err)
	}
	changed := r
	changed.Binding.Actor = "foreign"
	if _, err = w.Journal.Inspect(ctx, changed); !errors.Is(err, ErrConflict) {
		t.Fatal("foreign payload read recovery", err)
	}
	data, _ := os.ReadFile(filepath.Join(root, "input.txt"))
	if string(data) != "before" {
		t.Fatal("inspection dispatched")
	}
}
