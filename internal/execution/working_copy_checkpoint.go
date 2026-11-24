package execution

import (
	"context"
	"database/sql"
)

// Keep the source record/digest stable while publishing cleanup checkpoints.
// Strings compare the exact stored bytes without interpreting a new authority.
type copyCleanupRecord struct {
	State, Digest, Record string
	Reserved              int64
}

func readCopyCleanupRecord(ctx context.Context, reader copyRowReader, id string) (copyCleanupRecord, error) {
	var record copyCleanupRecord
	err := reader.QueryRowContext(ctx, `SELECT state,digest,record,reserved_bytes FROM worker_copies WHERE id=?`, id).Scan(&record.State, &record.Digest, &record.Record, &record.Reserved)
	return record, err
}

func requireCopyCheckpointWrite(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrRevision
	}
	return nil
}

func (s *WorkingCopies) writeCopyOwnership(ctx context.Context, id string, before, after workingCopyOwnership) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	current, err := readCopyOwnership(ctx, tx, id)
	if err != nil {
		return err
	}
	if current != before {
		return ErrRevision
	}
	record, err := readCopyCleanupRecord(ctx, tx, id)
	if err != nil {
		return err
	}
	if err = requireCopyCheckpointWrite(tx.ExecContext(ctx, `UPDATE worker_copy_ownership SET version=?,identity=?,empty=?,in_use=?,quarantine=?,detached=? WHERE copy_id=?`, after.Version, after.Identity, after.Empty, after.InUse, after.Quarantine, after.Detached, id)); err != nil {
		return err
	}
	confirmed, err := readCopyOwnership(ctx, tx, id)
	if err != nil {
		return err
	}
	confirmedRecord, err := readCopyCleanupRecord(ctx, tx, id)
	if err != nil {
		return err
	}
	if confirmed != after || confirmedRecord != record {
		return ErrRevision
	}
	return tx.Commit()
}

func (s *WorkingCopies) writeCopyCleanupState(ctx context.Context, id, state string, release bool) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	before, err := readCopyCleanupRecord(ctx, tx, id)
	if err != nil {
		return err
	}
	if state == "uncertain" && before.State == "released" {
		return nil
	}
	owner, err := readCopyOwnership(ctx, tx, id)
	if err != nil {
		return err
	}
	after := before
	after.State = state
	if release {
		after.Reserved = 0
	}
	if err = requireCopyCheckpointWrite(tx.ExecContext(ctx, `UPDATE worker_copies SET state=?,reserved_bytes=? WHERE id=?`, after.State, after.Reserved, id)); err != nil {
		return err
	}
	confirmed, err := readCopyCleanupRecord(ctx, tx, id)
	if err != nil {
		return err
	}
	confirmedOwner, err := readCopyOwnership(ctx, tx, id)
	if err != nil {
		return err
	}
	if confirmed != after || confirmedOwner != owner {
		return ErrRevision
	}
	return tx.Commit()
}
