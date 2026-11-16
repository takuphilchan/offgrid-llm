package rag

import (
	"unicode/utf8"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

type AnalysisSourceResult struct {
	AnalysisID string                    `json:"analysis_id"`
	DocumentID string                    `json:"document_id"`
	Citation   document.AnalysisCitation `json:"citation"`
	Text       string                    `json:"text"`
	Truncated  bool                      `json:"truncated"`
	Cell       *document.Cell            `json:"cell,omitempty"`
}

// ReadAnalysisSource resolves only a server-retained change reference. Both
// source policies remain required, even when opening just one side of a diff.
func (s *SQLiteStore) ReadAnalysisSource(owner, id string, step, change int, side string) (*AnalysisSourceResult, error) {
	if side != "before" && side != "after" {
		return nil, ErrAnalysisInvalid
	}
	page, err := s.AnalysisResults(owner, id, step, change, 1)
	if err != nil {
		return nil, err
	}
	if len(page.Result.Changes) != 1 {
		return nil, ErrAnalysisInvalid
	}
	citation := page.Result.Changes[0].Before
	if side == "after" {
		citation = page.Result.Changes[0].After
	}
	if citation == nil {
		return nil, ErrAnalysisUnavailable
	}
	graphs, err := s.analysisGraphs(owner, id)
	if err != nil {
		return nil, err
	}
	record, err := s.Analysis(owner, id)
	if err != nil {
		return nil, err
	}
	if err = document.ValidateAnalysisCitation(*citation, record.Plan.Sources, graphs); err != nil {
		return nil, ErrAnalysisInvalid
	}
	graph := graphs[citation.Source]
	end := min(citation.Span.End, citation.Span.Start+(256<<10))
	for end < citation.Span.End && !utf8.RuneStart(graph.Text[end]) {
		end--
	}
	result := &AnalysisSourceResult{AnalysisID: id, DocumentID: record.Plan.Sources[citation.Source].DocumentID, Citation: *citation, Text: graph.Text[citation.Span.Start:end], Truncated: end < citation.Span.End}
	if citation.BlockID != "" {
		for _, b := range graph.Blocks {
			if b.ID == citation.BlockID {
				result.Cell = b.Cell
				break
			}
		}
	}
	if _, err = s.Analysis(owner, id); err != nil {
		return nil, err
	}
	return result, nil
}
