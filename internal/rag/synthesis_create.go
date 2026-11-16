package rag

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

// CreateSynthesis accepts installed-model identity supplied by the service,
// never resolves/downloads a model, and never changes an existing job's model.
func (s *SQLiteStore) CreateSynthesis(owner, key, workflow string, sources []document.SourceRef, model document.SynthesisModelIdentity, primaryBytes int) (*SynthesisRecord, error) {
	if !validKnowledgeActor(owner) || key == "" || len(key) > 128 || strings.ContainsAny(key, "\x00\r\n") || (workflow != "summary" && workflow != "requirements") || len(sources) < 1 || len(sources) > 16 || model.Validate() != nil || primaryBytes < 1024 || primaryBytes > 8192 {
		return nil, ErrAnalysisInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := analysisAccess(s.db, owner, &document.AnalysisPlan{Sources: sources}); err != nil {
		return nil, err
	}
	graphs := make([]*document.Extraction, 0, len(sources))
	for _, ref := range sources {
		g, err := s.extractionLocked(ref.DocumentID, ref.Revision)
		if err != nil {
			return nil, ErrAnalysisUnavailable
		}
		graphs = append(graphs, g)
	}
	plan, err := document.BuildAnalysisPlan(workflow, sources, graphs)
	if err != nil || len(plan.Units) == 0 {
		return nil, ErrAnalysisInvalid
	}
	planJSON, err := json.Marshal(plan)
	if err != nil || len(planJSON) > 4<<20 {
		return nil, ErrAnalysisLimit
	}
	modelJSON, err := json.Marshal(model)
	if err != nil || len(modelJSON) > 4096 {
		return nil, ErrAnalysisInvalid
	}
	var priorID string
	var version int
	err = s.db.QueryRow(`SELECT id,schema_version FROM document_analyses WHERE owner=? AND request_key=?`, owner, key).Scan(&priorID, &version)
	if err == nil {
		if version != 2 {
			return nil, ErrAnalysisConflict
		}
		prior, err := s.synthesisLocked(owner, priorID)
		if err != nil {
			return nil, err
		}
		if prior.Plan.ID != plan.ID || prior.Model != model || prior.PrimaryBytes != primaryBytes {
			return nil, ErrAnalysisConflict
		}
		return prior, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	var total, owned int
	if err = s.db.QueryRow(`SELECT COUNT(*),COALESCE(SUM(owner=?),0) FROM document_analyses`, owner).Scan(&total, &owned); err != nil {
		return nil, err
	}
	if total >= 4096 || owned >= 256 {
		return nil, ErrAnalysisLimit
	}
	var random [16]byte
	if _, err = rand.Read(random[:]); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	r := &SynthesisRecord{ID: "analysis-" + hex.EncodeToString(random[:]), Plan: plan, Model: model, PrimaryBytes: primaryBytes, State: "queued", NextByte: plan.Units[0].Span.Start, ReviewRequired: len(plan.Gaps) > 0, CreatedAt: now, UpdatedAt: now}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = analysisAccess(tx, owner, plan); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(`INSERT INTO document_analyses(id,schema_version,owner,request_key,plan_json,pairs_json,state,review_required,created_at,updated_at) VALUES(?,2,?,?,?,'[]',?,?,?,?)`, r.ID, owner, key, string(planJSON), r.State, r.ReviewRequired, now, now); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(`INSERT INTO document_synthesis_configs(analysis_id,model_json,primary_bytes,unit_cursor,next_byte) VALUES(?,?,?,0,?)`, r.ID, string(modelJSON), primaryBytes, r.NextByte); err != nil {
		return nil, err
	}
	for i, ref := range sources {
		if _, err = tx.Exec(`INSERT INTO document_analysis_sources(analysis_id,ordinal,document_id,revision) VALUES(?,?,?,?)`, r.ID, i, ref.DocumentID, ref.Revision); err != nil {
			return nil, err
		}
		policy, _, err := documentPolicy(tx, ref.DocumentID)
		if err != nil {
			return nil, err
		}
		if err = recordCollectionAudit(tx, owner, policy.ID, "analysis_created", r.ID, policy.Revision); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return r, nil
}
