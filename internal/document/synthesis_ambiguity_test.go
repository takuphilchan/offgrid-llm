package document

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestSynthesisRejectsRepeatedQuotesAndCaseAliasedFields(t *testing.T) {
	g := analysisFixture(t, "Deadline 30 June. Deadline 30 June.")
	p, err := BuildAnalysisPlan("summary", analysisRefs(g), []*Extraction{g})
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
	draft := SynthesisDraft{BatchID: batch.ID, ReviewedEvidence: []string{"e0"}, Items: []SynthesisItem{{Text: "The deadline is 30 June.", Evidence: []SynthesisQuote{{"e0", "Deadline 30 June."}}, Conditions: []SynthesisQuote{}, Exceptions: []SynthesisQuote{}}}}
	raw, _ := json.Marshal(draft)
	result, err := traversal.CompileCandidates(batch, raw)
	if err != nil || len(result.Candidates) != 0 || len(result.Gaps) != 1 || result.Gaps[0].Code != "analysis_unsupported_citation" {
		t.Fatal("ambiguous citation", result, err)
	}
	aliased := strings.Replace(string(raw), "\"batch_id\":", "\"Batch_ID\":\"other\",\"batch_id\":", 1)
	if _, err = traversal.CompileCandidates(batch, []byte(aliased)); err == nil {
		t.Fatal("case-alias shadowing accepted")
	}
	draft.Items[0].Evidence[0].Quote = g.Text
	raw, _ = json.Marshal(draft)
	result, err = traversal.CompileCandidates(batch, raw)
	if err != nil || len(result.Candidates) != 1 || !result.SemanticReviewRequired {
		t.Fatal("full exact quote should be uniquely citable, but still require semantic review", result, err)
	}
}

func TestSynthesisBatchDoesNotExposeMutableNativeCells(t *testing.T) {
	g := analysisFixture(t, "17.25")
	cached := "17.25"
	g.Blocks[0].Kind = "cell"
	g.Blocks[0].Cell = &Cell{Reference: "B2", Value: "17.25", CachedValue: &cached, HiddenReasons: []string{"row"}}
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
	b.Evidence[1].Cell.Value = "tampered"
	*b.Evidence[1].Cell.CachedValue = "tampered"
	b.Evidence[1].Cell.HiddenReasons[0] = "tampered"
	if g.Verify() != nil || g.Blocks[0].Cell.Value != "17.25" || *g.Blocks[0].Cell.CachedValue != "17.25" || g.Blocks[0].Cell.HiddenReasons[0] != "row" {
		t.Fatal("batch mutated immutable graph")
	}
	if traversal.VerifyBatch(b) == nil {
		t.Fatal("tampered cell batch accepted")
	}
}
