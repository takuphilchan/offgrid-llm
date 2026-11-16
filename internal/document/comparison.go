package document

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
)

type ComparisonPair struct {
	BeforeUnit string `json:"before_unit,omitempty"`
	AfterUnit  string `json:"after_unit,omitempty"`
	Alignment  string `json:"alignment"`
}

type ComparisonChange struct {
	Kind   string            `json:"kind"`
	Before *AnalysisCitation `json:"before,omitempty"`
	After  *AnalysisCitation `json:"after,omitempty"`
}

type UnitComparison struct {
	Pair           ComparisonPair     `json:"pair"`
	Changes        []ComparisonChange `json:"changes"`
	UnchangedLines int                `json:"unchanged_lines"`
	BeforeLines    int                `json:"before_lines"`
	AfterLines     int                `json:"after_lines"`
	ComparedCells  int                `json:"compared_cells"`
	AmbiguousCells int                `json:"ambiguous_cells"`
}

// PairComparisonUnits uses exact unique sheet names; other units use their
// declared kind/index. This is explicit structural alignment, not a claim that
// two pages with the same number contain the same semantic section.
func PairComparisonUnits(before, after *Extraction) ([]ComparisonPair, error) {
	if before == nil || after == nil || before.Verify() != nil || after.Verify() != nil {
		return nil, fmt.Errorf("invalid comparison revision")
	}
	key := func(u Unit) string {
		if u.Kind == "sheet" && u.Name != "" {
			return "sheet:" + u.Name
		}
		return u.Kind + ":" + strconv.Itoa(u.Index)
	}
	leftCounts, rightCounts := map[string]int{}, map[string]int{}
	right := map[string]Unit{}
	for _, u := range before.Units {
		leftCounts[key(u)]++
	}
	for _, u := range after.Units {
		rightCounts[key(u)]++
		right[key(u)] = u
	}
	used := map[string]bool{}
	pairs := []ComparisonPair{}
	for _, u := range before.Units {
		pair := ComparisonPair{BeforeUnit: u.ID, Alignment: "unmatched"}
		k := key(u)
		if leftCounts[k] == 1 && rightCounts[k] == 1 {
			pair.AfterUnit = right[k].ID
			pair.Alignment = "position"
			if u.Kind == "sheet" && u.Name != "" {
				pair.Alignment = "sheet_name"
			}
			used[pair.AfterUnit] = true
		}
		pairs = append(pairs, pair)
	}
	for _, u := range after.Units {
		if !used[u.ID] {
			pairs = append(pairs, ComparisonPair{AfterUnit: u.ID, Alignment: "unmatched"})
		}
	}
	return pairs, nil
}

type comparisonLine struct {
	text string
	span Span
}

func comparisonLines(graph *Extraction, id string) ([]comparisonLine, error) {
	if id == "" {
		return nil, nil
	}
	var unit *Unit
	for i := range graph.Units {
		if graph.Units[i].ID == id {
			unit = &graph.Units[i]
			break
		}
	}
	if unit == nil {
		return nil, fmt.Errorf("comparison unit unavailable")
	}
	lines := []comparisonLine{}
	start := unit.Span.Start
	for i := start; i < unit.Span.End; i++ {
		if graph.Text[i] == '\n' {
			lines = append(lines, comparisonLine{graph.Text[start : i+1], Span{start, i + 1}})
			start = i + 1
		}
		if len(lines) > 200000 {
			return nil, fmt.Errorf("comparison line limit exceeded")
		}
	}
	if start < unit.Span.End {
		lines = append(lines, comparisonLine{graph.Text[start:unit.Span.End], Span{start, unit.Span.End}})
	}
	return lines, nil
}

// Unique exact lines establish monotonic anchors in O(n log n), avoiding a
// quadratic diff matrix on large documents. Remaining spans are honest removals
// and additions; approximate or repeated text is never asserted to be identical.
func comparisonAnchors(left, right []comparisonLine) [][2]int {
	lc, rc, ri := map[string]int{}, map[string]int{}, map[string]int{}
	for _, l := range left {
		lc[l.text]++
	}
	for i, l := range right {
		rc[l.text]++
		ri[l.text] = i
	}
	candidates := [][2]int{}
	for i, l := range left {
		if lc[l.text] == 1 && rc[l.text] == 1 {
			candidates = append(candidates, [2]int{i, ri[l.text]})
		}
	}
	tails, previous := []int{}, make([]int, len(candidates))
	for i, pair := range candidates {
		at := sort.Search(len(tails), func(j int) bool { return candidates[tails[j]][1] >= pair[1] })
		previous[i] = -1
		if at > 0 {
			previous[i] = tails[at-1]
		}
		if at == len(tails) {
			tails = append(tails, i)
		} else {
			tails[at] = i
		}
	}
	anchors := make([][2]int, len(tails))
	if len(tails) > 0 {
		for i, j := tails[len(tails)-1], len(tails)-1; j >= 0; j-- {
			anchors[j] = candidates[i]
			i = previous[i]
		}
	}
	return anchors
}

