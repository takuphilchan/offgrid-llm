package document

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func analysisFixture(t *testing.T, text string) *Extraction {
	t.Helper()
	e := &Extraction{SourceKind: "legacy_text", Format: "txt", Method: "native", Text: text, Units: []Unit{{ID: "unit", Kind: "section", Index: 1, Span: Span{End: len(text)}, State: "ready"}}, Coverage: &Coverage{Status: "complete", TotalUnits: 1, ReadableUnits: 1}, Blocks: []Block{{ID: "block", UnitID: "unit", Kind: "paragraph", Method: "native", Span: Span{End: len(text)}}}}
	if err := e.Seal(); err != nil {
		t.Fatal(err)
	}
	return e
}
func analysisRefs(graphs ...*Extraction) []SourceRef {
	refs := []SourceRef{}
	for i, g := range graphs {
		refs = append(refs, SourceRef{DocumentID: fmt.Sprint("doc-", i), Revision: g.ID, OriginalSHA256: g.OriginalSHA256})
	}
	return refs
}

func TestAnalysisTraversalCannotHideMissingEvidence(t *testing.T) {
	for _, workflow := range []string{"summary", "requirements", "comparison"} {
		good := analysisFixture(t, "Clause 17.25.\nException applies.")
		partial := analysisFixture(t, "Unread figure.")
		partial.Coverage.Status = "partial"
		partial.Units[0].State = "partial"
		partial.Blocks[0].Kind = "figure"
		if err := partial.Seal(); err != nil {
			t.Fatal(err)
		}
		graphs := []*Extraction{good, partial}
		refs := analysisRefs(graphs...)
		plan, err := BuildAnalysisPlan(workflow, refs, graphs)
		if err != nil {
			t.Fatal(err)
		}
		if len(plan.Units) != 2 || len(plan.Gaps) != 2 || plan.Units[1].Blocks != 1 {
			t.Fatal("coverage loss", plan)
		}
		again, err := BuildAnalysisPlan(workflow, refs, graphs)
		if err != nil || again.ID != plan.ID {
			t.Fatal("unstable checkpoint identity", err)
		}
		refs[1].Revision = good.ID
		if _, err = BuildAnalysisPlan(workflow, refs, graphs); err == nil {
			t.Fatal("stale revision admitted")
		}
	}
	g := analysisFixture(t, "policy")
	g.Coverage = nil
	if err := g.Seal(); err != nil {
		t.Fatal(err)
	}
	p, err := BuildAnalysisPlan("summary", analysisRefs(g), []*Extraction{g})
	if err != nil || len(p.Gaps) != 1 || p.Gaps[0].Code != "coverage_unknown" {
		t.Fatal(p, err)
	}
	unassigned := analysisFixture(t, "Policy. Hidden exception.")
	unassigned.Units[0].Span.End = 7
	unassigned.Blocks[0].Span.End = 7
	if err = unassigned.Seal(); err != nil {
		t.Fatal(err)
	}
	p, err = BuildAnalysisPlan("requirements", analysisRefs(unassigned), []*Extraction{unassigned})
	if err != nil || len(p.Gaps) != 1 || p.Gaps[0].Code != "unassigned_text" {
		t.Fatal("text outside declared units hidden", p, err)
	}
}

func TestComparisonExactSpansAndRepeatedLineAccounting(t *testing.T) {
	for _, tc := range []struct{ before, after string }{
		{"政策 17.25\nException: July.\n", "政策 18.25\nException: July.\n"},
		{"Same\nSame\nEnd\n", "Same\nInserted\nSame\nEnd\n"},
		{"A\nB\nC\n", "C\nB\nA\n"}, {"", ""}, {"Old", ""}, {"", "New"},
		{strings.Repeat("Repeat\n", 3000), strings.Repeat("Repeat\n", 2999) + "Different\n"},
	} {
		left, right := analysisFixture(t, tc.before), analysisFixture(t, tc.after)
		pairs, err := PairComparisonUnits(left, right)
		if err != nil {
			t.Fatal(err)
		}
		result, err := CompareUnit(context.Background(), left, right, pairs[0])
		if err != nil {
			t.Fatal(err)
		}
		removed, added := 0, 0
		for _, change := range result.Changes {
			var citation *AnalysisCitation
			if change.Kind == "text_removed" {
				removed++
				citation = change.Before
			} else if change.Kind == "text_added" {
				added++
				citation = change.After
			} else {
				t.Fatal(change)
			}
			if citation == nil || ValidateAnalysisCitation(*citation, analysisRefs(left, right), []*Extraction{left, right}) != nil {
				t.Fatal("unverifiable diff", change)
			}
			forged := *citation
			forged.TextSHA256 = strings.Repeat("0", 64)
			if ValidateAnalysisCitation(forged, analysisRefs(left, right), []*Extraction{left, right}) == nil {
				t.Fatal("forged citation accepted")
			}
		}
		if result.BeforeLines != removed+result.UnchangedLines || result.AfterLines != added+result.UnchangedLines {
			t.Fatal("unaccounted lines", result)
		}
		if tc.before == tc.after && len(result.Changes) != 0 {
			t.Fatal("invented change")
		}
		if left.Verify() != nil || right.Verify() != nil {
			t.Fatal("comparison mutated sources")
		}
	}
}

func TestComparisonDetectsFormulaChangeWithUnchangedDisplay(t *testing.T) {
	left := analysisFixture(t, "17.25")
	left.Format = "xlsx"
	left.Units[0].Kind = "sheet"
	left.Units[0].Name = "Budget"
	left.Blocks[0].Kind = "cell"
	left.Blocks[0].Cell = &Cell{Reference: "B2", Value: "17.25", Formula: "SUM(B3:B4)", Display: "17.25"}
	if err := left.Seal(); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(left)
	var right Extraction
	_ = json.Unmarshal(data, &right)
	right.Blocks[0].Cell.Formula = "17.25"
	if err := right.Seal(); err != nil {
		t.Fatal(err)
	}
	pairs, _ := PairComparisonUnits(left, &right)
	result, err := CompareUnit(context.Background(), left, &right, pairs[0])
	if err != nil || len(result.Changes) != 1 || result.Changes[0].Kind != "cell_modified" || result.UnchangedLines != 1 || result.ComparedCells != 1 {
		t.Fatal(result, err)
	}
	if result.Changes[0].Before.Revision == result.Changes[0].After.Revision {
		t.Fatal("lost two revision identities")
	}
	right.Units[0].Name = "Renamed budget"
	if err = right.Seal(); err != nil {
		t.Fatal(err)
	}
	pairs, _ = PairComparisonUnits(left, &right)
	if len(pairs) != 2 || pairs[0].Alignment != "unmatched" {
		t.Fatal("guessed renamed sheet identity", pairs)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = CompareUnit(ctx, left, &right, pairs[0]); err == nil {
		t.Fatal("ignored cancellation")
	}
}
