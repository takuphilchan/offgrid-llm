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

var ErrAnalysisInvalid = errors.New("invalid document analysis")
var ErrAnalysisConflict = errors.New("document analysis state changed")
var ErrAnalysisUnavailable = errors.New("document analysis unavailable")
var ErrAnalysisLimit = errors.New("document analysis limit exceeded")

type AnalysisRecord struct {
	ID             string                    `json:"id"`
	Plan           *document.AnalysisPlan    `json:"plan"`
	Pairs          []document.ComparisonPair `json:"pairs"`
	State          string                    `json:"state"`
	Attempt        int                       `json:"attempt"`
	CompletedSteps int                       `json:"completed_steps"`
	CompletedUnits int                       `json:"completed_units"`
	ReviewRequired bool                      `json:"review_required"`
	ErrorCode      string                    `json:"error_code,omitempty"`
	CreatedAt      time.Time                 `json:"created_at"`
	UpdatedAt      time.Time                 `json:"updated_at"`
}

const analysisColumns = `id,plan_json,pairs_json,state,attempt,completed_steps,completed_units,review_required,error_code,created_at,updated_at`

func scanAnalysis(row rowScanner) (*AnalysisRecord, error) {
	r := &AnalysisRecord{}
	var plan, pairs string
	err := row.Scan(&r.ID, &plan, &pairs, &r.State, &r.Attempt, &r.CompletedSteps, &r.CompletedUnits, &r.ReviewRequired, &r.ErrorCode, &r.CreatedAt, &r.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrAnalysisUnavailable
	}
	if err != nil {
		return nil, err
	}
	if len(plan) > 4<<20 || len(pairs) > 1<<20 || json.Unmarshal([]byte(plan), &r.Plan) != nil || r.Plan == nil || json.Unmarshal([]byte(pairs), &r.Pairs) != nil || r.Plan.Version != document.AnalysisVersion || r.CompletedSteps < 0 || r.CompletedSteps > len(r.Pairs) || r.CompletedUnits < 0 || r.CompletedUnits > len(r.Plan.Units) {
		return nil, ErrAnalysisInvalid
	}
	return r, nil
}

func analysisAccess(q policyQuery, owner string, plan *document.AnalysisPlan) error {
	if !validKnowledgeActor(owner) || plan == nil {
		return ErrAnalysisUnavailable
	}
	for _, source := range plan.Sources {
		policy, _, err := documentPolicy(q, source.DocumentID)
		if err != nil || !policy.Allows(owner, false) {
			return ErrAnalysisUnavailable
		}
	}
	return nil
}

// CreateComparison uses the same durable SQLite/source-policy boundary as
// imports. It needs neither embeddings nor a generation model. The caller's
// request key binds exact source revisions; replay cannot replace the sources.
func (s *SQLiteStore) CreateComparison(owner, key string, sources []document.SourceRef) (*AnalysisRecord, error) {
	if !validKnowledgeActor(owner) || key == "" || len(key) > 128 || strings.ContainsAny(key, "\x00\r\n") || len(sources) != 2 {
		return nil, ErrAnalysisInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := analysisAccess(s.db, owner, &document.AnalysisPlan{Sources: sources}); err != nil {
		return nil, err
	}
	graphs := []*document.Extraction{}
	for _, ref := range sources {
		g, err := s.extractionLocked(ref.DocumentID, ref.Revision)
		if err != nil {
			return nil, ErrAnalysisUnavailable
		}
		graphs = append(graphs, g)
	}
	plan, err := document.BuildAnalysisPlan("comparison", sources, graphs)
	if err != nil {
		return nil, ErrAnalysisInvalid
	}
	pairs, err := document.PairComparisonUnits(graphs[0], graphs[1])
	if err != nil {
		return nil, ErrAnalysisInvalid
	}
	prior, err := scanAnalysis(s.db.QueryRow(`SELECT `+analysisColumns+` FROM document_analyses WHERE owner=? AND request_key=? AND schema_version=1`, owner, key))
	if err == nil {
		if prior.Plan.ID != plan.ID {
			return nil, ErrAnalysisConflict
		}
		return prior, nil
	}
	if !errors.Is(err, ErrAnalysisUnavailable) {
		return nil, err
	}
	// Request keys share one namespace across analysis workflows. Do not let a
	// synthesis key fall through to a uniqueness error or replace its model.
	var keyExists bool
	if err = s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM document_analyses WHERE owner=? AND request_key=?)`, owner, key).Scan(&keyExists); err != nil {
		return nil, err
	}
	if keyExists {
		return nil, ErrAnalysisConflict
	}
	var count, owned int
	if err = s.db.QueryRow(`SELECT COUNT(*),COALESCE(SUM(owner=?),0) FROM document_analyses`, owner).Scan(&count, &owned); err != nil {
		return nil, err
	}
	if count >= 4096 || owned >= 256 {
		return nil, ErrAnalysisLimit
	}
	var random [16]byte
	if _, err = rand.Read(random[:]); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	r := &AnalysisRecord{ID: "analysis-" + hex.EncodeToString(random[:]), Plan: plan, Pairs: pairs, State: "queued", CreatedAt: now, UpdatedAt: now}
	planJSON, _ := json.Marshal(plan)
	pairsJSON, _ := json.Marshal(pairs)
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = analysisAccess(tx, owner, plan); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(`INSERT INTO document_analyses(id,schema_version,owner,request_key,plan_json,pairs_json,state,created_at,updated_at) VALUES(?,1,?,?,?,?,?,?,?)`, r.ID, owner, key, string(planJSON), string(pairsJSON), r.State, now, now); err != nil {
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

func (s *SQLiteStore) Analysis(owner, id string) (*AnalysisRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.analysisLocked(owner, id)
}
func (s *SQLiteStore) analysisLocked(owner, id string) (*AnalysisRecord, error) {
	r, err := scanAnalysis(s.db.QueryRow(`SELECT `+analysisColumns+` FROM document_analyses WHERE id=? AND owner=? AND schema_version=1`, id, owner))
	if err != nil {
		return nil, err
	}
	if err = analysisAccess(s.db, owner, r.Plan); err != nil {
		return nil, err
	}
	return r, nil
}

func (s *SQLiteStore) RecoverAnalyses() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`UPDATE document_analyses SET state='interrupted',error_code='analysis_interrupted',updated_at=? WHERE schema_version IN (1,2) AND state='running'`, time.Now().UTC())
	if err == nil {
		_, err = s.db.Exec(`UPDATE document_compositions SET state='interrupted',error_code='composition_interrupted',updated_at=? WHERE state='running'`, time.Now().UTC())
	}
	return err
}
