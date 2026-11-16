package document

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"
)

func synthesisFixture(t *testing.T, workflow string) (*SynthesisTraversal, *SynthesisBatch) {
	t.Helper()
	text := "Policy\nSubmit by 30 June.\n" + strings.Repeat("政策🙂 padding\n", 200) + "Exception: rural offices may submit by 31 July."
	g := analysisFixture(t, text)
	note := strings.Index(text, "Exception:")
	g.Blocks = []Block{{ID: "heading", UnitID: "unit", Kind: "heading", Method: "native", Span: Span{0, 6}}, {ID: "clause", UnitID: "unit", Kind: "paragraph", Method: "native", Span: Span{7, 25}}, {ID: "note", UnitID: "unit", Kind: "footnote", Method: "native", Span: Span{note, len(text)}}}
	g.Relationships = []Relationship{{From: "heading", To: "clause", Kind: "heading_for"}, {From: "note", To: "clause", Kind: "footnote_for"}}
	if err := g.Seal(); err != nil {
		t.Fatal(err)
	}
	p, err := BuildAnalysisPlan(workflow, analysisRefs(g), []*Extraction{g})
	if err != nil {
		t.Fatal(err)
	}
	traversal, err := PrepareSynthesis(p, []*Extraction{g})
	if err != nil {
		t.Fatal(err)
	}
	batch, err := traversal.Batch(context.Background(), 0, 0, 1024)
	if err != nil {
		t.Fatal(err)
	}
	return traversal, batch
}

func TestSynthesisBatchesPreserveAllBytesAndLinkedException(t *testing.T) {
	traversal, first := synthesisFixture(t, "requirements")
	if len(first.Evidence) != 2 || first.Evidence[1].Role != "dependency" || !strings.Contains(first.Evidence[1].Text, "31 July") {
		t.Fatal("lost exception", first)
	}
	if first.Evidence[0].Citation.Span.End > first.Evidence[1].Citation.Span.Start {
		t.Fatal("test exception must be distant")
	}
	var restored strings.Builder
	start := 0
	steps := 0
	for {
		b, err := traversal.Batch(context.Background(), 0, start, 1024)
		if err != nil {
			t.Fatal(err)
		}
		if !utf8.ValidString(b.Evidence[0].Text) || b.Primary.Start != start || b.Primary.End-b.Primary.Start > 1024 || traversal.VerifyBatch(b) != nil {
			t.Fatal("invalid batch", b)
		}
		again, err := traversal.Batch(context.Background(), 0, start, 1024)
		if err != nil || again.ID != b.ID {
			t.Fatal("unstable batch")
		}
		restored.WriteString(b.Evidence[0].Text)
		steps++
		if b.EndOfUnit {
			break
		}
		if b.Primary.End <= start {
			t.Fatal("no cursor progress")
		}
		start = b.Primary.End
	}
	if steps < 3 || restored.String() != traversal.graphs[0].Text {
		t.Fatal("source bytes omitted/duplicated")
	}
	copy := *first
	copy.Gaps = append(copy.Gaps, AnalysisGap{Code: "invented"})
	if traversal.VerifyBatch(&copy) == nil {
		t.Fatal("forged batch accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := traversal.Batch(ctx, 0, 0, 1024); err == nil {
		t.Fatal("cancel ignored")
	}
	for _, values := range [][3]int{{-1, 0, 1024}, {0, -1, 1024}, {0, 0, 100}, {0, len(traversal.graphs[0].Text), 1024}, {0, 28, 1024}} {
		// Byte 28 is inside the first multi-byte Chinese character.
		if _, err := traversal.Batch(context.Background(), values[0], values[1], values[2]); err == nil {
			t.Fatal("invalid cursor accepted", values)
		}
	}
}

func TestSynthesisDependenciesCannotDisappearOnBudgetLimits(t *testing.T) {
	traversal, _ := synthesisFixture(t, "summary")
	g := traversal.graphs[0]
	g.Text += strings.Repeat("long exception", 3000)
	g.Units[0].Span.End = len(g.Text)
	g.Blocks[2].Span.End = len(g.Text)
	if err := g.Seal(); err != nil {
		t.Fatal(err)
	}
	plan, err := BuildAnalysisPlan("summary", analysisRefs(g), []*Extraction{g})
	if err != nil {
		t.Fatal(err)
	}
	traversal, err = PrepareSynthesis(plan, []*Extraction{g})
	if err != nil {
		t.Fatal(err)
	}
	batch, err := traversal.Batch(context.Background(), 0, 0, 1024)
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Evidence) != 1 || len(batch.Gaps) != 1 || batch.Gaps[0].Code != "analysis_dependency_omitted" {
		t.Fatal(batch)
	}
	raw, _ := json.Marshal(plan)
	var altered AnalysisPlan
	_ = json.Unmarshal(raw, &altered)
	altered.Units[0].Span.End--
	if _, err = PrepareSynthesis(&altered, []*Extraction{g}); err == nil {
		t.Fatal("altered traversal plan accepted")
	}
}

func TestSynthesisNativeCellMetadataAndPartialCellGap(t *testing.T) {
	g := analysisFixture(t, "Amount 17.25\n"+strings.Repeat("x", 2048))
	cache := "17.25"
	g.Blocks = []Block{{ID: "value", UnitID: "unit", Kind: "cell", Method: "native", Span: Span{7, 12}, Cell: &Cell{Reference: "B2", ValueType: "number", Value: "17.25", Formula: "SUM(A1:A2)", CachedValue: &cache}}, {ID: "long", UnitID: "unit", Kind: "cell", Method: "native", Span: Span{13, len(g.Text)}, Cell: &Cell{Reference: "B3", ValueType: "string"}}}
	if err := g.Seal(); err != nil {
		t.Fatal(err)
	}
	p, err := BuildAnalysisPlan("requirements", analysisRefs(g), []*Extraction{g})
	if err != nil {
		t.Fatal(err)
	}
	traversal, err := PrepareSynthesis(p, []*Extraction{g})
	if err != nil {
		t.Fatal(err)
	}
	b, err := traversal.Batch(context.Background(), 0, 0, 1024)
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Evidence) != 2 || b.Evidence[1].Cell == nil || b.Evidence[1].Cell.Formula != "SUM(A1:A2)" || len(b.Gaps) != 1 || b.Gaps[0].Code != "analysis_cell_metadata_omitted" {
		t.Fatal(b)
	}
	b.Evidence[1].Text = "forged"
	if traversal.VerifyBatch(b) == nil {
		t.Fatal("altered source text")
	}
}
