package execution

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/takuphilchan/offgrid-llm/internal/jsonwire"
	"github.com/takuphilchan/offgrid-llm/internal/storage"
)

const hostProtectionSchema = `
CREATE TABLE host_protection_meta(singleton INTEGER PRIMARY KEY CHECK(singleton=1), version INTEGER NOT NULL CHECK(version=1), identity TEXT NOT NULL CHECK(length(identity)=64)) STRICT;
CREATE TABLE host_protection_records(id TEXT PRIMARY KEY CHECK(length(id)=64), payload BLOB NOT NULL CHECK(length(payload)<=49152), digest TEXT NOT NULL CHECK(length(digest)=64)) STRICT;
`

// Settled execution history is diagnostic, not perpetual admission authority.
// Keep a bounded tail by admission order; active and browser records are never
// eligible. Per-grant journals own binding validation and effect no-replay.
const hostProtectionReleasedHistory = 128

type hostProtectionTransaction struct {
	tx          *sql.Tx
	records     map[string]hostProtectionRecord
	afterCommit func()
}

func (s *HostProtectionRegistry) transaction(ctx context.Context, change func(*hostProtectionTransaction) error) (result error) {
	if s == nil || !protectionPathValid(s.directory) || change == nil {
		return ErrAuthority
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.checkAncestry(s.directory); err != nil {
		return err
	}
	parent := filepath.Dir(s.directory)
	if err := s.ensureParents(parent); err != nil {
		return errors.Join(ErrHostProtectionStorage, err)
	}
	// The account-parent lock serializes first creation too. A lock inside a
	// not-yet-initialized store would race fresh creation with crash detection.
	owner, err := storage.AcquireOwnership(parent)
	if errors.Is(err, storage.ErrWorkspaceInUse) {
		return ErrHostProtectionBusy
	}
	if err != nil {
		return errors.Join(ErrHostProtectionStorage, err)
	}
	defer func() { result = errors.Join(result, owner.Close()) }()
	if err = CheckPrivatePath(filepath.Join(parent, ".offgrid-owner.lock")); err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	identity, fresh, err := s.openIdentity()
	if err != nil {
		return err
	}
	path := filepath.Join(s.directory, "protection.sqlite")
	if fresh {
		file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
		if err != nil {
			return errors.Join(ErrHostProtectionStorage, err)
		}
		err = errors.Join(ProtectPrivatePath(path), file.Sync(), file.Close())
		if err != nil {
			return errors.Join(ErrHostProtectionStorage, err)
		}
		if err = syncHostProtectionDirectory(s.directory); err != nil {
			return errors.Join(ErrHostProtectionStorage, err)
		}
	} else {
		if err = protectionPrivateFile(path, false); err != nil {
			return errors.Join(ErrHostProtectionCorrupt, err)
		}
		info, err := os.Stat(path)
		if err != nil || info.Size() == 0 {
			return ErrHostProtectionCorrupt
		}
	}
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		if err = protectionPrivateFile(path+suffix, true); err != nil {
			return errors.Join(ErrHostProtectionCorrupt, err)
		}
	}
	db, err := storage.OpenSQLite(path)
	if err != nil {
		return errors.Join(ErrHostProtectionCorrupt, err)
	}
	defer func() { result = errors.Join(result, db.Close()) }()
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if err = protectionPrivateFile(path+suffix, suffix != ""); err != nil {
			return errors.Join(ErrHostProtectionCorrupt, err)
		}
	}
	var check string
	if err = db.QueryRowContext(ctx, "PRAGMA quick_check").Scan(&check); err != nil || check != "ok" {
		return errors.Join(ErrHostProtectionCorrupt, err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return errors.Join(ErrHostProtectionStorage, err)
	}
	defer tx.Rollback()
	if fresh {
		if _, err = tx.ExecContext(ctx, hostProtectionSchema); err == nil {
			_, err = tx.ExecContext(ctx, "INSERT INTO host_protection_meta VALUES(1,1,?)", identity)
		}
		if err != nil {
			return errors.Join(ErrHostProtectionStorage, err)
		}
	}
	var version int
	var saved string
	if err = tx.QueryRowContext(ctx, "SELECT version,identity FROM host_protection_meta WHERE singleton=1").Scan(&version, &saved); err != nil || version != 1 || saved != identity {
		return errors.Join(ErrHostProtectionCorrupt, err)
	}
	store := &hostProtectionTransaction{tx: tx, records: map[string]hostProtectionRecord{}}
	if err = store.read(ctx); err != nil {
		return err
	}
	if !fresh && len(store.records) == 0 {
		return ErrHostProtectionCorrupt
	}
	if err = change(store); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return errors.Join(ErrHostProtectionStorage, err)
	}
	// Capture proven settlement before deferred database/ownership close can
	// report a separate failure. A caller must never infer this from a missing
	// row: only an observed successful COMMIT grants this local knowledge.
	if store.afterCommit != nil {
		store.afterCommit()
	}
	return nil
}

