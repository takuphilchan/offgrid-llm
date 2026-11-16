package rag

import (
	"encoding/json"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

// This envelope is private retained evidence, not a public response. Results
// expose the validated receipt only; raw model output stays within job storage.
type synthesisCheckpoint struct {
	Batch       *document.SynthesisBatch        `json:"batch"`
	Receipt     *document.SynthesisBatchReceipt `json:"receipt"`
	RawResponse []byte                          `json:"raw_response"`
}

func (s *SQLiteStore) checkpointSynthesis(lease analysisLease, ordinal int, traversal *document.SynthesisTraversal, batch *document.SynthesisBatch, execution *document.SynthesisExecution) error {
	if traversal == nil || batch == nil || execution == nil || execution.Receipt == nil {
		return ErrAnalysisInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.synthesisLocked(lease.owner, lease.id)
	if err != nil {
		return err
	}
	if r.State != "running" || r.Attempt != lease.attempt || ordinal != r.CompletedBatches || r.UnitCursor >= len(r.Plan.Units) {
		return ErrAnalysisConflict
	}
	if traversal.PlanID() != r.Plan.ID || batch.PlanID != r.Plan.ID || batch.UnitOrdinal != r.UnitCursor || batch.Primary.Start != r.NextByte || batch.PrimaryLimit != r.PrimaryBytes {
		return ErrAnalysisInvalid
	}
	if err = traversal.ValidateReceipt(batch, r.Model, execution.Receipt, execution.RawResponse); err != nil {
		return ErrAnalysisInvalid
	}
	data, err := json.Marshal(synthesisCheckpoint{batch, execution.Receipt, execution.RawResponse})
	if err != nil {
		return err
	}
	if len(data) > 2<<20 {
		return ErrAnalysisLimit
	}
	unit, next := r.UnitCursor, batch.Primary.End
	if batch.EndOfUnit {
		unit++
		next = 0
		if unit < len(r.Plan.Units) {
			next = r.Plan.Units[unit].Span.Start
		}
	}
	state := "running"
	if unit == len(r.Plan.Units) {
		state = "awaiting_review"
	}
	review := r.ReviewRequired || execution.Receipt.Result.SemanticReviewRequired || len(execution.Receipt.Result.Gaps) > 0
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = analysisAccess(tx, lease.owner, r.Plan); err != nil {
		return err
	}
	var total, jobBytes int64
	if err = tx.QueryRow(`SELECT COALESCE(SUM(storage_bytes),0),COALESCE(SUM(CASE WHEN analysis_id=? THEN storage_bytes ELSE 0 END),0) FROM document_analysis_steps`, lease.id).Scan(&total, &jobBytes); err != nil {
		return err
	}
	if total+int64(len(data)) > 1<<30 || jobBytes+int64(len(data)) > 64<<20 {
		return ErrAnalysisLimit
	}
	if _, err = tx.Exec(`INSERT INTO document_analysis_steps(analysis_id,ordinal,result_json,storage_bytes) VALUES(?,?,?,?)`, lease.id, ordinal, string(data), len(data)); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE document_synthesis_configs SET unit_cursor=?,next_byte=? WHERE analysis_id=?`, unit, next, lease.id); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE document_analyses SET completed_steps=completed_steps+1,completed_units=?,review_required=?,state=?,error_code='',updated_at=? WHERE id=? AND owner=? AND attempt=? AND state='running'`, unit, review, state, time.Now().UTC(), lease.id, lease.owner, lease.attempt); err != nil {
		return err
	}
	return tx.Commit()
}

type SynthesisResultPage struct {
	AnalysisID   string                            `json:"analysis_id"`
	Offset       int                               `json:"offset"`
	NextOffset   *int                              `json:"next_offset"`
	TotalBatches int                               `json:"total_batches"`
	Receipts     []*document.SynthesisBatchReceipt `json:"receipts"`
}

// SynthesisResults revalidates private responses against immutable source graphs
// before returning candidates. A valid quote does not certify the model's claim.
func (s *SQLiteStore) SynthesisResults(owner, id string, offset, limit int) (*SynthesisResultPage, error) {
	if offset < 0 || offset > 100000 || limit < 1 || limit > 8 {
		return nil, ErrAnalysisInvalid
	}
	traversal, err := s.synthesisTraversal(owner, id)
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, err := s.synthesisLocked(owner, id)
	if err != nil {
		return nil, err
	}
	if offset > r.CompletedBatches || traversal.PlanID() != r.Plan.ID {
		return nil, ErrAnalysisConflict
	}
	rows, err := s.db.Query(`SELECT ordinal,result_json FROM document_analysis_steps WHERE analysis_id=? AND ordinal>=? AND storage_bytes<=? ORDER BY ordinal LIMIT ?`, id, offset, 2<<20, limit)
	if err != nil {
		return nil, err
	}
	page := &SynthesisResultPage{AnalysisID: id, Offset: offset, TotalBatches: r.CompletedBatches, Receipts: []*document.SynthesisBatchReceipt{}}
	for rows.Next() {
		var ordinal int
		var raw string
		var item synthesisCheckpoint
		if err = rows.Scan(&ordinal, &raw); err != nil {
			rows.Close()
			return nil, err
		}
		if ordinal != offset+len(page.Receipts) || len(raw) > 2<<20 || json.Unmarshal([]byte(raw), &item) != nil || item.Batch == nil || item.Batch.PlanID != r.Plan.ID || traversal.ValidateReceipt(item.Batch, r.Model, item.Receipt, item.RawResponse) != nil {
			rows.Close()
			return nil, ErrAnalysisInvalid
		}
		page.Receipts = append(page.Receipts, item.Receipt)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	end := offset + len(page.Receipts)
	if end != min(r.CompletedBatches, offset+limit) {
		return nil, ErrAnalysisInvalid
	}
	if end < r.CompletedBatches {
		page.NextOffset = &end
	}
	if err = analysisAccess(s.db, owner, r.Plan); err != nil {
		return nil, err
	}
	return page, nil
}
