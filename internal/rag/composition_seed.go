package rag

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

// Caller holds the source-policy lock. Validate the whole completed parent
// traversal once, including empty batches, rather than fetching paged results
// that would rebuild all source graphs for every page.
func (s *SQLiteStore) compositionSeedsLocked(ctx context.Context, owner string, parent *SynthesisRecord, snapshot []int) ([]document.CompositionSeed, []int, []*document.Extraction, error) {
	if parent.State != "awaiting_review" || parent.CompletedBatches < 1 || parent.CompletedBatches > 100000 || parent.CompletedUnits != len(parent.Plan.Units) || parent.UnitCursor != len(parent.Plan.Units) || parent.NextByte != 0 {
		return nil, nil, nil, ErrAnalysisConflict
	}
	if snapshot != nil && len(snapshot) != parent.CompletedBatches {
		return nil, nil, nil, ErrAnalysisInvalid
	}
	graphs := make([]*document.Extraction, len(parent.Plan.Sources))
	for i, ref := range parent.Plan.Sources {
		var err error
		graphs[i], err = s.extractionLocked(ref.DocumentID, ref.Revision)
		if err != nil {
			return nil, nil, nil, ErrAnalysisUnavailable
		}
	}
	traversal, err := document.PrepareSynthesis(parent.Plan, graphs)
	if err != nil {
		return nil, nil, nil, ErrAnalysisInvalid
	}
	seeds := []document.CompositionSeed{}
	reviews := []int{}
	unit, next := 0, parent.Plan.Units[0].Span.Start
	for ordinal := 0; ordinal < parent.CompletedBatches; ordinal++ {
		if err = ctx.Err(); err != nil {
			return nil, nil, nil, err
		}
		var raw string
		if err = s.db.QueryRowContext(ctx, `SELECT result_json FROM document_analysis_steps WHERE analysis_id=? AND ordinal=? AND storage_bytes<=?`, parent.ID, ordinal, 2<<20).Scan(&raw); err != nil {
			return nil, nil, nil, ErrAnalysisInvalid
		}
		var item synthesisCheckpoint
		if len(raw) > 2<<20 || json.Unmarshal([]byte(raw), &item) != nil || item.Batch == nil || unit >= len(parent.Plan.Units) || item.Batch.UnitOrdinal != unit || item.Batch.Primary.Start != next || item.Batch.PrimaryLimit != parent.PrimaryBytes || traversal.ValidateReceipt(item.Batch, parent.Model, item.Receipt, item.RawResponse) != nil {
			return nil, nil, nil, ErrAnalysisInvalid
		}
		var review *SynthesisReview
		if snapshot != nil && snapshot[ordinal] == 0 {
			decisions := make([]SynthesisDecision, len(item.Receipt.Result.Candidates))
			for i := range decisions {
				decisions[i].Decision = "unreviewed"
			}
			review = &SynthesisReview{Decisions: decisions}
		} else {
			version := 0
			if snapshot != nil {
				version = snapshot[ordinal]
				if version < 1 || version > 256 {
					return nil, nil, nil, ErrAnalysisInvalid
				}
			}
			review, err = s.synthesisReviewLocked(ctx, owner, parent.ID, ordinal, item.Receipt, version)
			if err != nil {
				return nil, nil, nil, err
			}
		}
		reviews = append(reviews, review.Revision)
		for i, candidate := range item.Receipt.Result.Candidates {
			seeds = append(seeds, document.CompositionSeed{Batch: ordinal, BatchID: item.Batch.ID, CandidateIndex: i, Candidate: candidate, ReviewRevision: review.Revision, Decision: review.Decisions[i].Decision, SectionChecked: review.SectionChecked})
			if len(seeds) > 65536 {
				return nil, nil, nil, ErrAnalysisLimit
			}
		}
		next = item.Batch.Primary.End
		if item.Batch.EndOfUnit {
			unit++
			next = 0
			if unit < len(parent.Plan.Units) {
				next = parent.Plan.Units[unit].Span.Start
			}
		}
	}
	var count int
	if err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM document_analysis_steps WHERE analysis_id=?`, parent.ID).Scan(&count); err != nil {
		return nil, nil, nil, err
	}
	if count != parent.CompletedBatches || unit != len(parent.Plan.Units) || next != 0 {
		return nil, nil, nil, ErrAnalysisInvalid
	}
	return seeds, reviews, graphs, nil
}

func (s *SQLiteStore) compositionReviewsCurrentLocked(ctx context.Context, parent string, snapshot []int) (bool, error) {
	return compositionReviewsCurrent(ctx, s.db, parent, snapshot)
}

type compositionReviewQuery interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func compositionReviewsCurrent(ctx context.Context, query compositionReviewQuery, parent string, snapshot []int) (bool, error) {
	current := make([]int, len(snapshot))
	rows, err := query.QueryContext(ctx, `SELECT ordinal,MAX(revision) FROM document_synthesis_reviews WHERE analysis_id=? GROUP BY ordinal`, parent)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var batch, version int
		if err = rows.Scan(&batch, &version); err != nil {
			return false, err
		}
		if batch < 0 || batch >= len(snapshot) || version < 1 || version > 256 {
			return false, ErrAnalysisInvalid
		}
		current[batch] = version
	}
	if err = rows.Err(); err != nil {
		return false, err
	}
	for i, v := range snapshot {
		if v < 0 || v > 256 {
			return false, ErrAnalysisInvalid
		}
		if current[i] != v {
			return false, nil
		}
	}
	return true, nil
}