// Account configuration directories may not exist on a clean installation.
// Create only missing components below the checked existing ancestry, with
// private permissions on each new component; never repair an existing parent.
func (s *HostProtectionRegistry) ensureParents(parent string) error {
	if err := s.checkAncestry(parent); err != nil {
		return err
	}
	var missing []string
	for cursor := parent; ; cursor = filepath.Dir(cursor) {
		info, err := os.Lstat(cursor)
		if err == nil {
			if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				return ErrAuthority
			}
			break
		}
		if !os.IsNotExist(err) {
			return err
		}
		if len(missing) >= 64 || filepath.Dir(cursor) == cursor {
			return ErrHostProtectionLimit
		}
		missing = append(missing, cursor)
	}
	for index := len(missing) - 1; index >= 0; index-- {
		if err := EnsurePrivateDirectory(missing[index]); err != nil {
			return err
		}
		if err := syncHostProtectionDirectory(filepath.Dir(missing[index])); err != nil {
			return err
		}
	}
	return EnsurePrivateDirectory(parent)
}

// A persistent identity outside the database prevents a missing/replaced store
// from becoming an empty admission registry. Initialization interrupted at any
// point remains unavailable; it never erases or silently repairs prior state.
func (s *HostProtectionRegistry) openIdentity() (string, bool, error) {
	marker := filepath.Join(filepath.Dir(s.directory), "host-protection.identity")
	_, markerErr := os.Lstat(marker)
	_, directoryErr := os.Lstat(s.directory)
	if os.IsNotExist(markerErr) && os.IsNotExist(directoryErr) {
		identity, err := opaqueToken()
		if err != nil {
			return "", false, err
		}
		file, err := os.OpenFile(marker, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return "", false, errors.Join(ErrHostProtectionStorage, err)
		}
		_, writeErr := file.WriteString(identity)
		err = errors.Join(writeErr, ProtectPrivatePath(marker), file.Sync(), file.Close())
		if err != nil {
			return "", false, errors.Join(ErrHostProtectionStorage, err)
		}
		if err = syncHostProtectionDirectory(filepath.Dir(marker)); err != nil {
			return "", false, errors.Join(ErrHostProtectionStorage, err)
		}
		if err = EnsurePrivateDirectory(s.directory); err != nil {
			return "", false, errors.Join(ErrHostProtectionStorage, err)
		}
		if err = syncHostProtectionDirectory(filepath.Dir(s.directory)); err != nil {
			return "", false, errors.Join(ErrHostProtectionStorage, err)
		}
		return identity, true, nil
	}
	if markerErr != nil || directoryErr != nil {
		return "", false, ErrHostProtectionCorrupt
	}
	if err := EnsurePrivateDirectory(s.directory); err != nil {
		return "", false, errors.Join(ErrHostProtectionCorrupt, err)
	}
	if err := protectionPrivateFile(marker, false); err != nil {
		return "", false, errors.Join(ErrHostProtectionCorrupt, err)
	}
	file, err := os.Open(marker)
	if err != nil {
		return "", false, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 65))
	if err != nil || !validCredential(string(data)) {
		return "", false, ErrHostProtectionCorrupt
	}
	return string(data), false, nil
}

