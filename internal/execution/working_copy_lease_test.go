package execution

import (
	"context"
	"errors"
	"os"
	"testing"
)

func TestWorkingCopyCleanupRequiresRetainedWorkerSettlement(t *testing.T) {
	endpoint, store, binding, _ := workingCopyFixture(t)
	ctx := context.Background()
	child, err := endpoint.PrepareWorker(ctx, workingCopyRequest(binding, "lease"))
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Release(ctx, child.Record.ID, binding); !errors.Is(err, ErrHostUnavailable) {
		t.Fatal("live worker admitted cleanup", err)
	}
	owner, err := store.copyOwnership(ctx, child.Record.ID)
	if err != nil || !owner.InUse || owner.Quarantine != "" {
		t.Fatal(owner, err)
	}
	if _, err = os.Stat(child.Context.Root); err != nil {
		t.Fatal(err)
	}
	if err = child.Close(); err != nil {
		t.Fatal(err)
	}
	err = child.Release(ctx)
	if assertDarwinCleanupRefusal(t, store, binding, child.Record, err) {
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if err = child.Close(); err != nil {
		t.Fatal("settled owner was not idempotent", err)
	}
}

func TestWorkingCopySettlementWriteRequiresSameClosedOwnerRetry(t *testing.T) {
	endpoint, store, binding, _ := workingCopyFixture(t)
	ctx := context.Background()
	child, err := endpoint.PrepareWorker(ctx, workingCopyRequest(binding, "settlement-write"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.db.Exec(`CREATE TRIGGER reject_settlement BEFORE UPDATE OF in_use ON worker_copy_ownership WHEN NEW.in_use=0 BEGIN SELECT RAISE(FAIL,'settlement receipt unavailable'); END`); err != nil {
		t.Fatal(err)
	}
	if err = child.Close(); err == nil {
		t.Fatal("lost settlement write reported success")
	}
	record, err := store.Inspect(ctx, child.Record.ID, binding)
	if err != nil || record.State != "uncertain" || record.ReservedBytes != child.Record.ReservedBytes {
		t.Fatal("failed close did not preserve an uncertain reservation", record, err)
	}
	if err = store.Release(ctx, child.Record.ID, binding); !errors.Is(err, ErrHostUnavailable) {
		t.Fatal(err)
	}
	if _, err = store.db.Exec(`DROP TRIGGER reject_settlement`); err != nil {
		t.Fatal(err)
	}
	if err = child.Close(); err != nil {
		t.Fatal("retained closed owner could not retry its proof", err)
	}
	err = store.Release(ctx, child.Record.ID, binding)
	if assertDarwinCleanupRefusal(t, store, binding, child.Record, err) {
		return
	}
	if err != nil {
		t.Fatal(err)
	}
}

func TestWorkingCopyUncertainCloseRetriesFailedStateWriteWithoutReleasing(t *testing.T) {
	endpoint, store, binding, _ := workingCopyFixture(t)
	ctx := context.Background()
	child, err := endpoint.PrepareWorker(ctx, workingCopyRequest(binding, "uncertain-close"))
	if err != nil {
		t.Fatal(err)
	}
	// Model the manager's retained uncertainty after losing native ownership.
	// The Linux escape fixture separately exercises the real process cause.
	child.Endpoint.Worker.Processes.cleanupUnproven = true
	if _, err = store.db.Exec(`CREATE TRIGGER ignore_uncertainty BEFORE UPDATE OF state ON worker_copies WHEN NEW.state='uncertain' BEGIN SELECT RAISE(IGNORE); END`); err != nil {
		t.Fatal(err)
	}
	if err = child.Close(); !errors.Is(err, ErrHostUnavailable) || !errors.Is(err, ErrRevision) {
		t.Fatal("failed uncertainty write disappeared", err)
	}
	if _, err = store.db.Exec(`DROP TRIGGER ignore_uncertainty`); err != nil {
		t.Fatal(err)
	}
	if err = child.Close(); !errors.Is(err, ErrHostUnavailable) || errors.Is(err, ErrRevision) {
		t.Fatal("retry changed cleanup proof or did not save uncertainty", err)
	}
	record, err := store.Inspect(ctx, child.Record.ID, binding)
	if err != nil || record.State != "uncertain" || record.ReservedBytes != child.Record.ReservedBytes {
		t.Fatal(record, err)
	}
	owner, err := store.copyOwnership(ctx, child.Record.ID)
	if err != nil || !owner.InUse || owner.Quarantine != "" {
		t.Fatal("uncertain close released ownership", owner, err)
	}
	if err = child.Release(ctx); !errors.Is(err, ErrHostUnavailable) {
		t.Fatal("uncertain close authorized removal", err)
	}
	if _, err = os.Stat(child.Context.Root); err != nil {
		t.Fatal("uncertain close removed data", err)
	}
}

func TestWorkingCopyRestartCannotInventSettlement(t *testing.T) {
	endpoint, store, binding, _ := workingCopyFixture(t)
	ctx := context.Background()
	child, err := endpoint.PrepareWorker(ctx, workingCopyRequest(binding, "lost-supervisor"))
	if err != nil {
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
	if err = reopened.Release(ctx, child.Record.ID, binding); !errors.Is(err, ErrHostUnavailable) {
		t.Fatal("restart cleared worker ownership", err)
	}
	if _, err = os.Stat(child.Context.Root); err != nil {
		t.Fatal(err)
	}
	record, err := reopened.Inspect(ctx, child.Record.ID, binding)
	if err != nil || record.ReservedBytes != child.Record.ReservedBytes || record.State != "uncertain" {
		t.Fatal(record, err)
	}
	if err = child.Close(); err == nil {
		t.Fatal("old owner settled a closed database")
	}
}

func TestWorkingCopyV11MigrationRetainsUnprovenLease(t *testing.T) {
	endpoint, store, binding, _ := workingCopyFixture(t)
	ctx := context.Background()
	child, err := endpoint.PrepareWorker(ctx, workingCopyRequest(binding, "old-identity"))
	if err != nil {
		t.Fatal(err)
	}
	if err = child.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = store.db.Exec(`BEGIN; ALTER TABLE worker_copy_ownership RENAME TO ownership_v12; CREATE TABLE worker_copy_ownership(copy_id TEXT PRIMARY KEY REFERENCES worker_copies(id),version INTEGER NOT NULL,identity TEXT NOT NULL,empty INTEGER NOT NULL); INSERT INTO worker_copy_ownership SELECT copy_id,version,identity,empty FROM ownership_v12; DROP TABLE ownership_v12; PRAGMA user_version=11; COMMIT;`); err != nil {
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
	owner, err := reopened.copyOwnership(ctx, child.Record.ID)
	if err != nil || !owner.InUse || owner.Identity == "" {
		t.Fatal(owner, err)
	}
	if err = reopened.Release(ctx, child.Record.ID, binding); !errors.Is(err, ErrHostUnavailable) {
		t.Fatal("old native identity became settlement proof", err)
	}
	if _, err = reopened.Manifest(ctx, child.Record.ID, binding); err != nil {
		t.Fatal(err)
	}
}
