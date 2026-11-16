package document

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const AnalysisVersion = "document-analysis-v1"

// SourceRef pins an immutable extraction. Document identity is supplied by the
// authorized storage boundary, never inferred from model prose or filenames.
type SourceRef struct {
	DocumentID     string `json:"document_id"`
	Revision       string `json:"revision"`
	OriginalSHA256 string `json:"original_sha256,omitempty"`
}

type AnalysisUnit struct {
	Ordinal int    `json:"ordinal"`
	Source  int    `json:"source"`
	UnitID  string `json:"unit_id"`
	Span    Span   `json:"span"`
	Blocks  int    `json:"blocks"`
}

type AnalysisGap struct {
	Source int    `json:"source"`
	UnitID string `json:"unit_id,omitempty"`
	Code   string `json:"code"`
}

// AnalysisPlan accounts for every retained source unit before work starts.
// Finishing these units means full traversal of retained extraction, not proof
// of complete original-file understanding. Gaps are independent of model output.
type AnalysisPlan struct {
	ID       string         `json:"id"`
	Version  string         `json:"version"`
	Workflow string         `json:"workflow"`
	Sources  []SourceRef    `json:"sources"`
	Units    []AnalysisUnit `json:"units"`
	Gaps     []AnalysisGap  `json:"gaps"`
}

func BuildAnalysisPlan(workflow string, sources []SourceRef, graphs []*Extraction) (*AnalysisPlan, error) {
	if !oneOf(workflow, "summary", "requirements", "comparison") || len(sources) == 0 || len(sources) > 16 || len(graphs) != len(sources) || workflow == "comparison" && len(sources) != 2 {
		return nil, fmt.Errorf("invalid analysis sources or workflow")
	}
	p := &AnalysisPlan{Version: AnalysisVersion, Workflow: workflow, Sources: append([]SourceRef{}, sources...), Units: []AnalysisUnit{}, Gaps: []AnalysisGap{}}
	seen := map[SourceRef]bool{}
	totalBytes, totalBlocks := 0, 0
	for i, graph := range graphs {
		source := sources[i]
		if graph == nil || graph.Verify() != nil || source.DocumentID == "" || len(source.DocumentID) > 256 || strings.ContainsAny(source.DocumentID, "\x00\r\n") || source.Revision != graph.ID || source.OriginalSHA256 != graph.OriginalSHA256 || seen[source] {
			return nil, fmt.Errorf("analysis source identity mismatch")
		}
		seen[source] = true
		spans := append([]Unit{}, graph.Units...)
		sort.Slice(spans, func(a, b int) bool { return spans[a].Span.Start < spans[b].Span.Start })
		covered, unassigned := 0, false
		for _, unit := range spans {
			if unit.Span.Start > covered && strings.TrimSpace(graph.Text[covered:unit.Span.Start]) != "" {
				unassigned = true
			}
			covered = max(covered, unit.Span.End)
		}
		if strings.TrimSpace(graph.Text[covered:]) != "" {
			unassigned = true
		}
		if unassigned {
			p.Gaps = append(p.Gaps, AnalysisGap{Source: i, Code: "unassigned_text"})
		}
		totalBytes += len(graph.Text)
		totalBlocks += len(graph.Blocks)
		if totalBytes > 64<<20 || totalBlocks > 400000 {
			return nil, fmt.Errorf("analysis source limit exceeded")
		}
		if graph.Coverage == nil {
			p.Gaps = append(p.Gaps, AnalysisGap{Source: i, Code: "coverage_unknown"})
		} else if graph.Coverage.Status != "complete" || graph.Coverage.TotalUnits != len(graph.Units) || graph.Coverage.ReadableUnits != len(graph.Units) || len(graph.Coverage.Warnings) != 0 || len(graph.Coverage.ReviewUnits) != 0 {
			p.Gaps = append(p.Gaps, AnalysisGap{Source: i, Code: "extraction_incomplete"})
		}
		counts, review := map[string]int{}, map[string]bool{}
		for _, b := range graph.Blocks {
			counts[b.UnitID]++
			if b.Kind == "figure" || b.Method == "model" || len(b.Warnings) != 0 || b.ReviewState != "" && b.ReviewState != "reviewed" && b.ReviewState != "corrected" {
				review[b.UnitID] = true
			}
		}
		for _, unit := range graph.Units {
			p.Units = append(p.Units, AnalysisUnit{Ordinal: len(p.Units), Source: i, UnitID: unit.ID, Span: unit.Span, Blocks: counts[unit.ID]})
			if !oneOf(unit.State, "ready", "empty") || review[unit.ID] {
				p.Gaps = append(p.Gaps, AnalysisGap{Source: i, UnitID: unit.ID, Code: "unit_requires_review"})
			}
			if counts[unit.ID] == 0 && strings.TrimSpace(graph.Text[unit.Span.Start:unit.Span.End]) != "" {
				p.Gaps = append(p.Gaps, AnalysisGap{Source: i, UnitID: unit.ID, Code: "structure_unavailable"})
			}
		}
	}
	data, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(data)
	p.ID = "analysis-plan-" + hex.EncodeToString(hash[:])
	return p, nil
}

type AnalysisCitation struct {
	Source     int    `json:"source"`
	Revision   string `json:"revision"`
	UnitID     string `json:"unit_id"`
	BlockID    string `json:"block_id,omitempty"`
	Span       Span   `json:"span"`
	TextSHA256 string `json:"text_sha256"`
}

func analysisCitation(source int, graph *Extraction, unit string, block string, span Span) AnalysisCitation {
	hash := sha256.Sum256([]byte(graph.Text[span.Start:span.End]))
	return AnalysisCitation{Source: source, Revision: graph.ID, UnitID: unit, BlockID: block, Span: span, TextSHA256: hex.EncodeToString(hash[:])}
}

// ValidateAnalysisCitation proves source identity/range, never semantic support.
// The caller must independently check current access before exposing the text.
func ValidateAnalysisCitation(c AnalysisCitation, sources []SourceRef, graphs []*Extraction) error {
	if c.Source < 0 || c.Source >= len(sources) || len(sources) != len(graphs) {
		return fmt.Errorf("invalid analysis citation source")
	}
	g := graphs[c.Source]
	if g == nil || g.Verify() != nil || c.Revision != g.ID || sources[c.Source].Revision != g.ID || sources[c.Source].OriginalSHA256 != g.OriginalSHA256 || !validSpan(g.Text, c.Span) {
		return fmt.Errorf("invalid analysis citation identity")
	}
	found := false
	for _, u := range g.Units {
		if u.ID == c.UnitID && c.Span.Start >= u.Span.Start && c.Span.End <= u.Span.End {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("invalid analysis citation unit")
	}
	if c.BlockID != "" {
		found = false
		for _, b := range g.Blocks {
			if b.ID == c.BlockID && b.UnitID == c.UnitID && c.Span.Start >= b.Span.Start && c.Span.End <= b.Span.End {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("invalid analysis citation block")
		}
	}
	expected := analysisCitation(c.Source, g, c.UnitID, c.BlockID, c.Span)
	if expected.TextSHA256 != c.TextSHA256 {
		return fmt.Errorf("invalid analysis citation text")
	}
	return nil
}
