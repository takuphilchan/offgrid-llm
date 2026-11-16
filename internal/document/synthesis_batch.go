package document

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"
)

const SynthesisProtocol = "document-synthesis-candidates-v1"

type SynthesisEvidence struct {
	ID       string           `json:"id"`
	Role     string           `json:"role"`
	Citation AnalysisCitation `json:"citation"`
	Text     string           `json:"text"`
	Cell     *Cell            `json:"cell,omitempty"`
}

// A batch accounts for a contiguous portion of one source unit. Dependencies
// are separately citable evidence, never text prepended under the primary span.
type SynthesisBatch struct {
	ID           string              `json:"id"`
	PlanID       string              `json:"plan_id"`
	Protocol     string              `json:"protocol"`
	Workflow     string              `json:"workflow"`
	UnitOrdinal  int                 `json:"unit_ordinal"`
	Primary      Span                `json:"primary"`
	PrimaryLimit int                 `json:"primary_limit_bytes"`
	EndOfUnit    bool                `json:"end_of_unit"`
	Continued    bool                `json:"continued"`
	Evidence     []SynthesisEvidence `json:"evidence"`
	Gaps         []AnalysisGap       `json:"gaps"`
}

type SynthesisTraversal struct {
	plan   *AnalysisPlan
	graphs []*Extraction
	links  []*LinkIndex
}

// PrepareSynthesis verifies immutable sources once. The storage boundary must
// independently enforce current ACLs at preparation, generation and publication.
// This object is request/worker-local and is not an authorization cache.
func PrepareSynthesis(plan *AnalysisPlan, graphs []*Extraction) (*SynthesisTraversal, error) {
	if plan == nil || !oneOf(plan.Workflow, "summary", "requirements") {
		return nil, errors.New("invalid synthesis workflow")
	}
	expected, err := BuildAnalysisPlan(plan.Workflow, plan.Sources, graphs)
	if err != nil || !reflect.DeepEqual(expected, plan) {
		return nil, errors.New("synthesis source plan mismatch")
	}
	traversal := &SynthesisTraversal{plan: plan, graphs: graphs}
	for _, g := range graphs {
		traversal.links = append(traversal.links, IndexLinks(g))
	}
	return traversal, nil
}

