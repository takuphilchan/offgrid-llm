package document

import (
	"encoding/json"
	"strings"
	"testing"
)

func synthesisDraft(batch *SynthesisBatch) SynthesisDraft {
	reviewed := []string{}
	for _, e := range batch.Evidence {
		reviewed = append(reviewed, e.ID)
	}
	return SynthesisDraft{BatchID: batch.ID, ReviewedEvidence: reviewed, Items: []SynthesisItem{{Text: "Submit by 30 June; rural offices may submit by 31 July.", Evidence: []SynthesisQuote{{"e0", "Submit by 30 June."}}, Conditions: []SynthesisQuote{}, Exceptions: []SynthesisQuote{{"e1", "Exception: rural offices may submit by 31 July."}}}}}
}

func TestSynthesisCandidatesBindExactExceptionAndNumericEvidence(t *testing.T) {
	traversal, batch := synthesisFixture(t, "requirements")
	draft := synthesisDraft(batch)
	raw, _ := json.Marshal(draft)
	result, err := traversal.CompileCandidates(batch, raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) != 1 || !result.SemanticReviewRequired || result.Candidates[0].Verification != "source_identity_only" || len(result.Candidates[0].Exceptions) != 1 {
		t.Fatal(result)
	}
	for _, q := range append(result.Candidates[0].Evidence, result.Candidates[0].Exceptions...) {
		if err := ValidateAnalysisCitation(q.Citation, traversal.plan.Sources, traversal.graphs); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name, code string
		change     func(*SynthesisDraft)
	}{
		{"invented number", "analysis_numeric_mismatch", func(d *SynthesisDraft) { d.Items[0].Text = "Submit by 29 June." }},
		{"changed quote", "analysis_unsupported_citation", func(d *SynthesisDraft) { d.Items[0].Evidence[0].Quote = "Submit by 31 June." }},
		{"invented identity", "analysis_unsupported_citation", func(d *SynthesisDraft) { d.Items[0].Evidence[0].EvidenceID = "other" }},
		{"dependency only", "analysis_unsupported_citation", func(d *SynthesisDraft) { d.Items[0].Evidence = []SynthesisQuote{{"e1", batch.Evidence[1].Text}} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := synthesisDraft(batch)
			tc.change(&d)
			raw, _ := json.Marshal(d)
			r, err := traversal.CompileCandidates(batch, raw)
			if err != nil || len(r.Candidates) != 0 || len(r.Gaps) != 1 || r.Gaps[0].Code != tc.code {
				t.Fatal(r, err)
			}
		})
	}
}

func TestSynthesisStrictEnvelopeAndNoFindingsAreNotProof(t *testing.T) {
	traversal, batch := synthesisFixture(t, "requirements")
	d := synthesisDraft(batch)
	raw, _ := json.Marshal(d)
	for _, bad := range [][]byte{append(append([]byte{}, raw...), raw...), []byte(strings.Replace(string(raw), "\"batch_id\":", "\"unknown\":1,\"batch_id\":", 1)), []byte(strings.Replace(string(raw), "\"batch_id\":", "\"batch_id\":\"forged\",\"batch_id\":", 1)), []byte("null"), []byte(strings.Repeat(" ", 128<<10) + string(raw))} {
		if _, err := traversal.CompileCandidates(batch, bad); err == nil {
			t.Fatal("invalid envelope accepted")
		}
	}
	d.ReviewedEvidence = []string{"e0", "e0"}
	raw, _ = json.Marshal(d)
	if _, err := traversal.CompileCandidates(batch, raw); err == nil {
		t.Fatal("missing dependency inspection")
	}
	d = synthesisDraft(batch)
	d.Items = []SynthesisItem{}
	d.NoFindings = true
	raw, _ = json.Marshal(d)
	r, err := traversal.CompileCandidates(batch, raw)
	if err != nil || !r.NoFindingsReported || !r.SemanticReviewRequired {
		t.Fatal("model absence claim became verification", r, err)
	}
	d.NoFindings = false
	raw, _ = json.Marshal(d)
	r, err = traversal.CompileCandidates(batch, raw)
	if err != nil || len(r.Gaps) != 1 || r.Gaps[0].Code != "analysis_empty_model_output" {
		t.Fatal(r, err)
	}
}

func TestSynthesisNumberTranscriptionPreservesSignPrecisionAndScript(t *testing.T) {
	for _, tc := range []struct {
		claim, source string
		valid         bool
	}{
		{"17.25", "17.25.", true}, {"17.2", "17.25", false}, {"17.25", "-17.25", false}, {"20%", "20", false}, {"١٧٫٢٥٪", "١٧٫٢٥٪", true}, {"17.25", "١٧٫٢٥", false}, {"1,000", "1000", false}, {"2026/11/13", "2026/11/13", true}, {"31 July", "30 June; 31 July", true},
	} {
		if synthesisNumbersSupported(tc.claim, tc.source) != tc.valid {
			t.Fatal(tc)
		}
	}
}
