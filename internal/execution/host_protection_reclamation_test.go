package execution

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"
)

// Seed the capacity edge using the same durable reserved -> released record
// transition. Actual admission/release calls exercise the boundary afterwards;
// this fixture avoids thousands of independent SQLite open/fsync operations.
func protectionFillReleased(t *testing.T, registry *HostProtectionRegistry, exemplar hostProtectionRecord, total int) {
	t.Helper()
	err := registry.transaction(context.Background(), func(store *hostProtectionTransaction) error {
		for index := len(store.records); index < total; index++ {
			row := exemplar
			row.ID = protectionHash(fmt.Sprintf("settled-fixture-%d", index))
			binding := *exemplar.Binding
			binding.Session = fmt.Sprintf("settled-fixture-%d", index)
			row.Binding = &binding
			row.State = "reserved"
			if err := store.insert(context.Background(), row); err != nil {
				return err
			}
			row.State = "released"
			if err := store.replace(context.Background(), row); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestHostProtectionReleasedCapacityAllowsNewExecution(t *testing.T) {
	ctx := context.Background()
	registry := protectionTestRegistry(t)
	root, journal := protectionTestRoot(t), protectionTestRoot(t)
	first, err := registry.ReserveExecution(ctx, protectionTestConfig(root, "first"), journal)
	if err != nil {
		t.Fatal(err)
	}
	if err = first.releaseVerified(ctx); err != nil {
		t.Fatal(err)
	}
	protectionFillReleased(t, registry, first.record, hostProtectionMaxRecords)
	next, err := registry.ReserveExecution(ctx, protectionTestConfig(root, "next"), journal)
	if err != nil {
		t.Fatalf("verified closed executions permanently exhausted registry: %v", err)
	}
	if err = next.releaseVerified(ctx); err != nil {
		t.Fatal(err)
	}
	if err = first.releaseVerified(ctx); err != nil {
		t.Fatalf("original settled capability lost idempotence: %v", err)
	}
	if _, err = registry.ReserveExecution(ctx, protectionTestConfig(root, "first"), journal); err != nil {
		t.Fatal(err)
	}
}

func TestHostProtectionReservedCapacityRemainsClosed(t *testing.T) {
	ctx := context.Background()
	registry := protectionTestRegistry(t)
	root, journal := protectionTestRoot(t), protectionTestRoot(t)
	first, err := registry.ReserveExecution(ctx, protectionTestConfig(root, "first"), journal)
	if err != nil {
		t.Fatal(err)
	}
	err = registry.transaction(ctx, func(store *hostProtectionTransaction) error {
		for index := 1; index < hostProtectionMaxRecords; index++ {
			row := first.record
			row.ID = protectionHash(fmt.Sprintf("unsettled-%d", index))
			binding := *row.Binding
			binding.Session = fmt.Sprintf("unsettled-%d", index)
			row.Binding = &binding
			if err := store.insert(ctx, row); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = registry.ReserveExecution(ctx, protectionTestConfig(root, "next"), journal); !errors.Is(err, ErrHostProtectionLimit) {
		t.Fatalf("active rows reclaimed: %v", err)
	}
}

func protectionRows(t *testing.T, registry *HostProtectionRegistry) map[string]hostProtectionRecord {
	t.Helper()
	var result map[string]hostProtectionRecord
	if err := registry.transaction(context.Background(), func(store *hostProtectionTransaction) error {
		result = store.records
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestHostProtectionReclamationKeepsUnsettledAndBrowserRows(t *testing.T) {
	ctx := context.Background()
	registry := protectionTestRegistry(t)
	root, journal := protectionTestRoot(t), protectionTestRoot(t)
	old, err := registry.ReserveExecution(ctx, protectionTestConfig(root, "settled"), journal)
	if err != nil {
		t.Fatal(err)
	}
	if err = old.releaseVerified(ctx); err != nil {
		t.Fatal(err)
	}
	active, err := registry.ReserveExecution(ctx, protectionTestConfig(root, "unsettled"), journal)
	if err != nil {
		t.Fatal(err)
	}
	protected, err := registry.RegisterBrowserRoot(ctx, protectionTestRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	pending, err := registry.RegisterBrowserRoot(ctx, filepath.Join(protectionTestRoot(t), "not-created"))
	if err != nil {
		t.Fatal(err)
	}
	protectionFillReleased(t, registry, old.record, hostProtectionMaxRecords)
	before := protectionRows(t, registry)
	if _, err = registry.RegisterBrowserRoot(ctx, filepath.Join(root, "conflicting-browser")); !errors.Is(err, ErrHostProtectionConflict) {
		t.Fatalf("conflicting admission was accepted: %v", err)
	}
	if got := protectionRows(t, registry); !reflect.DeepEqual(got, before) {
		t.Fatal("rejected admission reclaimed history")
	}
	// Browser admission must be able to reclaim settled executions too.
	if _, err = registry.RegisterBrowserRoot(ctx, protectionTestRoot(t)); err != nil {
		t.Fatal(err)
	}
	after := protectionRows(t, registry)
	if _, exists := after[old.ID()]; exists {
		t.Fatal("oldest settled admission was retained")
	}
	for _, id := range []string{active.ID(), protected.ID(), pending.ID()} {
		if !reflect.DeepEqual(before[id], after[id]) {
			t.Fatalf("unsettled/browser row changed: %s", id)
		}
	}
	released := 0
	for _, row := range after {
		if row.State == "released" {
			released++
		}
	}
	if released != hostProtectionReleasedHistory {
		t.Fatalf("unbounded settled history: %d", released)
	}
	if err = old.releaseVerified(ctx); err != nil {
		t.Fatal(err)
	}
	if err = old.CancelUnactivated(ctx); err != nil {
		t.Fatal(err)
	}
	if got := protectionRows(t, registry); !reflect.DeepEqual(got, after) {
		t.Fatal("settled retry changed retained state")
	}
	// Knowing a retired row's token or public ID cannot release another row.
	forged := &HostProtectionLease{registry: registry, id: active.ID(), capability: old.capability, record: active.record}
	if err = forged.releaseVerified(ctx); !errors.Is(err, ErrAuthority) {
		t.Fatalf("foreign capability released active row: %v", err)
	}
	reconstructed := &HostProtectionLease{registry: registry, id: old.ID(), capability: old.capability, record: old.record}
	if err = reconstructed.releaseVerified(ctx); !errors.Is(err, ErrAuthority) {
		t.Fatalf("missing row manufactured settlement: %v", err)
	}
	if got := protectionRows(t, registry); !reflect.DeepEqual(got, after) {
		t.Fatal("invalid retries changed retained state")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err = old.releaseVerified(cancelled); !errors.Is(err, context.Canceled) {
		t.Fatalf("settled retry ignored cancellation: %v", err)
	}
}

func TestHostProtectionReleaseCachesOnlySuccessfulCommit(t *testing.T) {
	ctx := context.Background()
	registry := protectionTestRegistry(t)
	lease, err := registry.ReserveExecution(ctx, protectionTestConfig(protectionTestRoot(t), "commit-observation"), protectionTestRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	protectionTestSQL(t, registry, "CREATE TABLE injected_settlement_failure(missing INTEGER REFERENCES host_protection_meta(singleton) DEFERRABLE INITIALLY DEFERRED); CREATE TRIGGER fail_settlement AFTER UPDATE ON host_protection_records BEGIN INSERT INTO injected_settlement_failure VALUES(2); END")
	lease.mu.Lock()
	err = lease.publishRelease(ctx)
	lease.mu.Unlock()
	if !errors.Is(err, ErrHostProtectionStorage) || lease.released {
		t.Fatalf("failed commit cached release: %v %v", lease.released, err)
	}
	protectionTestSQL(t, registry, "DROP TRIGGER fail_settlement")
	// Exercise publishRelease directly: the commit callback, rather than its
	// caller's later success branch, must record settlement before close defers.
	lease.mu.Lock()
	err = lease.publishRelease(ctx)
	lease.mu.Unlock()
	if err != nil || !lease.released {
		t.Fatalf("successful commit not captured by publication: %v %v", lease.released, err)
	}
}

func TestHostProtectionReclamationRollsBackWithAdmission(t *testing.T) {
	for _, failure := range []string{"delete", "insert", "commit"} {
		t.Run(failure, func(t *testing.T) {
			ctx := context.Background()
			registry := protectionTestRegistry(t)
			root, journal := protectionTestRoot(t), protectionTestRoot(t)
			old, err := registry.ReserveExecution(ctx, protectionTestConfig(root, "settled"), journal)
			if err != nil {
				t.Fatal(err)
			}
			if err = old.releaseVerified(ctx); err != nil {
				t.Fatal(err)
			}
			protectionFillReleased(t, registry, old.record, hostProtectionMaxRecords)
			before := protectionRows(t, registry)
			switch failure {
			case "delete":
				protectionTestSQL(t, registry, "CREATE TRIGGER fail_reclaim BEFORE DELETE ON host_protection_records BEGIN SELECT RAISE(ABORT,'injected delete failure'); END")
			case "insert":
				protectionTestSQL(t, registry, "CREATE TRIGGER fail_reclaim BEFORE INSERT ON host_protection_records BEGIN SELECT RAISE(ABORT,'injected insert failure'); END")
			case "commit":
				protectionTestSQL(t, registry, "CREATE TABLE injected_reclaim_failure(missing INTEGER REFERENCES host_protection_meta(singleton) DEFERRABLE INITIALLY DEFERRED); CREATE TRIGGER fail_reclaim AFTER DELETE ON host_protection_records BEGIN INSERT INTO injected_reclaim_failure VALUES(2); END")
			}
			if lease, err := registry.ReserveExecution(ctx, protectionTestConfig(root, "next"), journal); lease != nil || !errors.Is(err, ErrHostProtectionStorage) {
				t.Fatalf("failed admission published: %v %v", lease, err)
			}
			if got := protectionRows(t, registry); !reflect.DeepEqual(got, before) {
				t.Fatal("failed admission erased settled evidence")
			}
			protectionTestSQL(t, registry, "DROP TRIGGER fail_reclaim")
			if _, err = registry.ReserveExecution(ctx, protectionTestConfig(root, "next"), journal); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestHostProtectionReclamationMakesRoomAmongActiveRows(t *testing.T) {
	ctx := context.Background()
	registry := protectionTestRegistry(t)
	root, journal := protectionTestRoot(t), protectionTestRoot(t)
	first, err := registry.ReserveExecution(ctx, protectionTestConfig(root, "settled"), journal)
	if err != nil {
		t.Fatal(err)
	}
	if err = first.releaseVerified(ctx); err != nil {
		t.Fatal(err)
	}
	if err = registry.transaction(ctx, func(store *hostProtectionTransaction) error {
		for index := 1; index < hostProtectionMaxRecords; index++ {
			row := first.record
			row.ID = protectionHash(fmt.Sprintf("active-capacity-%d", index))
			binding := *row.Binding
			binding.Session = fmt.Sprintf("active-capacity-%d", index)
			row.Binding = &binding
			if err := store.insert(ctx, row); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err = registry.ReserveExecution(ctx, protectionTestConfig(root, "last-slot"), journal); err != nil {
		t.Fatal(err)
	}
	if _, err = registry.ReserveExecution(ctx, protectionTestConfig(root, "no-slot"), journal); !errors.Is(err, ErrHostProtectionLimit) {
		t.Fatalf("active capacity bypassed: %v", err)
	}
	rows := protectionRows(t, registry)
	if len(rows) != hostProtectionMaxRecords {
		t.Fatalf("wrong active count %d", len(rows))
	}
	if _, exists := rows[first.ID()]; exists {
		t.Fatal("settled row retained ahead of active admission")
	}
}
