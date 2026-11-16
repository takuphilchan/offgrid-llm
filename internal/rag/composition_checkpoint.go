package rag

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

var ErrCompositionReviewsChanged = errors.New("composition review snapshot changed")

func (s *SQLiteStore) beginComposition(ctx context.Context, owner, id string) (*CompositionRecord, analysisLease, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, _, err := s.compositionLocked(ctx, owner, id)
	if err != nil {
		return nil, analysisLease{}, err
	}
	if !r.ReviewsCurrent {
		return nil, analysisLease{}, ErrCompositionReviewsChanged
	}
	if r.State != "queued" && r.State != "interrupted" && r.State != "cancelled" && r.State != "failed" || r.CompletedSteps >= len(r.Plan.Tasks) {
		return nil, analysisLease{}, ErrAnalysisConflict
	}
	r.State = "running"
	r.Attempt++
	r.ErrorCode = ""
	r.UpdatedAt = time.Now().UTC()
	_, err = s.db.ExecContext(ctx, `UPDATE document_compositions SET state='running',attempt=?,error_code='',updated_at=? WHERE id=? AND owner=?`, r.Attempt, r.UpdatedAt, id, owner)
	return r, analysisLease{id, owner, r.Attempt}, err
}

func (s *SQLiteStore) compositionTraversal(ctx context.Context, owner, id string) (*document.Composition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, parent, err := s.compositionLocked(ctx, owner, id)
	if err != nil {
		return nil, err
	}
	if !r.ReviewsCurrent {
		return nil, ErrCompositionReviewsChanged
	}
	return s.compositionTraversalLocked(ctx, owner, r, parent)
}

func (s *SQLiteStore) checkpointComposition(ctx context.Context, lease analysisLease, tr *document.Composition, execution *document.CompositionExecution) error {
	if tr == nil || execution == nil || execution.Receipt == nil {
		return ErrAnalysisInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r, parent, err := s.compositionLocked(ctx, lease.owner, lease.id)
	if err != nil {
		return err
	}
	if !r.ReviewsCurrent {
		return ErrCompositionReviewsChanged
	}
	if r.State != "running" || r.Attempt != lease.attempt || r.Plan.ID != tr.PlanID() || execution.Receipt.Ordinal != r.CompletedSteps {
		return ErrAnalysisConflict
	}
	if err = tr.ValidateReceipt(r.Model, execution.Receipt, execution.RawResponse); err != nil {
		return ErrAnalysisInvalid
	}
	receipt, err := json.Marshal(execution.Receipt)
	if err != nil {
		return err
	}
	raw := execution.RawResponse
	if raw == nil {
		raw = []byte{}
	}
	size := len(receipt) + len(raw)
	if len(receipt) > 64<<10 || len(raw) > 64<<10 {
		return ErrAnalysisLimit
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = analysisAccess(tx, lease.owner, parent.Plan); err != nil {
		return err
	}
	current, err := compositionReviewsCurrent(ctx, tx, parent.ID, r.reviewSnapshot)
	if err != nil {
		return err
	}
	if !current {
		return ErrCompositionReviewsChanged
	}
	var total, jobBytes int64
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(storage_bytes),0),COALESCE(SUM(CASE WHEN composition_id=? THEN storage_bytes ELSE 0 END),0) FROM document_composition_steps`, r.ID).Scan(&total, &jobBytes); err != nil {
		return err
	}
	if total+int64(size) > 1<<30 || jobBytes+int64(size) > 128<<20 {
		return ErrAnalysisLimit
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO document_composition_steps(composition_id,ordinal,receipt_json,raw_response,storage_bytes) VALUES(?,?,?,?,?)`, r.ID, r.CompletedSteps, string(receipt), raw, size); err != nil {
		return err
	}
	state := "running"
	if r.CompletedSteps+1 == len(r.Plan.Tasks) {
		state = "awaiting_review"
	}
	updated, err := tx.ExecContext(ctx, `UPDATE document_compositions SET completed_steps=completed_steps+1,state=?,error_code='',updated_at=? WHERE id=? AND owner=? AND attempt=? AND state='running' AND completed_steps=?`, state, time.Now().UTC(), r.ID, lease.owner, lease.attempt, r.CompletedSteps)
	if err != nil {
		return err
	}
	count, err := updated.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrAnalysisConflict
	}
	return tx.Commit()
}

func (s *SQLiteStore) finishCompositionFailure(lease analysisLease, code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`UPDATE document_compositions SET state='interrupted',error_code=?,updated_at=? WHERE id=? AND owner=? AND attempt=? AND state='running'`, code, time.Now().UTC(), lease.id, lease.owner, lease.attempt)
	return err
}

func (s *SQLiteStore) CancelComposition(owner, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var exists bool
	if err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM document_compositions WHERE id=? AND owner=?)`, id, owner).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrAnalysisUnavailable
	}
	_, err := s.db.Exec(`UPDATE document_compositions SET state='cancelled',updated_at=? WHERE id=? AND owner=? AND state IN ('queued','running','interrupted','failed')`, time.Now().UTC(), id, owner)
	return err
}
