package rag

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

// Public status deliberately omits the potentially large merge tree.
type CompositionStatus struct {
	ID             string                          `json:"id"`
	AnalysisID     string                          `json:"analysis_id"`
	PlanID         string                          `json:"plan_id"`
	SourcePlanID   string                          `json:"source_plan_id"`
	Model          document.SynthesisModelIdentity `json:"model"`
	State          string                          `json:"state"`
	Attempt        int                             `json:"attempt"`
	CompletedSteps int                             `json:"completed_steps"`
	TotalSteps     int                             `json:"total_steps"`
	ReviewsCurrent bool                            `json:"reviews_current"`
	ErrorCode      string                          `json:"error_code,omitempty"`
	CreatedAt      time.Time                       `json:"created_at"`
	UpdatedAt      time.Time                       `json:"updated_at"`
}

func (r *CompositionRecord) Status() CompositionStatus {
	return CompositionStatus{r.ID, r.AnalysisID, r.Plan.ID, r.Plan.SourcePlanID, r.Model, r.State, r.Attempt, r.CompletedSteps, len(r.Plan.Tasks), r.ReviewsCurrent, r.ErrorCode, r.CreatedAt, r.UpdatedAt}
}

type CompositionHistory struct {
	AnalysisID string              `json:"analysis_id"`
	Items      []CompositionStatus `json:"items"`
	NextOffset *int                `json:"next_offset"`
}