func (s *hostProtectionTransaction) read(ctx context.Context) error {
	rows, err := s.tx.QueryContext(ctx, "SELECT id,payload,digest FROM host_protection_records LIMIT ?", hostProtectionMaxRecords+1)
	if err != nil {
		return errors.Join(ErrHostProtectionCorrupt, err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, digest string
		var payload []byte
		if err = rows.Scan(&id, &payload, &digest); err != nil {
			return errors.Join(ErrHostProtectionCorrupt, err)
		}
		if len(s.records) >= hostProtectionMaxRecords {
			return ErrHostProtectionLimit
		}
		var record hostProtectionRecord
		decoder := json.NewDecoder(bytes.NewReader(payload))
		decoder.DisallowUnknownFields()
		if len(payload) > 49152 || protectionHash(string(payload)) != digest || jsonwire.Validate(payload) != nil || decoder.Decode(&record) != nil || decoder.Decode(new(any)) != io.EOF || record.ID != id || !record.valid() {
			return ErrHostProtectionCorrupt
		}
		s.records[id] = record
	}
	return rows.Err()
}

func (r hostProtectionRecord) valid() bool {
	if r.Version != 1 || !validCredential(r.ID) || !protectionLocationValid(r.Root) {
		return false
	}
	if r.Kind == "browser" {
		return r.ID == protectionPathKey(r.Root.Path) && r.Journal == nil && r.Binding == nil && r.Backend == "" && r.Permissions == (Permissions{}) && ((r.State == "protected" && r.Root.Suffix == "" && (r.CapabilityHash == "" || validCredential(r.CapabilityHash))) || (r.State == "pending" && r.Root.Suffix != "" && validCredential(r.CapabilityHash)))
	}
	if r.Kind != "execution" || (r.State != "reserved" && r.State != "released") || r.Root.Suffix != "" || r.Journal == nil || !protectionLocationValid(*r.Journal) || r.Journal.Suffix != "" || r.Binding == nil || !protectionBindingValid(*r.Binding) || !validCredential(r.CapabilityHash) {
		return false
	}
	return r.Backend == "local" || r.Backend == "isolated_linux"
}

func (s *hostProtectionTransaction) insert(ctx context.Context, r hostProtectionRecord) error {
	if len(s.records) >= hostProtectionMaxRecords {
		return ErrHostProtectionLimit
	}
	if _, exists := s.records[r.ID]; exists {
		return ErrHostProtectionConflict
	}
	return s.write(ctx, r, false)
}

// reclaimReleased runs only after an admission's identity/conflict checks, in
// the same transaction as its insertion. A failed insertion or COMMIT rolls
// back reclamation. Neither elapsed time nor process liveness is evidence here:
// only the existing verified cleanup transition makes a row eligible.
func (s *hostProtectionTransaction) reclaimReleased(ctx context.Context) error {
	released := 0
	for _, row := range s.records {
		if row.Kind == "execution" && row.State == "released" {
			released++
		}
	}
	keep := min(hostProtectionReleasedHistory, hostProtectionMaxRecords-(len(s.records)-released)-1)
	if keep < 0 {
		return ErrHostProtectionLimit
	}
	remove := released - keep
	if remove <= 0 {
		return nil
	}
	rows, err := s.tx.QueryContext(ctx, "SELECT id FROM host_protection_records ORDER BY rowid LIMIT ?", hostProtectionMaxRecords)
	if err != nil {
		return errors.Join(ErrHostProtectionStorage, err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			break
		}
		row, ok := s.records[id]
		if !ok {
			err = ErrHostProtectionCorrupt
			break
		}
		if row.Kind == "execution" && row.State == "released" && len(ids) < remove {
			ids = append(ids, id)
		}
	}
	err = errors.Join(err, rows.Err(), rows.Close())
	if err != nil || len(ids) != remove {
		return errors.Join(ErrHostProtectionStorage, err)
	}
	for _, id := range ids {
		result, err := s.tx.ExecContext(ctx, "DELETE FROM host_protection_records WHERE id=?", id)
		if err != nil {
			return errors.Join(ErrHostProtectionStorage, err)
		}
		count, err := result.RowsAffected()
		if err != nil || count != 1 {
			return errors.Join(ErrHostProtectionStorage, err)
		}
		delete(s.records, id)
	}
	return nil
}
func (s *hostProtectionTransaction) replace(ctx context.Context, r hostProtectionRecord) error {
	if _, exists := s.records[r.ID]; !exists {
		return ErrAuthority
	}
	return s.write(ctx, r, true)
}
func (s *hostProtectionTransaction) write(ctx context.Context, r hostProtectionRecord, replace bool) error {
	if !r.valid() {
		return ErrHostProtectionCorrupt
	}
	payload, err := json.Marshal(r)
	if err != nil || len(payload) > 49152 {
		return ErrHostProtectionLimit
	}
	var result sql.Result
	if replace {
		result, err = s.tx.ExecContext(ctx, "UPDATE host_protection_records SET payload=?,digest=? WHERE id=?", payload, protectionHash(string(payload)), r.ID)
	} else {
		result, err = s.tx.ExecContext(ctx, "INSERT INTO host_protection_records(id,payload,digest) VALUES(?,?,?)", r.ID, payload, protectionHash(string(payload)))
	}
	if err != nil {
		return errors.Join(ErrHostProtectionStorage, err)
	}
	n, err := result.RowsAffected()
	if err != nil || n != 1 {
		return errors.Join(ErrHostProtectionStorage, err)
	}
	s.records[r.ID] = r
	return nil
}
