package document

import (
	"strings"
	"unicode"
)

// WithTextEnvelope preserves the attachment's existing extraction text, including
// outer whitespace. Native parser graphs may use a trimmed text projection for
// indexing; rebasing spans creates a new revision, never mutates that graph.
func (e *Extraction) WithTextEnvelope(text string) (*Extraction, error) {
	if err := e.Verify(); err != nil {
		return nil, err
	}
	if text == e.Text {
		return e, nil
	}
	if strings.TrimSpace(text) != e.Text {
		return nil, invalidExtraction("text envelope")
	}
	offset := len(text) - len(strings.TrimLeftFunc(text, unicode.IsSpace))
	copy := *e
	copy.Text = text
	copy.Units = append([]Unit(nil), e.Units...)
	copy.Blocks = append([]Block(nil), e.Blocks...)
	for i := range copy.Units {
		copy.Units[i].Span.Start += offset
		copy.Units[i].Span.End += offset
	}
	for i := range copy.Blocks {
		copy.Blocks[i].Span.Start += offset
		copy.Blocks[i].Span.End += offset
	}
	if err := copy.Seal(); err != nil {
		return nil, err
	}
	return &copy, nil
}
