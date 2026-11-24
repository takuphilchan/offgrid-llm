package execution

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestWorkingCopyCleanupPreservesRecreatedPathAfterLostReceipt(t *testing.T) {
	endpoint, store, binding, _ := workingCopyFixture(t)
	child, err := endpoint.PrepareWorker(context.Background(), workingCopyRequest(binding, "lost-cleanup-receipt"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.db.Exec(`CREATE TRIGGER reject_release BEFORE UPDATE ON worker_copies WHEN NEW.state='released' BEGIN SELECT RAISE(ABORT,'injected lost cleanup receipt'); END`); err != nil {
		t.Fatal(err)
	}
	err = child.Release(context.Background())
	if assertDarwinCleanupRefusal(t, store, binding, child.Record, err) {
		return
	}
	if err == nil {
		t.Fatal("cleanup receipt failure was acknowledged")
	}
	if _, err = store.db.Exec(`DROP TRIGGER reject_release`); err != nil {
		t.Fatal(err)
	}
	recreated := filepath.Join(store.directory, child.Record.ID)
	if err = os.Mkdir(recreated, 0700); err != nil {
		t.Fatal("fixture expected original cleanup to remove its directory:", err)
	}
	sentinel := filepath.Join(recreated, "unrelated-preserve.txt")
	if err = os.WriteFile(sentinel, []byte("not owned by the earlier preparation"), 0600); err != nil {
		t.Fatal(err)
	}
	err = store.Release(context.Background(), child.Record.ID, binding)
	content, readErr := os.ReadFile(sentinel)
	if err == nil || readErr != nil || string(content) != "not owned by the earlier preparation" {
		t.Fatalf("explicit retry must preserve the recreated directory: release=%v read=%v content=%q", err, readErr, content)
	}
	record, err := store.Inspect(context.Background(), child.Record.ID, binding)
	if err != nil || record.State != "uncertain" || record.ReservedBytes < child.Record.ReservedBytes {
		t.Fatal("uncertain cleanup released ownership/capacity", record, err)
	}
}
