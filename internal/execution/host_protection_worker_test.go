package execution

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/storage"
)

func protectedWorkerFixture(t *testing.T) (*Worker, *HostProtectionLease, *HostProtectionRegistry) {
	t.Helper()
	registry := protectionTestRegistry(t)
	root, state := protectionTestRoot(t), protectionTestRoot(t)
	config := protectionTestConfig(root, "protected-worker-session")
	lease, err := registry.ReserveExecution(context.Background(), config, state)
	if err != nil {
		t.Fatal(err)
	}
	session, err := OpenSession(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Close() })
	journal, err := OpenJournal(state)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { journal.Close() })
	return &Worker{Session: session, Journal: journal, Processes: NewProcessManager(session), Token: strings.Repeat("b", 64)}, lease, registry
}

func TestProtectedWorkerReleaseRequiresItsOwnConfirmedCleanup(t *testing.T) {
	for _, uncertain := range []bool{false, true} {
		t.Run(map[bool]string{false: "settled", true: "unproven-descendants"}[uncertain], func(t *testing.T) {
			worker, lease, registry := protectedWorkerFixture(t)
			worker.Processes.cleanupUnproven = uncertain
			var output bytes.Buffer
			err := worker.ServeProtected(context.Background(), io.NopCloser(bytes.NewReader(nil)), &output, lease)
			if uncertain && !errors.Is(err, ErrHostProtectionUncertain) || !uncertain && err != nil {
				t.Fatalf("serve: %v", err)
			}
			if !worker.Session.revoked.Load() {
				t.Fatal("dispatch authority survived cleanup")
			}
			if err := lease.CancelUnactivated(context.Background()); !errors.Is(err, ErrAuthority) {
				t.Fatalf("activated lease abandoned without cleanup: %v", err)
			}
			_, err = registry.RegisterBrowserRoot(context.Background(), worker.Session.config.Root)
			if uncertain && !errors.Is(err, ErrHostProtectionConflict) || !uncertain && err != nil {
				t.Fatalf("reservation after cleanup: %v", err)
			}
		})
	}
}

func TestProtectedWorkerCannotReleaseAnotherWorkersReservation(t *testing.T) {
	worker, lease, registry := protectedWorkerFixture(t)
	worker.Session.config.Binding.Grant = "foreign-grant"
	if err := worker.ServeProtected(context.Background(), io.NopCloser(bytes.NewReader(nil)), io.Discard, lease); !errors.Is(err, ErrHostProtectionChanged) {
		t.Fatalf("foreign worker: %v", err)
	}
	if _, err := registry.RegisterBrowserRoot(context.Background(), worker.Session.config.Root); !errors.Is(err, ErrHostProtectionConflict) {
		t.Fatalf("foreign worker released row: %v", err)
	}
	if err := lease.CancelUnactivated(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestProtectedWorkerFailedReleasePublicationRetainsAdmissionFence(t *testing.T) {
	worker, lease, registry := protectedWorkerFixture(t)
	db, err := storage.OpenSQLite(filepath.Join(registry.directory, "protection.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TRIGGER reject_release BEFORE UPDATE ON host_protection_records BEGIN SELECT RAISE(ABORT,'fixture release failure'); END`); err != nil {
		t.Fatal(err)
	}
	err = worker.ServeProtected(context.Background(), io.NopCloser(bytes.NewReader(nil)), io.Discard, lease)
	if !errors.Is(err, ErrHostProtectionStorage) || !worker.Processes.cleanupConfirmed() {
		t.Fatalf("cleanup/publication: %v", err)
	}
	if _, err = registry.RegisterBrowserRoot(context.Background(), worker.Session.config.Root); !errors.Is(err, ErrHostProtectionConflict) {
		t.Fatalf("failed write removed fence: %v", err)
	}
	if _, err = db.Exec(`DROP TRIGGER reject_release`); err != nil {
		t.Fatal(err)
	}
	// The same still-owned capability can retry its confirmed publication. A
	// restarted process cannot reconstruct this capability from the durable row.
	if err = lease.releaseVerified(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err = lease.releaseVerified(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err = registry.RegisterBrowserRoot(context.Background(), worker.Session.config.Root); err != nil {
		t.Fatal(err)
	}
}
