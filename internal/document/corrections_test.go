package document

import (
	"testing"
	"time"
)

func TestCorrectionPreservesOriginalAndUpdatesLinkedSpans(t *testing.T) {
	e := sampleExtraction()
	if err := e.Seal(); err != nil {
		t.Fatal(err)
	}
	id, text := e.ID, e.Text
	edits := []Correction{{BlockID: "note", ExpectedSHA256: TextSHA256("Exception 17.25"), Text: "例外 18.250"}}
	derived, err := e.Correct("reviewer", time.Now(), edits)
	if err != nil {
		t.Fatal(err)
	}
	if e.ID != id || e.Text != text || e.Verify() != nil {
		t.Fatal("original overwritten")
	}
	if derived.ParentID != id || derived.Author != "reviewer" || derived.Method != "review" || derived.ID == id || derived.Text != "政策\n例外 18.250" {
		t.Fatal("lost correction provenance")
	}
	if derived.Blocks[0].Span.End != len(derived.Text) || derived.Blocks[1].Span.End != len(derived.Text) || derived.Units[0].Span.End != len(derived.Text) {
		t.Fatal("parent offsets stale")
	}
	if derived.Relationships[0] != e.Relationships[0] {
		t.Fatal("clause relationship lost")
	}
	edits[0].ExpectedSHA256 = "wrong"
	if _, err = e.Correct("reviewer", time.Now(), edits); err == nil {
		t.Fatal("stale correction accepted")
	}
	if _, err = e.Correct("reviewer", time.Now(), []Correction{{BlockID: "clause", ExpectedSHA256: TextSHA256(e.Text), Text: "Lost exception"}}); err == nil {
		t.Fatal("parent edit erased linked note")
	}
}
