package execution

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkingCopyAlteredLeaseCannotAdmitWorker(t *testing.T) {
	endpoint, store, binding, _ := workingCopyFixture(t)
	if _, err := store.db.Exec(`CREATE TRIGGER alter_lease AFTER UPDATE OF in_use ON worker_copy_ownership WHEN NEW.in_use=1 BEGIN UPDATE worker_copy_ownership SET in_use=0 WHERE copy_id=NEW.copy_id; END`); err != nil {
		t.Fatal(err)
	}
	worker, err := endpoint.PrepareWorker(context.Background(), workingCopyRequest(binding, "altered-lease"))
	if !errors.Is(err, ErrRevision) || worker != nil {
		t.Fatal("worker admitted without durable lease", worker != nil, err)
	}
}

func TestWorkingCopyAlteredIdentityCannotCopyBytes(t *testing.T) {
	endpoint, store, binding, source := workingCopyFixture(t)
	if err := os.WriteFile(filepath.Join(source, "input"), []byte("private input"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`CREATE TRIGGER alter_identity AFTER UPDATE OF identity ON worker_copy_ownership WHEN NEW.identity!='' BEGIN UPDATE worker_copy_ownership SET identity='' WHERE copy_id=NEW.copy_id; END`); err != nil {
		t.Fatal(err)
	}
	if _, err := endpoint.PrepareWorker(context.Background(), workingCopyRequest(binding, "altered-identity")); !errors.Is(err, ErrRevision) {
		t.Fatal("altered identity accepted", err)
	}
	var id string
	if err := store.db.QueryRow(`SELECT id FROM worker_copies`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := store.root.Lstat(id + "/content"); !os.IsNotExist(err) {
		t.Fatal("bytes copied before exact ownership receipt", err)
	}
}

func TestWorkingCopyAlteredSettlementRetainsLease(t *testing.T) {
	endpoint, store, binding, _ := workingCopyFixture(t)
	ctx := context.Background()
	child, err := endpoint.PrepareWorker(ctx, workingCopyRequest(binding, "altered-settlement"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.db.Exec(`CREATE TRIGGER alter_settlement AFTER UPDATE OF in_use ON worker_copy_ownership WHEN NEW.in_use=0 BEGIN UPDATE worker_copy_ownership SET in_use=1 WHERE copy_id=NEW.copy_id; END`); err != nil {
		t.Fatal(err)
	}
	if err = child.Close(); !errors.Is(err, ErrRevision) {
		t.Fatal("altered settlement acknowledged", err)
	}
	if err = store.Release(ctx, child.Record.ID, binding); !errors.Is(err, ErrHostUnavailable) {
		t.Fatal("cleanup accepted unconfirmed settlement", err)
	}
	if _, err = store.db.Exec(`DROP TRIGGER alter_settlement`); err != nil {
		t.Fatal(err)
	}
	if err = child.Release(ctx); err != nil {
		t.Fatal("same closed owner could not retry", err)
	}
}

func TestWorkingCopyAlteredCleanupCheckpointRollsBack(t *testing.T) {
	for _, phase := range []string{"quarantine", "detached", "empty", "released", "uncertain"} {
		t.Run(phase, func(t *testing.T) {
			endpoint, store, binding, _ := workingCopyFixture(t)
			ctx := context.Background()
			child, err := endpoint.PrepareWorker(ctx, workingCopyRequest(binding, "altered-"+phase))
			if err != nil {
				t.Fatal(err)
			}
			if err = child.Close(); err != nil {
				t.Fatal(err)
			}
			var trigger string
			switch phase {
			case "quarantine":
				trigger = `CREATE TRIGGER alter_checkpoint AFTER UPDATE OF quarantine ON worker_copy_ownership WHEN NEW.quarantine!='' BEGIN UPDATE worker_copy_ownership SET quarantine='cleanup-` + strings.Repeat("0", 64) + `' WHERE copy_id=NEW.copy_id; END`
			case "detached":
				trigger = `CREATE TRIGGER alter_checkpoint AFTER UPDATE OF detached ON worker_copy_ownership WHEN NEW.detached=1 BEGIN UPDATE worker_copy_ownership SET detached=0 WHERE copy_id=NEW.copy_id; END`
			case "empty":
				trigger = `CREATE TRIGGER alter_checkpoint AFTER UPDATE OF empty ON worker_copy_ownership WHEN NEW.empty=1 BEGIN UPDATE worker_copy_ownership SET empty=0 WHERE copy_id=NEW.copy_id; END`
			case "released":
				trigger = `CREATE TRIGGER alter_checkpoint AFTER UPDATE OF state ON worker_copies WHEN NEW.state='released' BEGIN UPDATE worker_copies SET state='uncertain' WHERE id=NEW.id; END`
			case "uncertain":
				trigger = `CREATE TRIGGER alter_checkpoint AFTER UPDATE OF state ON worker_copies WHEN NEW.state='uncertain' BEGIN UPDATE worker_copies SET reserved_bytes=0 WHERE id=NEW.id; END`
			}
			if _, err = store.db.Exec(trigger); err != nil {
				t.Fatal(err)
			}
			err = store.releaseCopy(ctx, child.Record.ID, binding, func(stage, _ string) error {
				if phase == "uncertain" && stage == "intent_saved" {
					return ErrRevision
				}
				return nil
			})
			if !errors.Is(err, ErrRevision) {
				t.Fatal("altered checkpoint acknowledged", err)
			}
			record, err := store.Inspect(ctx, child.Record.ID, binding)
			if err != nil || record.ReservedBytes != child.Record.ReservedBytes || record.State == "released" {
				t.Fatal("altered checkpoint refunded capacity", record, err)
			}
			owner, err := store.copyOwnership(ctx, child.Record.ID)
			if err != nil {
				t.Fatal(err)
			}
			if phase == "quarantine" && owner.Quarantine != "" || phase == "detached" && owner.Detached || phase == "empty" && owner.Empty {
				t.Fatal("altered ownership checkpoint persisted", owner)
			}
			if _, err = store.db.Exec(`DROP TRIGGER alter_checkpoint`); err != nil {
				t.Fatal(err)
			}
			if err = store.Release(ctx, child.Record.ID, binding); err != nil {
				t.Fatal("original ownership could not be explicitly resumed", err)
			}
		})
	}
}
