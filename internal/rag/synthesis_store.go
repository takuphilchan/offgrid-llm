package rag

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

// SynthesisRecord is private to the job owner and all current source audiences.
// Traversal completion is not answer approval: candidates await support review.
type SynthesisRecord struct {
	ID               string                          `json:"id"`
	Plan             *document.AnalysisPlan          `json:"plan"`
	Model            document.SynthesisModelIdentity `json:"model"`
	PrimaryBytes     int                             `json:"primary_bytes"`
	State            string                          `json:"state"`
	Attempt          int                             `json:"attempt"`
	CompletedBatches int                             `json:"completed_batches"`
	CompletedUnits   int                             `json:"completed_units"`
	UnitCursor       int                             `json:"unit_cursor"`
	NextByte         int                             `json:"next_byte"`
	ReviewRequired   bool                            `json:"review_required"`
	ErrorCode        string                          `json:"error_code,omitempty"`
	CreatedAt        time.Time                       `json:"created_at"`
	UpdatedAt        time.Time                       `json:"updated_at"`
}

const synthesisColumns = `a.id,a.plan_json,a.state,a.attempt,a.completed_steps,a.completed_units,a.review_required,a.error_code,a.created_at,a.updated_at,c.model_json,c.primary_bytes,c.unit_cursor,c.next_byte`
const synthesisJoin = ` FROM document_analyses a JOIN document_synthesis_configs c ON c.analysis_id=a.id `

func scanSynthesis(row rowScanner) (*SynthesisRecord, error) {
	r := &SynthesisRecord{}
	var plan, model string
	err := row.Scan(&r.ID, &plan, &r.State, &r.Attempt, &r.CompletedBatches, &r.CompletedUnits, &r.ReviewRequired, &r.ErrorCode, &r.CreatedAt, &r.UpdatedAt, &model, &r.PrimaryBytes, &r.UnitCursor, &r.NextByte)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrAnalysisUnavailable
	}
	if err != nil {
		return nil, err
	}
	if len(plan) > 4<<20 || len(model) > 4096 || json.Unmarshal([]byte(plan), &r.Plan) != nil || r.Plan == nil || json.Unmarshal([]byte(model), &r.Model) != nil || r.Model.Validate() != nil {
		return nil, ErrAnalysisInvalid
	}
	if r.Plan.Version != document.AnalysisVersion || (r.Plan.Workflow != "summary" && r.Plan.Workflow != "requirements") || len(r.Plan.Units) == 0 || r.PrimaryBytes < 1024 || r.PrimaryBytes > 8192 || r.CompletedBatches < 0 || r.CompletedUnits != r.UnitCursor || r.UnitCursor < 0 || r.UnitCursor > len(r.Plan.Units) || r.Attempt < 0 {
		return nil, ErrAnalysisInvalid
	}
	if r.UnitCursor < len(r.Plan.Units) {
		u := r.Plan.Units[r.UnitCursor]
		if r.NextByte < u.Span.Start || r.NextByte > u.Span.End {
			return nil, ErrAnalysisInvalid
		}
	} else if r.NextByte != 0 {
		return nil, ErrAnalysisInvalid
	}
	return r, nil
}

func (s *SQLiteStore) Synthesis(owner, id string) (*SynthesisRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.synthesisLocked(owner, id)
}
func (s *SQLiteStore) synthesisLocked(owner, id string) (*SynthesisRecord, error) {
	r, err := scanSynthesis(s.db.QueryRow(`SELECT `+synthesisColumns+synthesisJoin+` WHERE a.id=? AND a.owner=? AND a.schema_version=2`, id, owner))
	if err != nil {
		return nil, err
	}
	if err = analysisAccess(s.db, owner, r.Plan); err != nil {
		return nil, err
	}
	return r, nil
}

func (s *SQLiteStore) beginSynthesis(owner, id string) (*SynthesisRecord, analysisLease, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.synthesisLocked(owner, id)
	if err != nil {
		return nil, analysisLease{}, err
	}
	if r.State != "queued" && r.State != "interrupted" && r.State != "cancelled" && r.State != "failed" {
		return nil, analysisLease{}, ErrAnalysisConflict
	}
	if r.UnitCursor >= len(r.Plan.Units) {
		return nil, analysisLease{}, ErrAnalysisConflict
	}
	r.State = "running"
	r.Attempt++
	r.ErrorCode = ""
	r.UpdatedAt = time.Now().UTC()
	_, err = s.db.Exec(`UPDATE document_analyses SET state='running',attempt=?,error_code='',updated_at=? WHERE id=? AND owner=? AND schema_version=2`, r.Attempt, r.UpdatedAt, id, owner)
	return r, analysisLease{id, owner, r.Attempt}, err
}

func (s *SQLiteStore) synthesisTraversal(owner, id string) (*document.SynthesisTraversal, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, err := s.synthesisLocked(owner, id)
	if err != nil {
		return nil, err
	}
	graphs := make([]*document.Extraction, 0, len(r.Plan.Sources))
	for _, ref := range r.Plan.Sources {
		g, err := s.extractionLocked(ref.DocumentID, ref.Revision)
		if err != nil {
			return nil, ErrAnalysisUnavailable
		}
		graphs = append(graphs, g)
	}
	traversal, err := document.PrepareSynthesis(r.Plan, graphs)
	if err != nil {
		return nil, ErrAnalysisInvalid
	}
	return traversal, nil
}

// ValidateAnalysisSources checks immutable identities and current audiences
// before a service spends resources loading the explicitly selected model.
func (s *SQLiteStore) ValidateAnalysisSources(owner, workflow string, refs []document.SourceRef) error {
	if len(refs) < 1 || len(refs) > 16 {
		return ErrAnalysisInvalid
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := analysisAccess(s.db, owner, &document.AnalysisPlan{Sources: refs}); err != nil {
		return err
	}
	graphs := make([]*document.Extraction, 0, len(refs))
	for _, ref := range refs {
		g, err := s.extractionLocked(ref.DocumentID, ref.Revision)
		if err != nil {
			return ErrAnalysisUnavailable
		}
		graphs = append(graphs, g)
	}
	plan, err := document.BuildAnalysisPlan(workflow, refs, graphs)
	if err != nil || len(plan.Units) == 0 {
		return ErrAnalysisInvalid
	}
	return nil
}
