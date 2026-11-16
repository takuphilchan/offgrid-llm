package rag

import (
	"crypto/sha256"
	"encoding/hex"
	"unicode/utf8"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

// Validate only the shortlisted page, without repeatedly loading whole graphs.
// Caller holds the store read lock and has checked current source policies.
func (s *SQLiteStore) validateAnalysisChanges(plan *document.AnalysisPlan, pair document.ComparisonPair, changes []document.ComparisonChange) ([]AnalysisChangePreview, error) {
	return s.visitAnalysisChanges(plan, pair, changes, nil)
}

func (s *SQLiteStore) visitAnalysisChanges(plan *document.AnalysisPlan, pair document.ComparisonPair, changes []document.ComparisonChange, visit func(int, int, document.AnalysisCitation, string) error) ([]AnalysisChangePreview, error) {
	bytes := 0
	previews := make([]AnalysisChangePreview, len(changes))
	for index, change := range changes {
		switch change.Kind {
		case "text_removed", "cell_removed":
			if change.Before == nil || change.After != nil {
				return nil, ErrAnalysisInvalid
			}
		case "text_added", "cell_added":
			if change.After == nil || change.Before != nil {
				return nil, ErrAnalysisInvalid
			}
		case "cell_modified":
			if change.Before == nil || change.After == nil {
				return nil, ErrAnalysisInvalid
			}
		default:
			return nil, ErrAnalysisInvalid
		}
		for side, c := range []*document.AnalysisCitation{change.Before, change.After} {
			if c == nil {
				continue
			}
			unit := pair.BeforeUnit
			if side == 1 {
				unit = pair.AfterUnit
			}
			if c.Source != side || side >= len(plan.Sources) || c.Revision != plan.Sources[side].Revision || c.UnitID != unit || c.Span.Start < 0 || c.Span.End < c.Span.Start {
				return nil, ErrAnalysisInvalid
			}
			found := false
			for _, u := range plan.Units {
				if u.Source == side && u.UnitID == unit && c.Span.Start >= u.Span.Start && c.Span.End <= u.Span.End {
					found = true
					break
				}
			}
			if !found {
				return nil, ErrAnalysisInvalid
			}
			bytes += c.Span.End - c.Span.Start
			if bytes > 32<<20 {
				return nil, ErrAnalysisLimit
			}
			source := plan.Sources[side]
			if c.BlockID != "" {
				var blockUnit string
				var start, end int
				if err := s.db.QueryRow(`SELECT unit_id,start_byte,end_byte FROM document_blocks WHERE document_id=? AND revision=? AND block_id=?`, source.DocumentID, source.Revision, c.BlockID).Scan(&blockUnit, &start, &end); err != nil || blockUnit != unit || c.Span.Start < start || c.Span.End > end {
					return nil, ErrAnalysisInvalid
				}
			}
			var text string
			if err := s.db.QueryRow(`SELECT CAST(substr(CAST(raw_text AS BLOB),?,?) AS TEXT) FROM document_extractions WHERE document_id=? AND revision=? AND source_sha256=? AND schema_version=1`, c.Span.Start+1, c.Span.End-c.Span.Start, source.DocumentID, source.Revision, source.OriginalSHA256).Scan(&text); err != nil {
				return nil, ErrAnalysisUnavailable
			}
			if len(text) != c.Span.End-c.Span.Start || !utf8.ValidString(text) {
				return nil, ErrAnalysisInvalid
			}
			hash := sha256.Sum256([]byte(text))
			if hex.EncodeToString(hash[:]) != c.TextSHA256 {
				return nil, ErrAnalysisInvalid
			}
			if visit != nil {
				if err := visit(index, side, *c, text); err != nil {
					return nil, err
				}
			}
			if side == 0 {
				previews[index].Before = analysisPreview(text, c.Span.Start)
			} else {
				previews[index].After = analysisPreview(text, c.Span.Start)
			}
		}
	}
	return previews, nil
}
