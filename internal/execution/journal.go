package execution

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/takuphilchan/offgrid-llm/internal/storage"
)

var ErrConflict = errors.New("execution_operation_conflict")

type Receipt struct {
	State  string          `json:"state"`
	Result json.RawMessage `json:"result,omitempty"`
}

// Journal is dispatch evidence, not a second task store. A claim survives a
// crash even when no conclusive result does. Never restore it from an old job
// checkpoint or delete it as a way to retry an uncertain command.
type Journal struct {
	db    *sql.DB
	owner *storage.Ownership
}

func OpenJournal(directory string) (*Journal, error) {
	owner, err := storage.AcquireOwnership(directory)
	if err != nil {
		return nil, err
	}
	fail := func(err error) (*Journal, error) { owner.Close(); return nil, err }
	path := filepath.Join(directory, "execution-dispatch.sqlite")
	if info, e := os.Lstat(path); e == nil && !info.Mode().IsRegular() {
		return fail(ErrPath)
	} else if e != nil && !os.IsNotExist(e) {
		return fail(e)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return fail(err)
	}
	err = errors.Join(f.Chmod(0600), f.Close())
	if err != nil {
		return fail(err)
	}
	db, err := storage.OpenSQLite(path)
	if err != nil {
		return fail(err)
	}
	var version int
	err = db.QueryRow("PRAGMA user_version").Scan(&version)
	if err == nil && version != 0 && version != 1 {
		err = ErrUnsupported
	}
	if err == nil && version == 0 {
		_, err = db.Exec(`BEGIN; CREATE TABLE execution_operations(id TEXT PRIMARY KEY, digest TEXT NOT NULL, state TEXT NOT NULL CHECK(state IN ('claimed','dispatched','complete','partial','uncertain','rejected')), result BLOB); PRAGMA user_version=1; COMMIT;`)
	}
	if err != nil {
		db.Close()
		return fail(err)
	}
	if _, err = db.Exec(`CREATE TABLE IF NOT EXISTS execution_directories(id TEXT PRIMARY KEY,digest TEXT NOT NULL,directory TEXT NOT NULL)`); err != nil {
		db.Close()
		return fail(err)
	}
	var integrity string
	if _, err = db.Exec(`CREATE TABLE IF NOT EXISTS execution_containers(operation TEXT PRIMARY KEY, daemon TEXT NOT NULL, name TEXT NOT NULL UNIQUE, owner TEXT NOT NULL, container TEXT NOT NULL)`); err != nil {
		db.Close()
		return fail(err)
	}
	if err = db.QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil || integrity != "ok" {
		db.Close()
		return fail(ErrUnsupported)
	}
	// A restarted worker does not adopt PIDs or assume an earlier command ended.
	if _, err = db.Exec(`UPDATE execution_operations SET state='uncertain', result=NULL WHERE state IN ('claimed','dispatched')`); err != nil {
		db.Close()
		return fail(err)
	}
	return &Journal{db: db, owner: owner}, nil
}

// RestoreSession validates the original grant/configuration and restores only
// its provider-local cwd. It cannot authorize another root or actor. Jobs and
// privileges are never restored from directory state.
func (j *Journal) RestoreSession(s *Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	config := s.config
	config.Binding.Job = ""
	encoded, _ := json.Marshal(config)
	hash := digest(encoded)
	if _, err := j.db.Exec(`INSERT INTO execution_directories(id,digest,directory) VALUES(?,?,?) ON CONFLICT(id) DO NOTHING`, config.Binding.Session, hash, config.Directory); err != nil {
		return err
	}
	var previous, directory string
	if err := j.db.QueryRow(`SELECT digest,directory FROM execution_directories WHERE id=?`, config.Binding.Session).Scan(&previous, &directory); err != nil {
		return err
	}
	if previous != hash {
		return ErrConflict
	}
	name, err := s.resolve(directory, ".")
	if err != nil {
		return err
	}
	info, err := s.root.Stat(name)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return ErrPath
	}
	s.config.Directory = name
	return nil
}

func (j *Journal) SaveDirectory(session, directory string) error {
	r, err := j.db.Exec(`UPDATE execution_directories SET directory=? WHERE id=?`, directory, session)
	if err != nil {
		return err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrConflict
	}
	return nil
}

func (j *Journal) Close() error { return errors.Join(j.db.Close(), j.owner.Close()) }

// Claim returns fresh=true only after intent is durable. Existing executing
// claims are uncertain, not permission to repeat a call, including after restart.
func (j *Journal) Claim(ctx context.Context, id string, payload []byte) (Receipt, bool, error) {
	if !validIdentity(id) || len(payload) > MaxFrameBytes {
		return Receipt{}, false, ErrLimit
	}
	hash := digest(payload)
	result, err := j.db.ExecContext(ctx, `INSERT INTO execution_operations(id,digest,state) VALUES(?,?,'claimed') ON CONFLICT(id) DO NOTHING`, id, hash)
	if err != nil {
		return Receipt{}, false, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return Receipt{}, false, err
	}
	if n == 1 {
		return Receipt{State: "claimed"}, true, nil
	}
	var previous string
	var receipt Receipt
	var resultBytes []byte
	err = j.db.QueryRowContext(ctx, `SELECT digest,state,result FROM execution_operations WHERE id=?`, id).Scan(&previous, &receipt.State, &resultBytes)
	if err != nil {
		return Receipt{}, false, err
	}
	if previous != hash {
		return Receipt{}, false, ErrConflict
	}
	receipt.Result = resultBytes
	if receipt.State == "claimed" {
		receipt.State = "uncertain"
	}
	return receipt, false, nil
}

func (j *Journal) Finish(ctx context.Context, id string, payload []byte, receipt Receipt) error {
	if receipt.State != "dispatched" && receipt.State != "complete" && receipt.State != "partial" && receipt.State != "uncertain" && receipt.State != "rejected" {
		return ErrUnsupported
	}
	if len(receipt.Result) > MaxFrameBytes || len(receipt.Result) > 0 && !json.Valid(receipt.Result) {
		return ErrLimit
	}
	result, err := j.db.ExecContext(ctx, `UPDATE execution_operations SET state=?, result=? WHERE id=? AND digest=? AND state='claimed'`, receipt.State, []byte(receipt.Result), id, digest(payload))
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrConflict
	}
	return nil
}

func (j *Journal) Settle(ctx context.Context, id string, payload []byte, result json.RawMessage) error {
	return j.settle(ctx, id, payload, result, "complete")
}

func (j *Journal) settle(ctx context.Context, id string, payload []byte, result json.RawMessage, state string) error {
	if state != "complete" && state != "uncertain" {
		return ErrUnsupported
	}
	if len(result) > MaxFrameBytes || !json.Valid(result) {
		return ErrLimit
	}
	r, err := j.db.ExecContext(ctx, `UPDATE execution_operations SET state=?,result=? WHERE id=? AND digest=? AND state='dispatched'`, state, []byte(result), id, digest(payload))
	if err != nil {
		return err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrConflict
	}
	return nil
}
