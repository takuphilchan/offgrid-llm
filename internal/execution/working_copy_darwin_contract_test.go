//go:build darwin

package execution

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

// Darwin currently has neither qualified no-replace quarantine nor a native
// descendant owner. These tests protect the explicit uncertainty contract;
// they do not establish completed worker support on this platform.
func TestDarwinUnsupportedQuarantineRetainsCopyAndReservation(t *testing.T) {
	endpoint, store, binding, _ := workingCopyFixture(t)
	ctx := context.Background()
	child, err := endpoint.PrepareWorker(ctx, workingCopyRequest(binding, "darwin-quarantine"))
	if err != nil {
		t.Fatal(err)
	}
	if err = child.Close(); err != nil {
		t.Fatal("an unused worker should close without process uncertainty", err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		if err = store.Release(ctx, child.Record.ID, binding); !errors.Is(err, ErrUnsupported) {
			t.Fatal("unsupported quarantine reported successful deletion", err)
		}
		record, err := store.Inspect(ctx, child.Record.ID, binding)
		if err != nil || record.State != "uncertain" || record.ReservedBytes != child.Record.ReservedBytes || record.ReservedBytes == 0 {
			t.Fatal("unsupported cleanup released capacity", record, err)
		}
		if _, err = os.Stat(child.Context.Root); err != nil {
			t.Fatal("unsupported cleanup removed the retained copy", err)
		}
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenWorkingCopies(store.directory, store.policy)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if !assertDarwinCleanupRefusal(t, reopened, binding, child.Record, reopened.Release(ctx, child.Record.ID, binding)) {
		t.Fatal("restart replayed or invented successful cleanup")
	}
}

func TestDarwinLeaderExitCannotReleaseWorkerLease(t *testing.T) {
	endpoint, store, binding, _ := workingCopyFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req := workingCopyRequest(binding, "darwin-leader")
	req.Permissions.Processes = true
	child, err := endpoint.PrepareWorker(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	m := child.Endpoint.Worker.Processes
	p, err := m.Start(ctx, child.Context.Binding, testCommand("exit 0"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := m.Wait(ctx, child.Context.Binding, p.Handle)
	if err != nil || result.State != "exited" || result.ExitCode == nil || *result.ExitCode != 0 {
		t.Fatal("leader outcome lost", result, err)
	}
	if ProcessDescendantsSettled(result) || ProcessDescendantState(result) != "unknown" || result.Runtime == nil || result.Runtime.Settled {
		t.Fatal("process group exit invented descendant retirement", result)
	}
	if err = child.Close(); !errors.Is(err, ErrHostUnavailable) {
		t.Fatal("unverified descendants allowed worker close", err)
	}
	if err = store.Release(ctx, child.Record.ID, binding); !errors.Is(err, ErrHostUnavailable) {
		t.Fatal("unverified descendants allowed copy release", err)
	}
	owner, err := store.copyOwnership(ctx, child.Record.ID)
	if err != nil || !owner.InUse || owner.Detached || owner.Quarantine != "" {
		t.Fatal("unverified descendants lost their durable lease", owner, err)
	}
	record, err := store.Inspect(ctx, child.Record.ID, binding)
	if err != nil || record.State != "uncertain" || record.ReservedBytes != child.Record.ReservedBytes {
		t.Fatal("unverified descendants released capacity", record, err)
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
		t.Fatal("restart converted unproven descendants to cleanup authority", err)
	}
	after, err := reopened.copyOwnership(ctx, child.Record.ID)
	if err != nil || after != owner {
		t.Fatal("restart replaced the unresolved ownership lease", after, owner, err)
	}
}
