package rag

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
	"unicode/utf8"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

// Reviews are human annotations, not changes to generated claims or proof that
// missing source material was read. Each revision is retained independently.
type SynthesisDecision struct {
	Decision string `json:"decision"`
	Note     string `json:"note"`
}

type SynthesisReview struct {
	AnalysisID     string              `json:"analysis_id"`
	Batch          int                 `json:"batch"`
	BatchID        string              `json:"batch_id"`
	ResponseSHA256 string              `json:"response_sha256"`
	Revision       int                 `json:"revision"`
	SectionChecked bool                `json:"section_checked"`
	Decisions      []SynthesisDecision `json:"decisions"`
	Author         string              `json:"author"`
	UpdatedAt      string              `json:"updated_at"`
}

type SynthesisReviewRequest struct {
	ID               string              `json:"id"`
	Batch            int                 `json:"batch"`
	BatchID          string              `json:"batch_id"`
	ResponseSHA256   string              `json:"response_sha256"`
	ExpectedRevision int                 `json:"expected_revision"`
	SectionChecked   bool                `json:"section_checked"`
	Decisions        []SynthesisDecision `json:"decisions"`
}

func validateSynthesisDecisions(decisions []SynthesisDecision, count int, checked bool) bool {
	if decisions == nil || len(decisions) != count || count > 32 {
		return false
	}
	for _, d := range decisions {
		if (d.Decision != "keep" && d.Decision != "exclude" && d.Decision != "unreviewed") || (checked && d.Decision == "unreviewed") || len(d.Note) > 2048 || !utf8.ValidString(d.Note) {
			return false
		}
	}
	return true
}

// Caller holds the policy lock and has already validated the immutable receipt.
func (s *SQLiteStore) synthesisReviewLocked(ctx context.Context, owner, id string, ordinal int, receipt *document.SynthesisBatchReceipt, version ...int) (*SynthesisReview, error) {
	var raw string
	var revision int
	wanted := 0
	if len(version) > 0 {
		wanted = version[0]
	}
	if wanted < 0 || wanted > 256 {
		return nil, ErrAnalysisInvalid
	}
	err := s.db.QueryRowContext(ctx, `SELECT revision,review_json FROM document_synthesis_reviews WHERE analysis_id=? AND ordinal=? AND (?=0 OR revision=?) ORDER BY revision DESC LIMIT 1`, id, ordinal, wanted, wanted).Scan(&revision, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		if wanted > 0 {
			return nil, ErrAnalysisUnavailable
		}
		d := make([]SynthesisDecision, len(receipt.Result.Candidates))
		for i := range d {
			d[i].Decision = "unreviewed"
		}
		return &SynthesisReview{AnalysisID: id, Batch: ordinal, BatchID: receipt.BatchID, ResponseSHA256: receipt.ResponseSHA256, Decisions: d}, nil
	}
	if err != nil {
		return nil, err
	}
	var r SynthesisReview
	if len(raw) > 128<<10 || json.Unmarshal([]byte(raw), &r) != nil || r.AnalysisID != id || r.Batch != ordinal || r.BatchID != receipt.BatchID || r.ResponseSHA256 != receipt.ResponseSHA256 || r.Revision != revision || r.Revision < 1 || r.Revision > 256 || r.Author != owner || r.UpdatedAt == "" || !validateSynthesisDecisions(r.Decisions, len(receipt.Result.Candidates), r.SectionChecked) {
		return nil, ErrAnalysisInvalid
	}
	if _, err = time.Parse(time.RFC3339Nano, r.UpdatedAt); err != nil {
		return nil, ErrAnalysisInvalid
	}
	return &r, nil
}

func (s *SQLiteStore) SynthesisReview(ctx context.Context, owner, id string, ordinal int, version ...int) (*SynthesisReview, error) {
	page, err := s.SynthesisResults(owner, id, ordinal, 1)
	if err != nil {
		return nil, err
	}
	if len(page.Receipts) != 1 {
		return nil, ErrAnalysisInvalid
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, err = s.synthesisLocked(owner, id); err != nil {
		return nil, err
	}
	return s.synthesisReviewLocked(ctx, owner, id, ordinal, page.Receipts[0], version...)
}

func (s *SQLiteStore) SaveSynthesisReview(ctx context.Context, owner string, req SynthesisReviewRequest) (*SynthesisReview, error) {
	if req.ExpectedRevision < 0 || req.ExpectedRevision >= 256 {
		return nil, ErrAnalysisInvalid
	}
	page, err := s.SynthesisResults(owner, req.ID, req.Batch, 1)
	if err != nil {
		return nil, err
	}
	if len(page.Receipts) != 1 {
		return nil, ErrAnalysisInvalid
	}
	receipt := page.Receipts[0]
	if receipt.BatchID != req.BatchID || receipt.ResponseSHA256 != req.ResponseSHA256 || !validateSynthesisDecisions(req.Decisions, len(receipt.Result.Candidates), req.SectionChecked) {
		return nil, ErrAnalysisInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.synthesisLocked(owner, req.ID)
	if err != nil {
		return nil, err
	}
	if r.State != "awaiting_review" {
		return nil, ErrAnalysisConflict
	}
	previous, err := s.synthesisReviewLocked(ctx, owner, req.ID, req.Batch, receipt)
	if err != nil {
		return nil, err
	}
	if previous.Revision != req.ExpectedRevision {
		return nil, ErrAnalysisConflict
	}
	review := &SynthesisReview{AnalysisID: req.ID, Batch: req.Batch, BatchID: req.BatchID, ResponseSHA256: req.ResponseSHA256, Revision: previous.Revision + 1, SectionChecked: req.SectionChecked, Decisions: req.Decisions, Author: owner, UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	data, err := json.Marshal(review)
	if err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = analysisAccess(tx, owner, r.Plan); err != nil {
		return nil, err
	}
	var latest int
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(revision),0) FROM document_synthesis_reviews WHERE analysis_id=? AND ordinal=?`, req.ID, req.Batch).Scan(&latest); err != nil {
		return nil, err
	}
	if latest != req.ExpectedRevision {
		return nil, ErrAnalysisConflict
	}
	var total, jobBytes int64
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(storage_bytes),0),COALESCE(SUM(CASE WHEN analysis_id=? THEN storage_bytes ELSE 0 END),0) FROM document_synthesis_reviews`, req.ID).Scan(&total, &jobBytes); err != nil {
		return nil, err
	}
	if total+int64(len(data)) > 256<<20 || jobBytes+int64(len(data)) > 8<<20 {
		return nil, ErrAnalysisLimit
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO document_synthesis_reviews(analysis_id,ordinal,revision,review_json,storage_bytes) VALUES(?,?,?,?,?)`, req.ID, req.Batch, review.Revision, string(data), len(data)); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, ref := range r.Plan.Sources {
		p, _, e := documentPolicy(tx, ref.DocumentID)
		if e != nil {
			return nil, e
		}
		if !seen[p.ID] {
			if e = recordCollectionAudit(tx, owner, p.ID, "synthesis_review_saved", req.ID, p.Revision); e != nil {
				return nil, e
			}
			seen[p.ID] = true
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return review, nil
}
