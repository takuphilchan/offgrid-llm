package execution

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/storage"
)

type protectionFileSnapshot struct {
	Mode     fs.FileMode
	Contents string
}

func protectionInspectFiles(t *testing.T, parent string) map[string]protectionFileSnapshot {
	t.Helper()
	result := map[string]protectionFileSnapshot{}
	err := filepath.WalkDir(parent, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(parent, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		value := protectionFileSnapshot{Mode: info.Mode()}
		if info.Mode().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			value.Contents = string(data)
		}
		result[rel] = value
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestHostProtectionInspectAbsentDoesNotCreateState(t *testing.T) {
	registry := protectionTestRegistry(t)
	parent := filepath.Dir(registry.directory)
	before := protectionInspectFiles(t, parent)
	result, err := registry.Inspect(context.Background(), "", 64)
	if err != nil || result.Status != "absent" || result.Version != 1 || len(result.Records) != 0 || result.Next != "" {
		t.Fatalf("missing registry: %+v %v", result, err)
	}
	if after := protectionInspectFiles(t, parent); !reflect.DeepEqual(before, after) {
		t.Fatal("inspection created or changed absent state")
	}
	for _, fixture := range []struct {
		after string
		limit int
	}{{"", 0}, {"", 65}, {"not-an-id", 1}} {
		if _, err = registry.Inspect(context.Background(), fixture.after, fixture.limit); !errors.Is(err, ErrAuthority) {
			t.Fatalf("invalid request accepted: %v", err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = registry.Inspect(ctx, "", 1); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if after := protectionInspectFiles(t, parent); !reflect.DeepEqual(before, after) {
		t.Fatal("invalid inspection changed state")
	}
}

func TestHostProtectionInspectTypedDeterministicReadOnlyPages(t *testing.T) {
	ctx := context.Background()
	registry := protectionTestRegistry(t)
	protected, err := registry.RegisterBrowserRoot(ctx, protectionTestRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	pending, err := registry.RegisterBrowserRoot(ctx, filepath.Join(protectionTestRoot(t), "missing"))
	if err != nil {
		t.Fatal(err)
	}
	lease, err := registry.ReserveExecution(ctx, protectionTestConfig(protectionTestRoot(t), "inspect"), protectionTestRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	before := protectionInspectFiles(t, filepath.Dir(registry.directory))
	var ids []string
	after := ""
	records := map[string]HostProtectionInspectionRecord{}
	for {
		page, err := registry.Inspect(ctx, after, 1)
		if err != nil {
			t.Fatal(err)
		}
		if page.Status != "ready" || page.Version != 1 || len(page.Records) != 1 {
			t.Fatalf("invalid page: %+v", page)
		}
		for _, row := range page.Records {
			ids = append(ids, row.ID)
			records[row.ID] = row
		}
		if page.Next == "" {
			break
		}
		if page.Next <= after {
			t.Fatal("cursor failed to advance")
		}
		after = page.Next
	}
	if len(ids) != 3 || !sort.StringsAreSorted(ids) {
		t.Fatalf("nondeterministic ordering: %v", ids)
	}
	if row := records[protected.ID()]; row.Root.State != "matched" || row.BlockingReason != "protected_browser_storage" || row.AdmissionScope != "overlap" {
		t.Fatalf("protected summary: %+v", row)
	}
	if row := records[pending.ID()]; row.Root.State != "pending_missing" || row.BlockingReason != "pending_browser_finalization" {
		t.Fatalf("pending summary: %+v", row)
	}
	if row := records[lease.ID()]; row.Binding == nil || row.Binding.Session != "inspect" || row.Journal == nil || row.Journal.State != "matched" || row.BlockingReason != "execution_cleanup_unconfirmed" {
		t.Fatalf("execution summary: %+v", row)
	}
	encoded, err := json.Marshal(records)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{lease.capability, lease.record.CapabilityHash, pending.capability, pending.record.CapabilityHash, "capability_hash", "identity", "\"grant\""} {
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("inspection leaked private field %q", secret)
		}
	}
	if current := protectionInspectFiles(t, filepath.Dir(registry.directory)); !reflect.DeepEqual(before, current) {
		t.Fatal("clean inspection mutated store bytes, modes or file list")
	}
}

func TestHostProtectionInspectReportsChangedPathsWithoutAdmission(t *testing.T) {
	for _, kind := range []string{"missing", "replacement", "pending_created", "pending_invalid", "journal_missing"} {
		t.Run(kind, func(t *testing.T) {
			ctx := context.Background()
			registry := protectionTestRegistry(t)
			root := protectionTestRoot(t)
			var id string
			if strings.HasPrefix(kind, "pending_") {
				root = filepath.Join(root, "pending")
				claim, err := registry.RegisterBrowserRoot(ctx, root)
				if err != nil {
					t.Fatal(err)
				}
				id = claim.ID()
				if kind == "pending_created" {
					if err = EnsurePrivateDirectory(root); err != nil {
						t.Fatal(err)
					}
				} else {
					if err = os.WriteFile(root, []byte("unexpected object"), 0600); err != nil {
						t.Fatal(err)
					}
				}
			} else if kind == "journal_missing" {
				journal := protectionTestRoot(t)
				lease, err := registry.ReserveExecution(ctx, protectionTestConfig(root, "missing-journal"), journal)
				if err != nil {
					t.Fatal(err)
				}
				id = lease.ID()
				if err = os.Rename(journal, journal+"-retained"); err != nil {
					t.Fatal(err)
				}
			} else {
				claim, err := registry.RegisterBrowserRoot(ctx, root)
				if err != nil {
					t.Fatal(err)
				}
				id = claim.ID()
				if err = os.Rename(root, root+"-retained"); err != nil {
					t.Fatal(err)
				}
				if kind == "replacement" {
					if err = EnsurePrivateDirectory(root); err != nil {
						t.Fatal(err)
					}
				}
			}
			before := protectionInspectFiles(t, filepath.Dir(registry.directory))
			page, err := registry.Inspect(ctx, "", 64)
			if err != nil || len(page.Records) != 1 || page.Records[0].ID != id {
				t.Fatalf("observation rejected changed path: %+v %v", page, err)
			}
			row := page.Records[0]
			want := map[string]string{"missing": "root_missing", "replacement": "root_identity_changed", "pending_created": "pending_browser_finalization", "pending_invalid": "root_unavailable", "journal_missing": "journal_missing"}[kind]
			if row.BlockingReason != want {
				t.Fatalf("wrong observation: %+v", row)
			}
			if kind == "pending_created" {
				if row.Root.State != "pending_created" || row.State != "pending" || row.AdmissionScope != "overlap" {
					t.Fatalf("adopted pending creation: %+v", row)
				}
			} else if row.AdmissionScope != "all" {
				t.Fatalf("lost global fence: %+v", row)
			}
			if current := protectionInspectFiles(t, filepath.Dir(registry.directory)); !reflect.DeepEqual(before, current) {
				t.Fatal("path observation mutated registry")
			}
		})
	}
}

func TestHostProtectionInspectCorruptionAndRetainedJournalDoNotMutate(t *testing.T) {
	for _, kind := range []string{"missing_database", "missing_marker", "payload", "database_bytes", "wal", "journal"} {
		t.Run(kind, func(t *testing.T) {
			ctx := context.Background()
			registry := protectionTestRegistry(t)
			if _, err := registry.RegisterBrowserRoot(ctx, protectionTestRoot(t)); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(registry.directory, "protection.sqlite")
			switch kind {
			case "missing_database":
				if err := os.Rename(path, path+".retained"); err != nil {
					t.Fatal(err)
				}
			case "missing_marker":
				marker := filepath.Join(filepath.Dir(registry.directory), "host-protection.identity")
				if err := os.Rename(marker, marker+".retained"); err != nil {
					t.Fatal(err)
				}
			case "payload":
				protectionTestSQL(t, registry, "UPDATE host_protection_records SET payload=x'7b7d'")
			case "database_bytes":
				if err := os.WriteFile(path, []byte("damaged SQLite"), 0600); err != nil {
					t.Fatal(err)
				}
			case "wal", "journal":
				if err := os.WriteFile(path+"-"+kind, []byte("retained recovery evidence"), 0600); err != nil {
					t.Fatal(err)
				}
				if err := ProtectPrivatePath(path + "-" + kind); err != nil {
					t.Fatal(err)
				}
			}
			before := protectionInspectFiles(t, filepath.Dir(registry.directory))
			page, err := registry.Inspect(ctx, "", 64)
			if len(page.Records) != 0 {
				t.Fatal("returned records from invalid snapshot")
			}
			if kind == "wal" || kind == "journal" {
				var unavailable *HostProtectionSnapshotUnavailableError
				if !errors.As(err, &unavailable) || unavailable.Reason != "retained_"+kind || !errors.Is(err, ErrHostProtectionSnapshotUnavailable) {
					t.Fatalf("ignored retained recovery state: %v", err)
				}
			} else if !errors.Is(err, ErrHostProtectionCorrupt) {
				t.Fatalf("corruption not distinguished: %v", err)
			}
			if current := protectionInspectFiles(t, filepath.Dir(registry.directory)); !reflect.DeepEqual(before, current) {
				t.Fatal("failed inspection changed store")
			}
		})
	}
}

func TestHostProtectionInspectBusyAndActualWAL(t *testing.T) {
	ctx := context.Background()
	registry := protectionTestRegistry(t)
	if _, err := registry.RegisterBrowserRoot(ctx, protectionTestRoot(t)); err != nil {
		t.Fatal(err)
	}
	parent := filepath.Dir(registry.directory)
	before := protectionInspectFiles(t, parent)
	owner, err := storage.AcquireOwnership(parent)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	if _, err = registry.Inspect(ctx, "", 64); !errors.Is(err, ErrHostProtectionBusy) {
		t.Fatalf("concurrent owner was ignored: %v", err)
	}
	if err = owner.Close(); err != nil {
		t.Fatal(err)
	}
	if current := protectionInspectFiles(t, parent); !reflect.DeepEqual(before, current) {
		t.Fatal("busy inspection mutated store")
	}
	db, err := storage.OpenSQLite(filepath.Join(registry.directory, "protection.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec("PRAGMA wal_autocheckpoint=0"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("UPDATE host_protection_meta SET identity=?", strings.Repeat("f", 64)); err != nil {
		t.Fatal(err)
	}
	before = protectionInspectFiles(t, parent)
	if _, err = registry.Inspect(ctx, "", 64); !errors.Is(err, ErrHostProtectionSnapshotUnavailable) {
		t.Fatalf("actual retained WAL silently ignored: %v", err)
	}
	if current := protectionInspectFiles(t, parent); !reflect.DeepEqual(before, current) {
		t.Fatal("WAL inspection mutated source")
	}
}

func TestHostProtectionInspectValidatesOutsidePageAndBounds(t *testing.T) {
	t.Run("corrupt outside page", func(t *testing.T) {
		ctx := context.Background()
		registry := protectionTestRegistry(t)
		for index := 0; index < 2; index++ {
			if _, err := registry.RegisterBrowserRoot(ctx, protectionTestRoot(t)); err != nil {
				t.Fatal(err)
			}
		}
		protectionTestSQL(t, registry, "UPDATE host_protection_records SET payload=x'7b7d' WHERE id=(SELECT MAX(id) FROM host_protection_records)")
		before := protectionInspectFiles(t, filepath.Dir(registry.directory))
		page, err := registry.Inspect(ctx, "", 1)
		if !errors.Is(err, ErrHostProtectionCorrupt) || len(page.Records) != 0 {
			t.Fatalf("returned healthy page from corrupt store: %+v %v", page, err)
		}
		if current := protectionInspectFiles(t, filepath.Dir(registry.directory)); !reflect.DeepEqual(before, current) {
			t.Fatal("corrupt page inspection mutated store")
		}
	})
	t.Run("record bound", func(t *testing.T) {
		ctx := context.Background()
		registry := protectionTestRegistry(t)
		lease, err := registry.ReserveExecution(ctx, protectionTestConfig(protectionTestRoot(t), "inspect-bound"), protectionTestRoot(t))
		if err != nil {
			t.Fatal(err)
		}
		protectionFillReleased(t, registry, lease.record, hostProtectionMaxRecords)
		if err = registry.transaction(ctx, func(store *hostProtectionTransaction) error {
			row := lease.record
			row.ID = protectionHash("over-bound fixture")
			return store.write(ctx, row, false)
		}); err != nil {
			t.Fatal(err)
		}
		before := protectionInspectFiles(t, filepath.Dir(registry.directory))
		page, err := registry.Inspect(ctx, "", 1)
		if !errors.Is(err, ErrHostProtectionLimit) || len(page.Records) != 0 {
			t.Fatalf("unbounded store accepted: %+v %v", page, err)
		}
		if current := protectionInspectFiles(t, filepath.Dir(registry.directory)); !reflect.DeepEqual(before, current) {
			t.Fatal("over-bound inspection mutated store")
		}
	})
}

func TestHostProtectionInspectReleasedMissingPathDoesNotBlock(t *testing.T) {
	ctx := context.Background()
	registry := protectionTestRegistry(t)
	root := protectionTestRoot(t)
	lease, err := registry.ReserveExecution(ctx, protectionTestConfig(root, "released-observation"), protectionTestRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	if err = lease.releaseVerified(ctx); err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(root, root+"-retained"); err != nil {
		t.Fatal(err)
	}
	page, err := registry.Inspect(ctx, "", 64)
	if err != nil || len(page.Records) != 1 {
		t.Fatalf("inspection: %+v %v", page, err)
	}
	row := page.Records[0]
	if row.Root.State != "missing" || row.State != "released" || row.AdmissionScope != "none" || row.BlockingReason != "none" {
		t.Fatalf("released history blocks admission: %+v", row)
	}
}
