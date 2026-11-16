package rag

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

// ExportSynthesis exports review material, never an approved or exhaustive answer.
// Validate the entire retained traversal, not just the currently visible page.
// Policy changes are excluded while assembling the bounded artifact; delivery
// rechecks access. Raw model responses never leave private checkpoint storage.
func (s *SQLiteStore) ExportSynthesis(ctx context.Context, owner, id, format string) (*AnalysisExport, error) {
	if format != "json" && format != "csv" && format != "markdown" {
		return nil, ErrAnalysisInvalid
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, err := s.synthesisLocked(owner, id)
	if err != nil {
		return nil, err
	}
	if r.State != "awaiting_review" {
		return nil, ErrAnalysisConflict
	}
	if r.CompletedUnits != len(r.Plan.Units) || r.UnitCursor != len(r.Plan.Units) || r.NextByte != 0 || r.CompletedBatches < 1 {
		return nil, ErrAnalysisInvalid
	}
	graphs := make([]*document.Extraction, len(r.Plan.Sources))
	for i, ref := range r.Plan.Sources {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		graphs[i], err = s.extractionLocked(ref.DocumentID, ref.Revision)
		if err != nil {
			return nil, ErrAnalysisUnavailable
		}
	}
	traversal, err := document.PrepareSynthesis(r.Plan, graphs)
	if err != nil {
		return nil, ErrAnalysisInvalid
	}
	out := newSynthesisExportWriter(format)
	if err = out.record("analysis", map[string]any{
		"id": r.ID, "workflow": r.Plan.Workflow, "state": r.State,
		"plan_id": r.Plan.ID, "model": r.Model, "created_at": r.CreatedAt,
		"completed_units": r.CompletedUnits, "total_units": len(r.Plan.Units),
		"completed_batches": r.CompletedBatches, "semantic_review_required": true,
		"scope": "draft_candidates_from_retained_text", "verification": "source_identity_only",
		"notice":    "Source quotations were validated. Model claims and omissions require human review. Traversing retained text does not establish complete extraction or a complete review of the original documents.",
		"retention": "Exported copies have separate retention; later deletion or access revocation cannot recall them.",
	}); err != nil {
		return nil, err
	}
	for i, ref := range r.Plan.Sources {
		var name string
		if err = s.db.QueryRowContext(ctx, `SELECT name FROM documents WHERE id=?`, ref.DocumentID).Scan(&name); err != nil {
			return nil, ErrAnalysisUnavailable
		}
		if err = out.record("source", map[string]any{"source": i, "reference": ref, "display_name": name}); err != nil {
			return nil, err
		}
	}
	for _, unit := range r.Plan.Units {
		if err = out.record("unit", unit); err != nil {
			return nil, err
		}
	}
	for _, gap := range r.Plan.Gaps {
		if err = out.record("gap", gap); err != nil {
			return nil, err
		}
	}
	unit, next := 0, r.Plan.Units[0].Span.Start
	checked, kept, excluded, pending := 0, 0, 0, 0
	for ordinal := 0; ordinal < r.CompletedBatches; ordinal++ {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		var raw string
		if err = s.db.QueryRowContext(ctx, `SELECT result_json FROM document_analysis_steps WHERE analysis_id=? AND ordinal=? AND storage_bytes<=?`, id, ordinal, 2<<20).Scan(&raw); err != nil {
			return nil, ErrAnalysisInvalid
		}
		var item synthesisCheckpoint
		if len(raw) > 2<<20 || json.Unmarshal([]byte(raw), &item) != nil || item.Batch == nil || unit >= len(r.Plan.Units) || item.Batch.UnitOrdinal != unit || item.Batch.Primary.Start != next || item.Batch.PrimaryLimit != r.PrimaryBytes || traversal.ValidateReceipt(item.Batch, r.Model, item.Receipt, item.RawResponse) != nil {
			return nil, ErrAnalysisInvalid
		}
		receipt := item.Receipt
		review, reviewErr := s.synthesisReviewLocked(ctx, owner, id, ordinal, receipt)
		if reviewErr != nil {
			return nil, reviewErr
		}
		if review.SectionChecked {
			checked++
		}
		for _, d := range review.Decisions {
			switch d.Decision {
			case "keep":
				kept++
			case "exclude":
				excluded++
			default:
				pending++
			}
		}
		if err = out.record("section_review", review); err != nil {
			return nil, err
		}
		if err = out.record("batch", map[string]any{"ordinal": ordinal, "batch_id": receipt.BatchID, "unit_ordinal": unit, "primary": receipt.Primary, "end_of_unit": receipt.EndOfUnit, "method": receipt.Method, "validation": receipt.Validation, "response_sha256": receipt.ResponseSHA256, "model_identity": receipt.ModelIdentity, "prompt_tokens": receipt.PromptTokens, "no_findings_reported": receipt.Result.NoFindingsReported, "semantic_review_required": receipt.Result.SemanticReviewRequired}); err != nil {
			return nil, err
		}
		for _, gap := range receipt.Result.Gaps {
			if err = out.record("batch_gap", map[string]any{"batch": ordinal, "gap": gap}); err != nil {
				return nil, err
			}
		}
		for index, candidate := range receipt.Result.Candidates {
			if err = out.record("candidate", map[string]any{"batch": ordinal, "candidate": index, "value": candidate, "human_review": review.Decisions[index], "review_revision": review.Revision}); err != nil {
				return nil, err
			}
		}
		next = item.Batch.Primary.End
		if item.Batch.EndOfUnit {
			unit++
			next = 0
			if unit < len(r.Plan.Units) {
				next = r.Plan.Units[unit].Span.Start
			}
		}
	}
	var stored int
	if err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM document_analysis_steps WHERE analysis_id=?`, id).Scan(&stored); err != nil {
		return nil, err
	}
	if unit != len(r.Plan.Units) || next != 0 || stored != r.CompletedBatches {
		return nil, ErrAnalysisInvalid
	}
	if err = out.record("human_review_summary", map[string]any{"checked_sections": checked, "total_sections": r.CompletedBatches, "kept_candidates": kept, "excluded_candidates": excluded, "unreviewed_candidates": pending, "all_retained_sections_checked": checked == r.CompletedBatches && pending == 0, "source_coverage_unchanged": true, "independent_verification": false}); err != nil {
		return nil, err
	}
	content, err := out.finish()
	if err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
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
	seen := map[string]bool{}
	for _, ref := range r.Plan.Sources {
		p, _, e := documentPolicy(tx, ref.DocumentID)
		if e != nil {
			return nil, e
		}
		if !seen[p.ID] {
			if e = recordCollectionAudit(tx, owner, p.ID, "synthesis_export_prepared_"+format, id, p.Revision); e != nil {
				return nil, e
			}
			seen[p.ID] = true
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	hash := sha256.Sum256([]byte(content))
	return &AnalysisExport{AnalysisID: id, Format: format, Content: content, SHA256: hex.EncodeToString(hash[:])}, nil
}
