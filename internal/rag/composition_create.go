package rag

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

func (s *SQLiteStore) CreateComposition(ctx context.Context, owner, key, parentID string, model document.SynthesisModelIdentity) (*CompositionRecord, error) {
	if owner == "" || key == "" || len(key) > 128 || strings.ContainsAny(key, "\x00\r\n") || model.Validate() != nil {
		return nil, ErrAnalysisInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	parent, err := s.synthesisLocked(owner, parentID)
	if err != nil {
		return nil, err
	}
	if parent.Plan.Workflow != "summary" || parent.State != "awaiting_review" {
		return nil, ErrAnalysisConflict
	}
	prior, err := scanComposition(s.db.QueryRowContext(ctx, `SELECT `+compositionColumns+` FROM document_compositions WHERE owner=? AND request_key=?`, owner, key))
	if err == nil {
		if prior.AnalysisID != parentID || !reflect.DeepEqual(prior.Model, model) {
			return nil, ErrAnalysisConflict
		}
		prior.ReviewsCurrent, err = s.compositionReviewsCurrentLocked(ctx, parentID, prior.reviewSnapshot)
		return prior, err
	}
	if !errors.Is(err, ErrAnalysisUnavailable) {
		return nil, err
	}
	var total, owned int
	if err = s.db.QueryRowContext(ctx, `SELECT COUNT(*),COALESCE(SUM(owner=?),0) FROM document_compositions`, owner).Scan(&total, &owned); err != nil {
		return nil, err
	}
	if total >= 4096 || owned >= 256 {
		return nil, ErrAnalysisLimit
	}
	seeds, snapshot, _, err := s.compositionSeedsLocked(ctx, owner, parent, nil)
	if err != nil {
		return nil, err
	}
	plan, err := document.BuildCompositionPlan(parentID, parent.Plan, seeds)
	if err != nil {
		return nil, ErrAnalysisInvalid
	}
	planJSON, _ := json.Marshal(plan)
	modelJSON, _ := json.Marshal(model)
	snapshotJSON, _ := json.Marshal(snapshot)
	if len(planJSON) > 32<<20 || len(snapshotJSON) > 1<<20 {
		return nil, ErrAnalysisLimit
	}
	var random [16]byte
	if _, err = rand.Read(random[:]); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	r := &CompositionRecord{ID: "composition-" + hex.EncodeToString(random[:]), AnalysisID: parentID, Plan: plan, Model: model, State: "queued", ReviewsCurrent: true, CreatedAt: now, UpdatedAt: now, reviewSnapshot: snapshot}
	if len(plan.Tasks) == 0 {
		r.State = "awaiting_review"
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = analysisAccess(tx, owner, parent.Plan); err != nil {
		return nil, err
	}
	current, e := compositionReviewsCurrent(ctx, tx, parentID, snapshot)
	if e != nil {
		return nil, e
	}
	if !current {
		return nil, ErrAnalysisConflict
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO document_compositions(id,analysis_id,owner,request_key,plan_json,model_json,review_snapshot_json,state,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, r.ID, parentID, owner, key, string(planJSON), string(modelJSON), string(snapshotJSON), r.State, now, now); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, ref := range parent.Plan.Sources {
		p, _, e := documentPolicy(tx, ref.DocumentID)
		if e != nil {
			return nil, e
		}
		if !seen[p.ID] {
			if e = recordCollectionAudit(tx, owner, p.ID, "composition_created", r.ID, p.Revision); e != nil {
				return nil, e
			}
			seen[p.ID] = true
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return r, nil
}