// Batch uses byte bounds solely to bound preparation, not as a tokenizer
// estimate. The model runner must count the final prompt under its generation
// lease and reject an over-budget request without silently dropping evidence.
func (t *SynthesisTraversal) Batch(ctx context.Context, unitOrdinal, start, primaryBytes int) (*SynthesisBatch, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if unitOrdinal < 0 || unitOrdinal >= len(t.plan.Units) || primaryBytes < 1024 || primaryBytes > 8192 {
		return nil, errors.New("invalid synthesis batch bounds")
	}
	unit := t.plan.Units[unitOrdinal]
	g := t.graphs[unit.Source]
	if start < unit.Span.Start || start > unit.Span.End || start == unit.Span.End && unit.Span.Start != unit.Span.End || !validSpan(g.Text, Span{start, start}) {
		return nil, errors.New("invalid synthesis cursor")
	}
	end := min(unit.Span.End, start+primaryBytes)
	for end < len(g.Text) && !utf8.RuneStart(g.Text[end]) {
		end--
	}
	if end < unit.Span.End {
		if boundary := strings.LastIndexByte(g.Text[start:end], '\n'); boundary >= primaryBytes/2 {
			end = start + boundary + 1
		}
	}
	span := Span{start, end}
	b := &SynthesisBatch{PlanID: t.plan.ID, Protocol: SynthesisProtocol, Workflow: t.plan.Workflow, UnitOrdinal: unitOrdinal, Primary: span, PrimaryLimit: primaryBytes, EndOfUnit: end == unit.Span.End, Continued: start != unit.Span.Start, Evidence: []SynthesisEvidence{}, Gaps: []AnalysisGap{}}
	for _, gap := range t.plan.Gaps {
		if gap.Source == unit.Source && (gap.UnitID == "" || gap.UnitID == unit.UnitID) {
			b.Gaps = append(b.Gaps, gap)
		}
	}
	add := func(role, block, unitID string, span Span, cell *Cell) {
		b.Evidence = append(b.Evidence, SynthesisEvidence{ID: "e" + strconv.Itoa(len(b.Evidence)), Role: role, Citation: analysisCitation(unit.Source, g, unitID, block, span), Text: g.Text[span.Start:span.End], Cell: synthesisCellCopy(cell)})
	}
	add("primary", "", unit.UnitID, span, nil)
	linked := t.links[unit.Source].ForSpan(span)
	if linked.LimitReached {
		b.Gaps = append(b.Gaps, AnalysisGap{Source: unit.Source, UnitID: unit.UnitID, Code: "analysis_link_limit"})
	}
	used := end - start
	for _, block := range linked.Required {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if block.Span.Start >= start && block.Span.End <= end {
			continue
		}
		raw, _ := json.Marshal(block.Cell)
		size := block.Span.End - block.Span.Start + len(raw)
		if len(b.Evidence) >= 32 || used+size > 24<<10 {
			b.Gaps = append(b.Gaps, AnalysisGap{Source: unit.Source, UnitID: unit.UnitID, Code: "analysis_dependency_omitted"})
			continue
		}
		add("dependency", block.ID, block.UnitID, block.Span, block.Cell)
		used += size
	}
	// Native cell metadata must remain distinct from visible text. A cell is
	// offered only with its full span; partial cells are accounted by later batches.
	for _, id := range linked.BlockIDs {
		block := t.links[unit.Source].blocks[id]
		if block.Cell == nil {
			continue
		}
		if block.Span.Start < start || block.Span.End > end {
			b.Gaps = append(b.Gaps, AnalysisGap{Source: unit.Source, UnitID: unit.UnitID, Code: "analysis_cell_metadata_omitted"})
			continue
		}
		raw, _ := json.Marshal(block.Cell)
		size := block.Span.End - block.Span.Start + len(raw)
		if len(b.Evidence) >= 32 || used+size > 24<<10 {
			b.Gaps = append(b.Gaps, AnalysisGap{Source: unit.Source, UnitID: unit.UnitID, Code: "analysis_cell_metadata_omitted"})
			continue
		}
		add("cell", block.ID, block.UnitID, block.Span, block.Cell)
		used += size
	}
	raw, err := json.Marshal(b)
	if err != nil || len(raw) > 128<<10 {
		return nil, errors.New("synthesis batch envelope limit")
	}
	hash := sha256.Sum256(raw)
	b.ID = "synthesis-batch-" + hex.EncodeToString(hash[:])
	return b, nil
}

func synthesisCellCopy(cell *Cell) *Cell {
	if cell == nil {
		return nil
	}
	copy := *cell
	copy.HiddenReasons = append([]string(nil), cell.HiddenReasons...)
	if cell.CachedValue != nil {
		value := *cell.CachedValue
		copy.CachedValue = &value
	}
	if cell.SharedFormulaIndex != nil {
		value := *cell.SharedFormulaIndex
		copy.SharedFormulaIndex = &value
	}
	return &copy
}

// VerifyBatch reconstructs the deterministic batch. A recomputed hash on altered
// source ranges, metadata or omitted dependency gaps cannot validate a forgery.
func (t *SynthesisTraversal) VerifyBatch(b *SynthesisBatch) error {
	if b == nil {
		return errors.New("invalid synthesis batch")
	}
	expected, err := t.Batch(context.Background(), b.UnitOrdinal, b.Primary.Start, b.PrimaryLimit)
	if err != nil || !reflect.DeepEqual(expected, b) {
		return errors.New("synthesis batch changed")
	}
	return nil
}
