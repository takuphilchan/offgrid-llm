package execution

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/storage"
)

func protectionTestSQL(t *testing.T, registry *HostProtectionRegistry, statement string) {
	t.Helper()
	db, err := storage.OpenSQLite(filepath.Join(registry.directory, "protection.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	_, execErr := db.Exec(statement)
	closeErr := db.Close()
	if err = errors.Join(execErr, closeErr); err != nil {
		t.Fatal(err)
	}
}

func TestHostProtectionCorruptionDoesNotResetAdmissions(t *testing.T) {
	for _, kind := range []string{"payload", "file", "missing database", "missing directory", "identity"} {
		t.Run(kind, func(t *testing.T) {
			ctx := context.Background()
			registry := protectionTestRegistry(t)
			root := protectionTestRoot(t)
			if _, err := registry.RegisterBrowserRoot(ctx, root); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(registry.directory, "protection.sqlite")
			switch kind {
			case "payload":
				protectionTestSQL(t, registry, "UPDATE host_protection_records SET payload=x'7b7d'")
			case "file":
				if err := os.WriteFile(path, []byte("corrupt retained store"), 0600); err != nil {
					t.Fatal(err)
				}
			case "missing database":
				if err := os.Rename(path, path+".retained"); err != nil {
					t.Fatal(err)
				}
			case "missing directory":
				if err := os.Rename(registry.directory, registry.directory+".retained"); err != nil {
					t.Fatal(err)
				}
			case "identity":
				if err := os.WriteFile(filepath.Join(filepath.Dir(registry.directory), "host-protection.identity"), []byte("invalid"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := registry.ReserveExecution(ctx, protectionTestConfig(root, "would-overlap"), protectionTestRoot(t)); !errors.Is(err, ErrHostProtectionCorrupt) {
				t.Fatalf("did not fence corruption: %v", err)
			}
			if kind == "missing database" {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatal("recreated missing database")
				}
			}
			if kind == "missing directory" {
				if _, err := os.Stat(registry.directory); !os.IsNotExist(err) {
					t.Fatal("recreated missing registry")
				}
			}
			if kind == "file" {
				if contents, err := os.ReadFile(path); err != nil || string(contents) != "corrupt retained store" {
					t.Fatal("rewrote corrupt evidence")
				}
			}
		})
	}
}

func TestHostProtectionFailedWritesDoNotPublishOrRelease(t *testing.T) {
	ctx := context.Background()
	registry := protectionTestRegistry(t)
	if _, err := registry.RegisterBrowserRoot(ctx, protectionTestRoot(t)); err != nil {
		t.Fatal(err)
	}
	workspace := protectionTestRoot(t)
	journal := protectionTestRoot(t)
	config := protectionTestConfig(workspace, "lease")
	protectionTestSQL(t, registry, "CREATE TRIGGER fail_admission BEFORE INSERT ON host_protection_records BEGIN SELECT RAISE(ABORT,'injected write failure'); END")
	if lease, err := registry.ReserveExecution(ctx, config, journal); lease != nil || !errors.Is(err, ErrHostProtectionStorage) {
		t.Fatalf("published failed reservation: %v %v", lease, err)
	}
	protectionTestSQL(t, registry, "DROP TRIGGER fail_admission")
	lease, err := registry.ReserveExecution(ctx, config, journal)
	if err != nil {
		t.Fatal(err)
	}
	protectionTestSQL(t, registry, "CREATE TRIGGER fail_release BEFORE UPDATE ON host_protection_records BEGIN SELECT RAISE(ABORT,'injected write failure'); END")
	if err = lease.releaseVerified(ctx); !errors.Is(err, ErrHostProtectionStorage) {
		t.Fatalf("release did not report failed persistence: %v", err)
	}
	reopened := &HostProtectionRegistry{directory: registry.directory}
	if _, err = reopened.RegisterBrowserRoot(ctx, filepath.Join(workspace, "browser")); !errors.Is(err, ErrHostProtectionConflict) {
		t.Fatalf("failed release refunded reservation: %v", err)
	}
	protectionTestSQL(t, registry, "DROP TRIGGER fail_release")
	if err = lease.releaseVerified(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = reopened.RegisterBrowserRoot(ctx, filepath.Join(workspace, "browser")); err != nil {
		t.Fatal(err)
	}
}

func TestHostProtectionFailedFinalizationKeepsOriginalReservation(t *testing.T) {
	ctx := context.Background()
	registry := protectionTestRegistry(t)
	root := filepath.Join(protectionTestRoot(t), "browser")
	claim, err := registry.RegisterBrowserRoot(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if err = EnsurePrivateDirectory(root); err != nil {
		t.Fatal(err)
	}
	protectionTestSQL(t, registry, "CREATE TRIGGER fail_finalize BEFORE UPDATE ON host_protection_records BEGIN SELECT RAISE(ABORT,'injected write failure'); END")
	if err = claim.Finalize(ctx); !errors.Is(err, ErrHostProtectionStorage) {
		t.Fatal(err)
	}
	if _, err = registry.RegisterBrowserRoot(ctx, root); !errors.Is(err, ErrHostProtectionUncertain) {
		t.Fatalf("adopted failed finalization: %v", err)
	}
	protectionTestSQL(t, registry, "DROP TRIGGER fail_finalize")
	if err = claim.Finalize(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestHostProtectionFailedCommitCannotRelease(t *testing.T) {
	ctx := context.Background()
	registry := protectionTestRegistry(t)
	workspace := protectionTestRoot(t)
	lease, err := registry.ReserveExecution(ctx, protectionTestConfig(workspace, "commit-failure"), protectionTestRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	protectionTestSQL(t, registry, `CREATE TABLE injected_commit_failure(missing INTEGER REFERENCES host_protection_meta(singleton) DEFERRABLE INITIALLY DEFERRED); CREATE TRIGGER fail_commit AFTER UPDATE ON host_protection_records BEGIN INSERT INTO injected_commit_failure VALUES(2); END`)
	if err = lease.releaseVerified(ctx); !errors.Is(err, ErrHostProtectionStorage) {
		t.Fatalf("did not report commit failure: %v", err)
	}
	if _, err = registry.RegisterBrowserRoot(ctx, filepath.Join(workspace, "browser")); !errors.Is(err, ErrHostProtectionConflict) {
		t.Fatalf("failed commit released reservation: %v", err)
	}
	protectionTestSQL(t, registry, "DROP TRIGGER fail_commit; DROP TABLE injected_commit_failure")
	if err = lease.releaseVerified(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestHostProtectionBoundedRecordsAndBusyAdmission(t *testing.T) {
	ctx := context.Background()
	registry := protectionTestRegistry(t)
	root := protectionTestRoot(t)
	first, err := registry.RegisterBrowserRoot(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := storage.AcquireOwnership(filepath.Dir(registry.directory))
	if err != nil {
		t.Fatal(err)
	}
	_, blockedErr := registry.RegisterBrowserRoot(ctx, protectionTestRoot(t))
	closeErr := owner.Close()
	if !errors.Is(blockedErr, ErrHostProtectionBusy) || closeErr != nil {
		t.Fatalf("lock contention not fenced: %v %v", blockedErr, closeErr)
	}
	err = registry.transaction(ctx, func(store *hostProtectionTransaction) error {
		for i := 1; i < hostProtectionMaxRecords; i++ {
			location := first.record.Root
			location.Suffix = fmt.Sprintf("pending-%04d", i)
			location.Path = filepath.Join(location.Anchor, location.Suffix)
			row := hostProtectionRecord{Version: 1, ID: protectionPathKey(location.Path), Kind: "browser", State: "pending", Root: location, CapabilityHash: protectionHash("unresolved fixture capability")}
			if err := store.insert(ctx, row); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = registry.RegisterBrowserRoot(ctx, protectionTestRoot(t)); !errors.Is(err, ErrHostProtectionLimit) {
		t.Fatalf("record cap not enforced: %v", err)
	}
	if _, err = registry.RegisterBrowserRoot(ctx, root); err != nil {
		t.Fatalf("limit broke idempotent registration: %v", err)
	}
}
