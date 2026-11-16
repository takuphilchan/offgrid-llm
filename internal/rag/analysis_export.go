package rag

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"reflect"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

const AnalysisExportLimit = 8 << 20

type AnalysisExport struct {
	AnalysisID string `json:"analysis_id"`
	Format     string `json:"format"`
	Content    string `json:"content"`
	SHA256     string `json:"sha256"`
}

// ExportComparison prepares a complete bounded artifact before delivery. No
// partial file is returned on corruption, cancellation, access failure or limit.
// The read lock excludes policy changes while evidence is assembled. Access and
// a content-free audit receipt commit together before the artifact is returned.
func (s *SQLiteStore) ExportComparison(ctx context.Context, owner, id, format string) (*AnalysisExport, error) {
	if format != "json" && format != "csv" && format != "markdown" {
		return nil, ErrAnalysisInvalid
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, err := s.analysisLocked(owner, id)
	if err != nil {
		return nil, err
	}
	if r.State != "complete" && r.State != "incomplete" {
		return nil, ErrAnalysisConflict
	}
	if r.Plan.Workflow != "comparison" || len(r.Plan.Sources) != 2 || r.CompletedSteps != len(r.Pairs) || r.CompletedUnits != len(r.Plan.Units) || r.State == "complete" && (r.ReviewRequired || len(r.Plan.Gaps) != 0) {
		return nil, ErrAnalysisInvalid
	}
	// Verify immutable graphs once, including cell metadata, rather than trusting
	// a projection or rehashing the entire document for every exported change.
	graphs := make([]*document.Extraction, 2)
	for i, ref := range r.Plan.Sources {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		graphs[i], err = s.extractionLocked(ref.DocumentID, ref.Revision)
		if err != nil {
			return nil, ErrAnalysisInvalid
		}
	}
	expected, err := document.BuildAnalysisPlan("comparison", r.Plan.Sources, graphs)
	if err != nil || !reflect.DeepEqual(expected, r.Plan) {
		return nil, ErrAnalysisInvalid
	}
	expectedPairs, err := document.PairComparisonUnits(graphs[0], graphs[1])
	if err != nil || !reflect.DeepEqual(expectedPairs, r.Pairs) {
		return nil, ErrAnalysisInvalid
	}
	cells := []map[string]*document.Cell{{}, {}}
	for i, g := range graphs {
		for _, b := range g.Blocks {
			if b.Cell != nil {
				cells[i][b.ID] = b.Cell
			}
		}
	}
	out := newAnalysisExportWriter(format)
	if err = out.record("analysis", map[string]any{"id": r.ID, "workflow": r.Plan.Workflow, "state": r.State, "plan_id": r.Plan.ID, "created_at": r.CreatedAt, "updated_at": r.UpdatedAt, "completed_units": r.CompletedUnits, "total_units": len(r.Plan.Units), "completed_steps": r.CompletedSteps, "review_required": r.ReviewRequired, "error_code": r.ErrorCode, "scope": "retained_text_and_native_cells", "retention": "Exported copies have separate retention; later source deletion or access revocation cannot recall them."}); err != nil {
		return nil, err
	}
	for i, source := range r.Plan.Sources {
		var name string
		if err = s.db.QueryRowContext(ctx, `SELECT name FROM documents WHERE id=?`, source.DocumentID).Scan(&name); err != nil {
			return nil, ErrAnalysisUnavailable
		}
		if err = out.record("source", map[string]any{"source": i, "reference": source, "display_name": name}); err != nil {
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
	units := 0
	seen := map[string]bool{}
	for step, pair := range r.Pairs {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		for side, unit := range []string{pair.BeforeUnit, pair.AfterUnit} {
			if unit == "" {
				continue
			}
			key := string(rune('0'+side)) + ":" + unit
			if seen[key] {
				return nil, ErrAnalysisInvalid
			}
			seen[key] = true
			found := false
			for _, u := range r.Plan.Units {
				if u.Source == side && u.UnitID == unit {
					found = true
					break
				}
			}
			if !found {
				return nil, ErrAnalysisInvalid
			}
			units++
		}
		var data string
		if err = s.db.QueryRowContext(ctx, `SELECT result_json FROM document_analysis_steps WHERE analysis_id=? AND ordinal=? AND storage_bytes<=?`, id, step, 8<<20).Scan(&data); err != nil {
			return nil, ErrAnalysisUnavailable
		}
		var result document.UnitComparison
		if len(data) > 8<<20 || json.Unmarshal([]byte(data), &result) != nil || result.Pair != pair {
			return nil, ErrAnalysisInvalid
		}
		if err = out.record("step", map[string]any{"step": step, "pair": pair, "changes": len(result.Changes), "ambiguous_cells": result.AmbiguousCells, "unchanged_lines": result.UnchangedLines, "before_lines": result.BeforeLines, "after_lines": result.AfterLines, "compared_cells": result.ComparedCells}); err != nil {
			return nil, err
		}
		if result.AmbiguousCells > 0 && (r.State == "complete" || !r.ReviewRequired) {
			return nil, ErrAnalysisInvalid
		}
		// Small batches bound validation scratch space while keeping global ordinals.
		for offset := 0; offset < len(result.Changes); offset += 20 {
			if err = ctx.Err(); err != nil {
				return nil, err
			}
			_, err = s.visitAnalysisChanges(r.Plan, pair, result.Changes[offset:min(offset+20, len(result.Changes))], func(index, side int, c document.AnalysisCitation, text string) error {
				row := map[string]any{"step": step, "change": offset + index, "kind": result.Changes[offset+index].Kind, "side": []string{"before", "after"}[side], "document_id": r.Plan.Sources[side].DocumentID, "citation": c, "text": text}
				if c.BlockID != "" {
					if cell := cells[side][c.BlockID]; cell != nil {
						row["cell"] = cell
					}
				}
				return out.record("change", row)
			})
			if err != nil {
				return nil, err
			}
		}
	}
	if units != r.CompletedUnits {
		return nil, ErrAnalysisInvalid
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
	audited := map[string]bool{}
	for _, ref := range r.Plan.Sources {
		p, _, e := documentPolicy(tx, ref.DocumentID)
		if e != nil {
			return nil, e
		}
		if !audited[p.ID] {
			if e = recordCollectionAudit(tx, owner, p.ID, "analysis_export_prepared_"+format, id, p.Revision); e != nil {
				return nil, e
			}
			audited[p.ID] = true
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	hash := sha256.Sum256([]byte(content))
	return &AnalysisExport{AnalysisID: id, Format: format, Content: content, SHA256: hex.EncodeToString(hash[:])}, nil
}
