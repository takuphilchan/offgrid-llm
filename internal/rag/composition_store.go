package rag

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

type CompositionRecord struct {
	ID             string                          `json:"id"`
	AnalysisID     string                          `json:"analysis_id"`
	Plan           *document.CompositionPlan       `json:"plan"`
	Model          document.SynthesisModelIdentity `json:"model"`
	State          string                          `json:"state"`
	Attempt        int                             `json:"attempt"`
	CompletedSteps int                             `json:"completed_steps"`
	ReviewsCurrent bool                            `json:"reviews_current"`
	ErrorCode      string                          `json:"error_code,omitempty"`
	CreatedAt      time.Time                       `json:"created_at"`
	UpdatedAt      time.Time                       `json:"updated_at"`
	reviewSnapshot []int
}

const compositionColumns = `id,analysis_id,plan_json,model_json,review_snapshot_json,state,attempt,completed_steps,error_code,created_at,updated_at`

func scanComposition(row rowScanner) (*CompositionRecord, error) {
	r := &CompositionRecord{}
	var plan, model, reviews string
	err := row.Scan(&r.ID, &r.AnalysisID, &plan, &model, &reviews, &r.State, &r.Attempt, &r.CompletedSteps, &r.ErrorCode, &r.CreatedAt, &r.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrAnalysisUnavailable
	}
	if err != nil {
		return nil, err
	}
	if len(plan) > 32<<20 || len(model) > 4096 || len(reviews) > 1<<20 || json.Unmarshal([]byte(plan), &r.Plan) != nil || r.Plan == nil || r.Plan.Protocol != document.CompositionProtocol || r.Plan.AnalysisID != r.AnalysisID || json.Unmarshal([]byte(model), &r.Model) != nil || r.Model.Validate() != nil || json.Unmarshal([]byte(reviews), &r.reviewSnapshot) != nil || len(r.reviewSnapshot) < 1 || len(r.reviewSnapshot) > 100000 || r.CompletedSteps < 0 || r.CompletedSteps > len(r.Plan.Tasks) || r.Attempt < 0 {
		return nil, ErrAnalysisInvalid
	}
	switch r.State {
	case "queued", "running", "interrupted", "cancelled", "failed":
	case "awaiting_review":
		if r.CompletedSteps != len(r.Plan.Tasks) {
			return nil, ErrAnalysisInvalid
		}
	default:
		return nil, ErrAnalysisInvalid
	}
	for _, revision := range r.reviewSnapshot {
		if revision < 0 || revision > 256 {
			return nil, ErrAnalysisInvalid
		}
	}
	return r, nil
}

func (s *SQLiteStore) compositionLocked(ctx context.Context, owner, id string) (*CompositionRecord, *SynthesisRecord, error) {
	r, err := scanComposition(s.db.QueryRowContext(ctx, `SELECT `+compositionColumns+` FROM document_compositions WHERE id=? AND owner=?`, id, owner))
	if err != nil {
		return nil, nil, err
	}
	parent, err := s.synthesisLocked(owner, r.AnalysisID)
	if err != nil {
		return nil, nil, err
	}
	if r.Plan.SourcePlanID != parent.Plan.ID || len(r.reviewSnapshot) != parent.CompletedBatches {
		return nil, nil, ErrAnalysisInvalid
	}
	r.ReviewsCurrent, err = s.compositionReviewsCurrentLocked(ctx, parent.ID, r.reviewSnapshot)
	if err != nil {
		return nil, nil, err
	}
	return r, parent, nil
}

func (s *SQLiteStore) Composition(ctx context.Context, owner, id string) (*CompositionRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, _, err := s.compositionLocked(ctx, owner, id)
	return r, err
}

// Hydrate and revalidate immutable source and review snapshots before replaying
// committed merge receipts. Access is checked independently on every read.
func (s *SQLiteStore) compositionTraversalLocked(ctx context.Context, owner string, r *CompositionRecord, parent *SynthesisRecord) (*document.Composition, error) {
	seeds, _, graphs, err := s.compositionSeedsLocked(ctx, owner, parent, r.reviewSnapshot)
	if err != nil {
		return nil, err
	}
	tr, err := document.PrepareComposition(r.Plan, parent.Plan, graphs, seeds)
	if err != nil {
		return nil, ErrAnalysisInvalid
	}
	rows, err := s.db.QueryContext(ctx, `SELECT ordinal,receipt_json,raw_response FROM document_composition_steps WHERE composition_id=? AND storage_bytes<=? ORDER BY ordinal`, r.ID, 128<<10)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		var ordinal int
		var data string
		var raw []byte
		var receipt document.CompositionReceipt
		if err = rows.Scan(&ordinal, &data, &raw); err != nil {
			return nil, err
		}
		if ordinal != count || len(data) > 64<<10 || len(raw) > 64<<10 || json.Unmarshal([]byte(data), &receipt) != nil || tr.Apply(r.Model, &receipt, raw) != nil {
			return nil, ErrAnalysisInvalid
		}
		count++
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if count != r.CompletedSteps {
		return nil, ErrAnalysisInvalid
	}
	return tr, nil
}

func (s *SQLiteStore) CompositionReport(ctx context.Context, owner, id string) (*document.CompositionReport, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, parent, err := s.compositionLocked(ctx, owner, id)
	if err != nil {
		return nil, false, err
	}
	if r.State != "awaiting_review" {
		return nil, false, ErrAnalysisConflict
	}
	tr, err := s.compositionTraversalLocked(ctx, owner, r, parent)
	if err != nil {
		return nil, false, err
	}
	report, err := tr.Report()
	return report, r.ReviewsCurrent, err
}
