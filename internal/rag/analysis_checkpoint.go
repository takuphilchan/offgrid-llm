package rag

import (
	"encoding/json"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

type analysisLease struct {
	id, owner string
	attempt   int
}

func (s *SQLiteStore) beginAnalysis(owner, id string) (*AnalysisRecord, analysisLease, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.analysisLocked(owner, id)
	if err != nil {
		return nil, analysisLease{}, err
	}
	if r.State != "queued" && r.State != "interrupted" && r.State != "cancelled" && r.State != "failed" {
		return nil, analysisLease{}, ErrAnalysisConflict
	}
	r.State = "running"
	r.Attempt++
	r.ErrorCode = ""
	r.UpdatedAt = time.Now().UTC()
	_, err = s.db.Exec(`UPDATE document_analyses SET state=?,attempt=?,error_code='',updated_at=? WHERE id=? AND owner=?`, r.State, r.Attempt, r.UpdatedAt, id, owner)
	return r, analysisLease{id, owner, r.Attempt}, err
}

func (s *SQLiteStore) checkpointComparison(lease analysisLease, ordinal int, result *document.UnitComparison) error {
	if result == nil {
		return ErrAnalysisInvalid
	}
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	if len(data) > 8<<20 {
		return ErrAnalysisLimit
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.analysisLocked(lease.owner, lease.id)
	if err != nil {
		return err
	}
	if r.State != "running" || r.Attempt != lease.attempt || ordinal != r.CompletedSteps || ordinal >= len(r.Pairs) {
		return ErrAnalysisConflict
	}
	if r.Pairs[ordinal] != result.Pair {
		return ErrAnalysisInvalid
	}
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
	units := 0
	if result.Pair.BeforeUnit != "" {
		units++
	}
	if result.Pair.AfterUnit != "" {
		units++
	}
	state, code := "running", r.ErrorCode
	review := r.ReviewRequired || result.AmbiguousCells > 0
	if review {
		code = "analysis_ambiguous_cells"
	}
	if ordinal+1 == len(r.Pairs) {
		state = "complete"
		if len(r.Plan.Gaps) > 0 || review {
			state = "incomplete"
		}
		if r.CompletedUnits+units != len(r.Plan.Units) {
			return ErrAnalysisInvalid
		}
	}
	_, err = tx.Exec(`UPDATE document_analyses SET completed_steps=completed_steps+1,completed_units=completed_units+?,review_required=?,state=?,error_code=?,updated_at=? WHERE id=? AND attempt=? AND state='running'`, units, review, state, code, time.Now().UTC(), lease.id, lease.attempt)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SQLiteStore) finishAnalysisFailure(lease analysisLease, code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`UPDATE document_analyses SET state='interrupted',error_code=?,updated_at=? WHERE id=? AND owner=? AND attempt=? AND state='running'`, code, time.Now().UTC(), lease.id, lease.owner, lease.attempt)
	return err
}

func (s *SQLiteStore) CancelAnalysis(owner, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Ownership suffices to stop one's own work after source revocation. This
	// response contains no source identity or result text.
	var exists bool
	if err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM document_analyses WHERE id=? AND owner=? AND schema_version IN (1,2))`, id, owner).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrAnalysisUnavailable
	}
	_, err := s.db.Exec(`UPDATE document_analyses SET state='cancelled',updated_at=? WHERE id=? AND owner=? AND state IN ('queued','running','interrupted','failed')`, time.Now().UTC(), id, owner)
	return err
}

type AnalysisResultPage struct {
	AnalysisID   string                   `json:"analysis_id"`
	Step         int                      `json:"step"`
	Offset       int                      `json:"offset"`
	NextOffset   *int                     `json:"next_offset"`
	TotalChanges int                      `json:"total_changes"`
	Result       *document.UnitComparison `json:"result"`
	Previews     []AnalysisChangePreview  `json:"previews"`
}

func (s *SQLiteStore) AnalysisResults(owner, id string, step, offset, limit int) (*AnalysisResultPage, error) {
	if step < 0 || offset < 0 || offset > 400000 || limit < 1 || limit > 100 {
		return nil, ErrAnalysisInvalid
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, err := s.analysisLocked(owner, id)
	if err != nil {
		return nil, err
	}
	if step >= r.CompletedSteps {
		return nil, ErrAnalysisConflict
	}
	var data string
	if err = s.db.QueryRow(`SELECT result_json FROM document_analysis_steps WHERE analysis_id=? AND ordinal=? AND storage_bytes<=?`, id, step, 8<<20).Scan(&data); err != nil {
		return nil, ErrAnalysisUnavailable
	}
	var result document.UnitComparison
	if len(data) > 8<<20 || json.Unmarshal([]byte(data), &result) != nil || result.Pair != r.Pairs[step] || offset > len(result.Changes) {
		return nil, ErrAnalysisInvalid
	}
	end := min(len(result.Changes), offset+limit)
	page := &AnalysisResultPage{AnalysisID: id, Step: step, Offset: offset, TotalChanges: len(result.Changes), Result: &result}
	if end < len(result.Changes) {
		page.NextOffset = &end
	}
	result.Changes = result.Changes[offset:end]
	if page.Previews, err = s.validateAnalysisChanges(r.Plan, result.Pair, result.Changes); err != nil {
		return nil, err
	}
	if err = analysisAccess(s.db, owner, r.Plan); err != nil {
		return nil, err
	}
	return page, nil
}
