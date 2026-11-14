package execution

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkingCopyFailedWritesRetainArtifactsAndReservations(t *testing.T) {
	for _, stage := range []string{"reserve", "ready", "release"} {
		t.Run(stage, func(t *testing.T) {
			e, store, b, root := workingCopyFixture(t)
			if err := os.WriteFile(filepath.Join(root, "source"), []byte("exact input"), 0600); err != nil {
				t.Fatal(err)
			}
			req := workingCopyRequest(b, "fault")
			query := `CREATE TRIGGER fail_copy BEFORE INSERT ON worker_copies BEGIN SELECT RAISE(ABORT,'injected'); END`
			if stage == "ready" {
				query = `CREATE TRIGGER fail_copy BEFORE UPDATE ON worker_copies WHEN NEW.state='ready' BEGIN SELECT RAISE(ABORT,'injected'); END`
			}
			if stage == "release" {
				query = `CREATE TRIGGER fail_copy BEFORE UPDATE ON worker_copies WHEN NEW.state='released' BEGIN SELECT RAISE(ABORT,'injected'); END`
			}
			if _, err := store.db.Exec(query); err != nil {
				t.Fatal(err)
			}
			prepared, err := e.PrepareWorker(context.Background(), req)
			if stage == "release" {
				if err != nil {
					t.Fatal(err)
				}
				err = prepared.Release(context.Background())
			}
			if err == nil {
				t.Fatal("fault did not reject preparation/settlement")
			}
			if _, err = store.db.Exec(`DROP TRIGGER fail_copy`); err != nil {
				t.Fatal(err)
			}
			var count int
			var reserved int64
			if err = store.db.QueryRow(`SELECT COUNT(*),COALESCE(SUM(reserved_bytes),0) FROM worker_copies`).Scan(&count, &reserved); err != nil {
				t.Fatal(err)
			}
			if stage == "reserve" {
				if count != 0 || reserved != 0 {
					t.Fatal("failed reservation committed")
				}
				entries, _ := os.ReadDir(store.directory)
				for _, entry := range entries {
					if validCopyID(entry.Name()) {
						t.Fatal("copied before reservation")
					}
				}
				return
			}
			if count != 1 || reserved != req.MaxBytes {
				t.Fatal("failed receipt released capacity", count, reserved)
			}
			var id string
			if err = store.db.QueryRow(`SELECT id FROM worker_copies`).Scan(&id); err != nil {
				t.Fatal(err)
			}
			if err = store.Close(); err != nil {
				t.Fatal(err)
			}
			reopened, err := OpenWorkingCopies(store.directory, store.policy)
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			record, err := reopened.Inspect(context.Background(), id, b)
			if err != nil || record.State != "uncertain" || record.ReservedBytes != req.MaxBytes {
				t.Fatal(record, err)
			}
			if _, err = reopened.Prepare(context.Background(), e.Worker.Session, req); !errors.Is(err, ErrConflict) {
				t.Fatal("restart retried preparation", err)
			}
			if err = reopened.Release(context.Background(), id, b); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestWorkingCopyReadOnlyNarrowingAndDirectoryIndependence(t *testing.T) {
	e, _, b, root := workingCopyFixture(t)
	if err := os.Mkdir(filepath.Join(root, "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	req := workingCopyRequest(b, "read")
	req.Permissions = Permissions{Read: true}
	child, err := e.PrepareWorker(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err = child.Endpoint.Worker.Session.Write(ctx, child.Context.Binding, "new", []byte("no"), ""); err == nil {
		t.Fatal("read-only worker wrote")
	}
	if _, err = child.Endpoint.Worker.Processes.Start(ctx, child.Context.Binding, ProcessSpec{Program: "go"}); err == nil {
		t.Fatal("read-only worker ran a command")
	}
	if _, err = child.Endpoint.Worker.Session.ChangeDirectory(ctx, child.Context.Binding, "nested"); err != nil {
		t.Fatal(err)
	}
	original, err := e.Check(ctx, b)
	if err != nil || original.Directory != "." {
		t.Fatal("worker changed parent cwd", original, err)
	}
}

func TestWorkingCopyGrowthBlocksFurtherAdmission(t *testing.T) {
	e, store, b, _ := workingCopyFixture(t)
	child, err := e.PrepareWorker(context.Background(), workingCopyRequest(b, "growth"))
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(child.Context.Root, "large"), []byte(strings.Repeat("x", 1500)), 0600); err != nil {
		t.Fatal(err)
	}
	measured, err := store.Measure(context.Background(), child.Record.ID, b)
	if err != nil || measured.MeasuredBytes != 1500 || measured.ReservedBytes != 1500 {
		t.Fatal(measured, err)
	}
	if _, err = e.PrepareWorker(context.Background(), workingCopyRequest(b, "no-room")); !errors.Is(err, ErrLimit) {
		t.Fatal("growth treated as free capacity", err)
	}
	if err = child.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = store.Measure(ctx, child.Record.ID, b); err == nil {
		t.Fatal("cancelled inventory claimed measurement")
	}
	saved, err := store.Inspect(context.Background(), child.Record.ID, b)
	if err != nil || saved.ReservedBytes < 1500 {
		t.Fatal(saved, err)
	}
}
