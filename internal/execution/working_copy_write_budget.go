package execution

import (
	"context"
	"encoding/json"
	"math"
)

// Reserve recovery artifacts from the worker's existing allocation before a
// file is staged. A worker cannot open another independent storage allowance.
func (s *WorkingCopies) reserveWriteRecovery(ctx context.Context, id string, owner Binding, extra, ceiling int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrHostUnavailable
	}
	r, err := s.measureLocked(ctx, id, owner)
	if err != nil {
		return err
	}
	if extra < 0 || r.ContentBytes < 0 || r.ArtifactReservedBytes < 0 || r.ArtifactReservedBytes > math.MaxInt64-r.ContentBytes || extra > math.MaxInt64-r.ContentBytes-r.ArtifactReservedBytes {
		return ErrLimit
	}
	needed := max(r.ReservedBytes, r.ContentBytes+r.ArtifactReservedBytes+extra)
	if needed > ceiling {
		return ErrLimit
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var used int64
	if err = tx.QueryRow(`SELECT COALESCE(SUM(reserved_bytes),0) FROM worker_copies WHERE state!='released'`).Scan(&used); err != nil {
		return err
	}
	if used < 0 || used > s.policy.MaxTotalBytes || needed-r.ReservedBytes > s.policy.MaxTotalBytes-used {
		return ErrLimit
	}
	r.ReservedBytes = needed
	r.ArtifactReservedBytes += extra
	r.UsageMeasured = false
	encoded, _ := json.Marshal(r)
	if _, err = tx.Exec(`UPDATE worker_copies SET reserved_bytes=?,record=? WHERE id=? AND state='ready'`, needed, encoded, id); err != nil {
		return err
	}
	return tx.Commit()
}
