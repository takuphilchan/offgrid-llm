package document

import (
	"encoding/json"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Constrain short-source quotations to exact native passages. This prevents a
// multilingual model from translating the quote along with its explanation.
// Larger batches retain the exact-quote compiler; grammar growth is bounded.
func synthesisQuoteChoices(batch *SynthesisBatch) []string {
	total := 0
	for _, e := range batch.Evidence {
		total += len(e.Text)
	}
	if total > 4096 {
		return nil
	}
	choices := []string{}
	seen := map[string]bool{}
	add := func(text string) {
		text = strings.TrimSpace(text)
		if text != "" && !seen[text] && len(text) <= 8192 && len(choices) < 96 {
			seen[text] = true
			choices = append(choices, text)
		}
	}
	// Always offer every complete evidence item before finer passages.
	for _, e := range batch.Evidence {
		add(e.Text)
	}
	for _, e := range batch.Evidence {
		start := 0
		for at, r := range e.Text {
			end := at + utf8.RuneLen(r)
			boundary := r == '\n' || strings.ContainsRune("。！？", r)
			if strings.ContainsRune(".!?", r) {
				next, _ := utf8.DecodeRuneInString(e.Text[end:])
				boundary = end == len(e.Text) || unicode.IsSpace(next)
			}
			if boundary {
				add(e.Text[start:end])
				start = end
			}
		}
		add(e.Text[start:])
	}
	raw, err := json.Marshal(choices)
	if err != nil || len(raw) > 12<<10 {
		return nil
	}
	return choices
}
