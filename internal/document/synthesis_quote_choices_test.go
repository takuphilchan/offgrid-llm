package document

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSynthesisQuoteGrammarPreservesNativeTextAndNumbers(t *testing.T) {
	text := "Policy 17.25 applies. Exception: clinics may submit by 15 August. 中文例外：8月15日。"
	batch := &SynthesisBatch{ID: "batch", Workflow: "summary", Evidence: []SynthesisEvidence{{ID: "e0", Text: text}}}
	choices := synthesisQuoteChoices(batch)
	found := false
	for _, quote := range choices {
		if !strings.Contains(text, quote) {
			t.Fatal("invented quote", quote)
		}
		if quote == "Policy 17.25 applies." {
			found = true
		}
	}
	if !found || len(choices) < 3 || choices[0] != text {
		t.Fatal("lost precise or full passage", choices)
	}
	raw, err := json.Marshal(synthesisResponseSchema(batch))
	if err != nil || len(raw) > 64<<10 {
		t.Fatal("unbounded schema", err)
	}
	batch.Evidence[0].Text = strings.Repeat("字", 8192)
	if synthesisQuoteChoices(batch) != nil {
		t.Fatal("large source copied into grammar")
	}
}
