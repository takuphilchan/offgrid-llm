package document

import "testing"

func TestTextEnvelopePreservesUnicodeOffsetsAndOriginalRevision(t *testing.T) {
	e := sampleExtraction()
	if err := e.Seal(); err != nil {
		t.Fatal(err)
	}
	original, span := e.ID, e.Blocks[0].Span
	prefix := "\u3000\n\t"
	derived, err := e.WithTextEnvelope(prefix + e.Text + "\r\n")
	if err != nil {
		t.Fatal(err)
	}
	if derived.ID == original || derived.Verify() != nil || e.ID != original || e.Blocks[0].Span != span || e.Verify() != nil {
		t.Fatal("revision or original changed")
	}
	for i, b := range derived.Blocks {
		before := e.Blocks[i].Span
		if derived.Text[b.Span.Start:b.Span.End] != e.Text[before.Start:before.End] {
			t.Fatal("source span changed")
		}
	}
	if _, err := e.WithTextEnvelope("changed " + e.Text); err == nil {
		t.Fatal("non-whitespace replacement admitted")
	}
}
