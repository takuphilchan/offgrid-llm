package execution

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkingCopyRestartRemeasuresUnrecordedGrowth(t *testing.T) {
	e, store, b, _ := workingCopyFixture(t)
	child, err := e.PrepareWorker(context.Background(), workingCopyRequest(b, "grow"))
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(child.Context.Root, "extra"), []byte(strings.Repeat("x", 1500)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = store.db.Exec(`CREATE TRIGGER reject_measurement BEFORE UPDATE ON worker_copies WHEN NEW.reserved_bytes>OLD.reserved_bytes BEGIN SELECT RAISE(ABORT,'injected'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Measure(context.Background(), child.Record.ID, b); err == nil {
		t.Fatal("lost measurement acknowledged")
	}
	if _, err = e.PrepareWorker(context.Background(), workingCopyRequest(b, "during-failure")); err == nil {
		t.Fatal("failed measurement reopened capacity")
	}
	if _, err = store.db.Exec(`DROP TRIGGER reject_measurement`); err != nil {
		t.Fatal(err)
	}
	if err = child.Close(); err != nil {
		t.Fatal(err)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	loaded, err := OpenWorkingCopies(store.directory, store.policy)
	if err != nil {
		t.Fatal(err)
	}
	defer loaded.Close()
	if _, err = loaded.Prepare(context.Background(), e.Worker.Session, workingCopyRequest(b, "after-restart")); !errors.Is(err, ErrLimit) {
		t.Fatal("restart reopened occupied capacity", err)
	}
	record, err := loaded.Inspect(context.Background(), child.Record.ID, b)
	if err != nil || record.MeasuredBytes != 1500 || record.ReservedBytes != 1500 {
		t.Fatal(record, err)
	}
}

func TestWorkingCopyUnknownGrowthBlocksAdmissionAndReleasePreservesLinkTarget(t *testing.T) {
	e, store, b, _ := workingCopyFixture(t)
	child, err := e.PrepareWorker(context.Background(), workingCopyRequest(b, "link-growth"))
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "preserve")
	if err = os.WriteFile(target, []byte("outside preserved"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(target, filepath.Join(child.Context.Root, "unsupported")); err != nil {
		t.Skip("disposable symlink unavailable:", err)
	}
	if _, err = store.Measure(context.Background(), child.Record.ID, b); err == nil {
		t.Fatal("link measured as empty regular file")
	}
	record, err := store.Inspect(context.Background(), child.Record.ID, b)
	if err != nil || record.State != "uncertain" || record.ReservedBytes < store.policy.MaxTotalBytes {
		t.Fatal(record, err)
	}
	if _, err = e.PrepareWorker(context.Background(), workingCopyRequest(b, "blocked")); !errors.Is(err, ErrLimit) {
		t.Fatal("unknown storage allowed new preparation", err)
	}
	if err = child.Release(context.Background()); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "outside preserved" {
		t.Fatal("cleanup followed a link outside its owned root", string(data), err)
	}
}