// CompareUnit records exact text and native cell-metadata differences. It does
// not run a model, execute formulas, infer legal significance or verify visuals.
// Callers verify source access before computing and before publishing each unit.
func compareUnit(ctx context.Context, before, after *Extraction, pair ComparisonPair) (*UnitComparison, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	left, err := comparisonLines(before, pair.BeforeUnit)
	if err != nil {
		return nil, err
	}
	right, err := comparisonLines(after, pair.AfterUnit)
	if err != nil {
		return nil, err
	}
	result := &UnitComparison{Pair: pair, Changes: []ComparisonChange{}, BeforeLines: len(left), AfterLines: len(right)}
	appendLine := func(kind string, source int, g *Extraction, unit string, line comparisonLine) {
		c := analysisCitation(source, g, unit, "", line.span)
		change := ComparisonChange{Kind: kind}
		if source == 0 {
			change.Before = &c
		} else {
			change.After = &c
		}
		result.Changes = append(result.Changes, change)
	}
	anchors := append(comparisonAnchors(left, right), [2]int{len(left), len(right)})
	li, ri := 0, 0
	for _, anchor := range anchors {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		// Equal prefixes/suffixes also retain repeated lines around unique anchors.
		for li < anchor[0] && ri < anchor[1] && left[li].text == right[ri].text {
			result.UnchangedLines++
			li++
			ri++
		}
		le, re := anchor[0], anchor[1]
		for le > li && re > ri && left[le-1].text == right[re-1].text {
			result.UnchangedLines++
			le--
			re--
		}
		for ; li < le; li++ {
			appendLine("text_removed", 0, before, pair.BeforeUnit, left[li])
		}
		for ; ri < re; ri++ {
			appendLine("text_added", 1, after, pair.AfterUnit, right[ri])
		}
		li, ri = anchor[0]+1, anchor[1]+1
		if anchor[0] < len(left) {
			result.UnchangedLines++
		}
	}
	if err = compareCells(ctx, before, after, pair, result); err != nil {
		return nil, err
	}
	return result, nil
}

func compareCells(ctx context.Context, before, after *Extraction, pair ComparisonPair, result *UnitComparison) error {
	// Only named-sheet alignment confers cell-address correspondence. Office
	// tables without stable addresses remain represented in the exact text diff.
	if pair.Alignment != "sheet_name" {
		return nil
	}
	left, right := map[string]Block{}, map[string]Block{}
	counts := map[string]int{}
	for side, g := range []*Extraction{before, after} {
		unit := pair.BeforeUnit
		if side == 1 {
			unit = pair.AfterUnit
		}
		for _, b := range g.Blocks {
			if b.UnitID != unit || b.Cell == nil || b.Cell.Reference == "" {
				continue
			}
			key := strconv.Itoa(side) + ":" + b.Cell.Reference
			counts[key]++
			if side == 0 {
				left[b.Cell.Reference] = b
			} else {
				right[b.Cell.Reference] = b
			}
		}
	}
	keys := []string{}
	for ref := range left {
		keys = append(keys, ref)
	}
	for ref := range right {
		if _, ok := left[ref]; !ok {
			keys = append(keys, ref)
		}
	}
	sort.Strings(keys)
	for _, ref := range keys {
		if err := ctx.Err(); err != nil {
			return err
		}
		if counts["0:"+ref] > 1 || counts["1:"+ref] > 1 {
			result.AmbiguousCells++
			continue
		}
		l, lok := left[ref]
		r, rok := right[ref]
		result.ComparedCells++
		lb, _ := json.Marshal(l.Cell)
		rb, _ := json.Marshal(r.Cell)
		if lok && rok && string(lb) == string(rb) {
			continue
		}
		change := ComparisonChange{Kind: "cell_modified"}
		if lok {
			c := analysisCitation(0, before, pair.BeforeUnit, l.ID, l.Span)
			change.Before = &c
		} else {
			change.Kind = "cell_added"
		}
		if rok {
			c := analysisCitation(1, after, pair.AfterUnit, r.ID, r.Span)
			change.After = &c
		} else {
			change.Kind = "cell_removed"
		}
		result.Changes = append(result.Changes, change)
	}
	return nil
}