func (s *SQLiteStore) CompositionHistory(ctx context.Context, owner, parentID string, offset int) (*CompositionHistory, error) {
	if offset < 0 || offset > 256 {
		return nil, ErrAnalysisInvalid
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, err := s.synthesisLocked(owner, parentID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM document_compositions WHERE owner=? AND analysis_id=? ORDER BY created_at DESC,id DESC LIMIT 21 OFFSET ?`, owner, parentID, offset)
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	page := &CompositionHistory{AnalysisID: parentID, Items: []CompositionStatus{}}
	if len(ids) > 20 {
		next := offset + 20
		page.NextOffset = &next
		ids = ids[:20]
	}
	for _, id := range ids {
		r, _, e := s.compositionLocked(ctx, owner, id)
		if e != nil {
			return nil, e
		}
		page.Items = append(page.Items, r.Status())
	}
	return page, nil
}

type CompositionSectionView struct {
	Text         string `json:"text"`
	Contributors int    `json:"contributors"`
	Verification string `json:"verification"`
}
type CompositionGapCount struct {
	Code  string `json:"code"`
	Count int    `json:"count"`
}
type CompositionReportView struct {
	Composition            CompositionStatus        `json:"composition"`
	SourceUnits            int                      `json:"source_units"`
	Sections               []CompositionSectionView `json:"sections"`
	Gaps                   []CompositionGapCount    `json:"gaps"`
	ExtractionGaps         []CompositionGapCount    `json:"extraction_gaps"`
	CandidateGaps          []CompositionGapCount    `json:"candidate_gaps"`
	UnrepresentedBatches   int                      `json:"unrepresented_batches"`
	SemanticReviewRequired bool                     `json:"semantic_review_required"`
}

func compositionGapCounts(counts map[string]int) []CompositionGapCount {
	out := []CompositionGapCount{}
	for code, n := range counts {
		out = append(out, CompositionGapCount{code, n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out
}
func (s *SQLiteStore) compositionReportLocked(ctx context.Context, owner, id string) (*CompositionRecord, *document.CompositionReport, error) {
	r, parent, err := s.compositionLocked(ctx, owner, id)
	if err != nil {
		return nil, nil, err
	}
	if r.State != "awaiting_review" {
		return nil, nil, ErrAnalysisConflict
	}
	tr, err := s.compositionTraversalLocked(ctx, owner, r, parent)
	if err != nil {
		return nil, nil, err
	}
	report, err := tr.Report()
	return r, report, err
}
func (s *SQLiteStore) CompositionReportView(ctx context.Context, owner, id string) (*CompositionReportView, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, report, err := s.compositionReportLocked(ctx, owner, id)
	if err != nil {
		return nil, err
	}
	out := &CompositionReportView{Composition: r.Status(), SourceUnits: report.SourceUnits, Sections: []CompositionSectionView{}, SemanticReviewRequired: true}
	for _, section := range report.Sections {
		out.Sections = append(out.Sections, CompositionSectionView{section.Text, len(section.Contributors), section.Verification})
	}
	counts := map[string]int{}
	for _, gap := range report.Gaps {
		counts[gap.Code]++
	}
	out.Gaps = compositionGapCounts(counts)
	counts = map[string]int{}
	for _, gap := range report.ExtractionGaps {
		counts[gap.Code]++
	}
	out.ExtractionGaps = compositionGapCounts(counts)
	counts, err = s.compositionCandidateGapsLocked(ctx, r.AnalysisID)
	if err != nil {
		return nil, err
	}
	out.CandidateGaps = compositionGapCounts(counts)
	represented := map[int]bool{}
	for _, section := range report.Sections {
		for _, leaf := range section.Contributors {
			represented[leaf.Batch] = true
		}
	}
	out.UnrepresentedBatches = len(r.reviewSnapshot) - len(represented)
	return out, nil
}

func (s *SQLiteStore) compositionCandidateGapsLocked(ctx context.Context, parent string) (map[string]int, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT result_json FROM document_analysis_steps WHERE analysis_id=? ORDER BY ordinal`, parent)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := map[string]int{}
	for rows.Next() {
		var data string
		var checkpoint synthesisCheckpoint
		if err = rows.Scan(&data); err != nil {
			return nil, err
		}
		if len(data) > 2<<20 || json.Unmarshal([]byte(data), &checkpoint) != nil || checkpoint.Receipt == nil || checkpoint.Receipt.Result == nil {
			return nil, ErrAnalysisInvalid
		}
		for _, gap := range checkpoint.Receipt.Result.Gaps {
			counts[gap.Code]++
		}
		if checkpoint.Receipt.Result.NoFindingsReported {
			counts["synthesis_no_findings_reported"]++
		}
	}
	return counts, rows.Err()
}

type CompositionEvidence struct {
	Input     document.CompositionLeaf    `json:"input"`
	Candidate document.SynthesisCandidate `json:"candidate"`
}
type CompositionEvidencePage struct {
	CompositionID string                `json:"composition_id"`
	PlanID        string                `json:"plan_id"`
	Section       int                   `json:"section"`
	Offset        int                   `json:"offset"`
	Total         int                   `json:"total"`
	Items         []CompositionEvidence `json:"items"`
	NextOffset    *int                  `json:"next_offset"`
}

// Source quotes remain separately paginated; a large report never sends an
// unbounded tree or every source passage in its initial response.
func (s *SQLiteStore) CompositionEvidence(ctx context.Context, owner, id string, section, offset int) (*CompositionEvidencePage, error) {
	if section < 0 || section > 3 || offset < 0 || offset > 65536 {
		return nil, ErrAnalysisInvalid
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, report, err := s.compositionReportLocked(ctx, owner, id)
	if err != nil {
		return nil, err
	}
	if section >= len(report.Sections) {
		return nil, ErrAnalysisInvalid
	}
	leaves := report.Sections[section].Contributors
	if offset > len(leaves) {
		return nil, ErrAnalysisInvalid
	}
	out := &CompositionEvidencePage{CompositionID: id, PlanID: r.Plan.ID, Section: section, Offset: offset, Total: len(leaves), Items: []CompositionEvidence{}}
	end := offset + 8
	if end < len(leaves) {
		out.NextOffset = &end
	} else {
		end = len(leaves)
	}
	for _, leaf := range leaves[offset:end] {
		var data string
		var checkpoint synthesisCheckpoint
		if err = s.db.QueryRowContext(ctx, `SELECT result_json FROM document_analysis_steps WHERE analysis_id=? AND ordinal=?`, r.AnalysisID, leaf.Batch).Scan(&data); err != nil {
			return nil, err
		}
		if len(data) > 2<<20 || json.Unmarshal([]byte(data), &checkpoint) != nil || checkpoint.Receipt == nil || checkpoint.Receipt.Result == nil || leaf.CandidateIndex >= len(checkpoint.Receipt.Result.Candidates) {
			return nil, ErrAnalysisInvalid
		}
		out.Items = append(out.Items, CompositionEvidence{leaf, checkpoint.Receipt.Result.Candidates[leaf.CandidateIndex]})
	}
	return out, nil
}
