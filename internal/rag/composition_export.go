package rag

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
)

// Exported reports retain original quotations and pinned review metadata. The
// bounded writer fails rather than producing a silently truncated artifact.
func (s *SQLiteStore) ExportComposition(ctx context.Context, owner, id, format string) (*AnalysisExport, error) {
	if format != "json" && format != "csv" && format != "markdown" {
		return nil, ErrAnalysisInvalid
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, report, err := s.compositionReportLocked(ctx, owner, id)
	if err != nil {
		return nil, err
	}
	parent, err := s.synthesisLocked(owner, r.AnalysisID)
	if err != nil {
		return nil, err
	}
	seeds, _, _, err := s.compositionSeedsLocked(ctx, owner, parent, r.reviewSnapshot)
	if err != nil {
		return nil, err
	}
	candidateGaps, err := s.compositionCandidateGapsLocked(ctx, parent.ID)
	if err != nil {
		return nil, err
	}
	out := newSynthesisExportWriter(format)
	if err = out.record("composition", map[string]any{"status": r.Status(), "scope": "draft_summary_from_retained_candidates", "semantic_review_required": true, "verification": "source_identity_only", "source_units": report.SourceUnits, "sources": parent.Plan.Sources, "review_snapshot": r.reviewSnapshot, "candidate_gaps": compositionGapCounts(candidateGaps), "notice": "New prose requires human review. Source identity and numeric checks do not establish semantic support or a complete review of original documents.", "retention": "Exported copies have separate retention; later source deletion or access revocation cannot recall them."}); err != nil {
		return nil, err
	}
	for i, section := range report.Sections {
		if err = out.record("section", map[string]any{"section": i, "value": section}); err != nil {
			return nil, err
		}
	}
	for _, gap := range report.Gaps {
		if err = out.record("composition_gap", gap); err != nil {
			return nil, err
		}
	}
	for _, gap := range report.ExtractionGaps {
		if err = out.record("extraction_gap", gap); err != nil {
			return nil, err
		}
	}
	// Include excluded and unrepresented inputs too, so exported review material
	// exposes omissions rather than only the passages that survived compression.
	for _, seed := range seeds {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		if err = out.record("source_candidate", seed); err != nil {
			return nil, err
		}
	}
	content, err := out.finish()
	if err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = analysisAccess(tx, owner, parent.Plan); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, ref := range parent.Plan.Sources {
		p, _, e := documentPolicy(tx, ref.DocumentID)
		if e != nil {
			return nil, e
		}
		if !seen[p.ID] {
			if e = recordCollectionAudit(tx, owner, p.ID, "composition_export_prepared_"+format, id, p.Revision); e != nil {
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
