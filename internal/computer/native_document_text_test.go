package computer

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func textOperationFixture(kind, text string, selection DocumentTextRange) (DocumentInventory, DocumentSnapshot, DocumentTextOperation) {
	view, snapshot := documentFixture()
	snapshot.Text, snapshot.Selection = "α café 日本語 😀 tail", &selection
	op := DocumentTextOperation{Kind: kind, Snapshot: snapshot.ID, Binding: snapshot.Binding, Text: &text}
	return view, snapshot, op
}

func TestDocumentTextModesPreserveExactSurroundingText(t *testing.T) {
	for _, test := range []struct {
		kind, text, expected string
		selection            DocumentTextRange
	}{
		{"insert_text", "—", "α —café 日本語 😀 tail", DocumentTextRange{Start: 2, End: 2}},
		{"replace_selection", "日本語", "α 日本語 日本語 😀 tail", DocumentTextRange{Start: 2, End: 6}},
		{"replace_selection", "", "α  日本語 😀 tail", DocumentTextRange{Start: 2, End: 6}},
		{"replace_value", "OffGrid test — café 日本語 12345", "OffGrid test — café 日本語 12345", DocumentTextRange{Start: 2, End: 6}},
		{"replace_value", "", "", DocumentTextRange{Start: 2, End: 6}},
	} {
		t.Run(test.kind+test.text, func(t *testing.T) {
			view, before, operation := textOperationFixture(test.kind, test.text, test.selection)
			prepared, err := PrepareDocumentText(view, before, operation, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			if prepared.ExpectedText() != test.expected || prepared.ID() == "" || len(prepared.Digest()) != 64 {
				t.Fatal("wrong host-derived result")
			}
			if err := prepared.Revalidate(view, before, time.Now()); err != nil {
				t.Fatal(err)
			}
			// The host plan is not a wire response containing the private preimage.
			encoded, err := json.Marshal(prepared)
			if err != nil || string(encoded) != "{}" {
				t.Fatal("private preparation became a wire structure")
			}
		})
	}
}

func TestDocumentTextSelectionIsNotReplacement(t *testing.T) {
	view, before, op := textOperationFixture("select_text", "ignored", DocumentTextRange{})
	op.Text = nil
	op.Range = &DocumentTextRange{Start: 7, End: 10}
	prepared, err := PrepareDocumentText(view, before, op, time.Now())
	if err != nil || prepared.ExpectedText() != before.Text || prepared.ExpectedSelection() != *op.Range {
		t.Fatal(prepared, err)
	}
	before.Selection.Start = 1
	op.Range.Start = 0
	exposed := prepared.Operation()
	exposed.Range.Start = 1
	exposedPreimage := prepared.Preimage()
	exposedPreimage.Selection.Start = 2
	if prepared.Operation().Range.Start != 7 || prepared.Preimage().Selection.Start != 0 {
		t.Fatal("caller mutated frozen preparation")
	}
	view, before, op = textOperationFixture("insert_text", "new", DocumentTextRange{})
	prepared, err = PrepareDocumentText(view, before, op, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	*op.Text = "different"
	exposed = prepared.Operation()
	*exposed.Text = "other"
	if *prepared.Operation().Text != "new" {
		t.Fatal("text pointer escaped")
	}
}

func TestDocumentTextRejectsModeConfusionAndInvalidBindings(t *testing.T) {
	for _, change := range []func(*DocumentInventory, *DocumentSnapshot, *DocumentTextOperation){
		func(_ *DocumentInventory, _ *DocumentSnapshot, op *DocumentTextOperation) { op.Kind = "shortcut" },
		func(_ *DocumentInventory, _ *DocumentSnapshot, op *DocumentTextOperation) { op.Snapshot = "different" },
		func(_ *DocumentInventory, _ *DocumentSnapshot, op *DocumentTextOperation) {
			op.Binding.Generation = "different"
		},
		func(_ *DocumentInventory, _ *DocumentSnapshot, op *DocumentTextOperation) {
			op.Range = &DocumentTextRange{}
		},
		func(_ *DocumentInventory, _ *DocumentSnapshot, op *DocumentTextOperation) { op.Text = nil },
		func(_ *DocumentInventory, _ *DocumentSnapshot, op *DocumentTextOperation) { *op.Text = "invalid\x00" },
		func(_ *DocumentInventory, _ *DocumentSnapshot, op *DocumentTextOperation) {
			*op.Text = string([]byte{0xff})
		},
		func(_ *DocumentInventory, _ *DocumentSnapshot, op *DocumentTextOperation) {
			*op.Text = strings.Repeat("a", MaxDocumentInputBytes+1)
		},
		func(_ *DocumentInventory, s *DocumentSnapshot, _ *DocumentTextOperation) { s.Selection = nil },
		func(_ *DocumentInventory, s *DocumentSnapshot, _ *DocumentTextOperation) { s.Selection.End = 2 },
		func(_ *DocumentInventory, s *DocumentSnapshot, _ *DocumentTextOperation) { s.Composition = "active" },
		func(_ *DocumentInventory, s *DocumentSnapshot, _ *DocumentTextOperation) { s.Composition = "unknown" },
		func(_ *DocumentInventory, s *DocumentSnapshot, _ *DocumentTextOperation) {
			s.Complete = false
			s.Selection = nil
		},
		func(v *DocumentInventory, _ *DocumentSnapshot, _ *DocumentTextOperation) { v.Complete = false },
		func(v *DocumentInventory, _ *DocumentSnapshot, _ *DocumentTextOperation) {
			v.Documents[0].Capabilities.InsertText = false
		},
		func(v *DocumentInventory, _ *DocumentSnapshot, _ *DocumentTextOperation) {
			v.Observation.CapturedAt = time.Now().Add(-time.Hour)
		},
	} {
		view, snapshot, op := textOperationFixture("insert_text", "new", DocumentTextRange{})
		change(&view, &snapshot, &op)
		if _, err := PrepareDocumentText(view, snapshot, op, time.Now()); err == nil {
			t.Fatal("invalid preparation accepted")
		}
	}
	view, snapshot, op := textOperationFixture("replace_selection", "new", DocumentTextRange{})
	if _, err := PrepareDocumentText(view, snapshot, op, time.Now()); err == nil {
		t.Fatal("collapsed selection treated as replacement")
	}
	view, snapshot, op = textOperationFixture("insert_text", "x", DocumentTextRange{})
	snapshot.Text = strings.Repeat("x", MaxDocumentSnapshotBytes)
	if _, err := PrepareDocumentText(view, snapshot, op, time.Now()); err != ErrDocumentIncomplete {
		t.Fatal("oversized expected result", err)
	}
}

func TestDocumentPreparationRequiresUnchangedTextSelectionAndIdentity(t *testing.T) {
	for _, change := range []func(*DocumentInventory, *DocumentSnapshot){
		func(_ *DocumentInventory, s *DocumentSnapshot) { s.Text += "!" },
		func(_ *DocumentInventory, s *DocumentSnapshot) { s.Binding.Revision = "new" },
		func(_ *DocumentInventory, s *DocumentSnapshot) { s.Binding.Document = "other" },
		func(_ *DocumentInventory, s *DocumentSnapshot) { s.Binding.Generation = "other" },
		func(_ *DocumentInventory, s *DocumentSnapshot) { s.Binding.Container = "other" },
		func(_ *DocumentInventory, s *DocumentSnapshot) { s.Binding.Editor = "other" },
		func(_ *DocumentInventory, s *DocumentSnapshot) { s.Binding.Target.ProcessGeneration = "other" },
		func(_ *DocumentInventory, s *DocumentSnapshot) { s.Selection = &DocumentTextRange{Start: 1, End: 1} },
		func(_ *DocumentInventory, s *DocumentSnapshot) { s.Composition = "active" },
		func(v *DocumentInventory, _ *DocumentSnapshot) { v.Documents[0].Capabilities.InsertText = false },
	} {
		view, before, op := textOperationFixture("insert_text", "new", DocumentTextRange{})
		prepared, err := PrepareDocumentText(view, before, op, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		current := copyDocumentSnapshot(before)
		change(&view, &current)
		if prepared.Revalidate(view, current, time.Now()) == nil {
			t.Fatal("changed preparation accepted")
		}
	}
	view, before, op := textOperationFixture("insert_text", "new", DocumentTextRange{})
	prepared, err := PrepareDocumentText(view, before, op, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	view.Observation.ID = "fresh"
	view.Active.Observation = "fresh"
	before.Binding.Observation = "fresh"
	before.ID = "fresh-read"
	if err := prepared.Revalidate(view, before, time.Now()); err != nil {
		t.Fatal("fresh identical read rejected", err)
	}
}

func TestDocumentPagerBoundedUnicodeAndStableRevision(t *testing.T) {
	_, snapshot := documentFixture()
	snapshot.Text = strings.Repeat("😀 e\u0301 日本語 مرحبا\r\n", 600)
	pager, err := NewDocumentPager(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	var collected strings.Builder
	start, pages := 0, 0
	for {
		page, err := pager.Page(snapshot.ID, snapshot.Binding.Revision, start, snapshot)
		if err != nil {
			t.Fatal(err)
		}
		if !page.SourceComplete || page.Start != start || len(page.Text) > MaxDocumentPageBytes || !utf8.ValidString(page.Text) || page.End-page.Start != utf8.RuneCountInString(page.Text) {
			t.Fatal("invalid page")
		}
		collected.WriteString(page.Text)
		pages++
		if page.Next == nil {
			break
		}
		if *page.Next != page.End || page.End <= start {
			t.Fatal("non-advancing page")
		}
		start = *page.Next
		if pages > 100 {
			t.Fatal("page loop")
		}
	}
	if collected.String() != snapshot.Text || pages < 2 {
		t.Fatal("pagination changed exact content")
	}
	for _, change := range []func(*DocumentSnapshot){
		func(s *DocumentSnapshot) { s.Text += "x" },
		func(s *DocumentSnapshot) { s.Binding.Revision = "new" },
		func(s *DocumentSnapshot) { s.Binding.Document = "other" },
		func(s *DocumentSnapshot) { s.Binding.Generation = "other" },
		func(s *DocumentSnapshot) { s.Binding.Editor = "other" },
		func(s *DocumentSnapshot) { s.Complete = false; s.Selection = nil },
	} {
		current := copyDocumentSnapshot(snapshot)
		change(&current)
		if _, err := pager.Page(snapshot.ID, snapshot.Binding.Revision, 0, current); err == nil {
			t.Fatal("stale page accepted")
		}
	}
	if _, err := pager.Page("other", snapshot.Binding.Revision, 0, snapshot); err == nil {
		t.Fatal("wrong snapshot")
	}
	if _, err := pager.Page(snapshot.ID, "other", 0, snapshot); err == nil {
		t.Fatal("wrong revision")
	}
	if _, err := pager.Page(snapshot.ID, snapshot.Binding.Revision, -1, snapshot); err == nil {
		t.Fatal("negative offset")
	}
}

func TestDocumentPagerNeverSplitsOversizedGrapheme(t *testing.T) {
	_, snapshot := documentFixture()
	snapshot.Text = "e" + strings.Repeat("\u0301", MaxDocumentPageBytes)
	pager, err := NewDocumentPager(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pager.Page(snapshot.ID, snapshot.Binding.Revision, 0, snapshot); err != ErrDocumentRange {
		t.Fatal("oversized cluster silently split", err)
	}
	if _, err = pager.Page(snapshot.ID, snapshot.Binding.Revision, 1, snapshot); err != ErrDocumentRange {
		t.Fatal("inside-cluster offset accepted", err)
	}
	snapshot.Text = ""
	pager, err = NewDocumentPager(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	page, err := pager.Page(snapshot.ID, snapshot.Binding.Revision, 0, snapshot)
	if err != nil || page.Text != "" || page.Next != nil || page.End != 0 {
		t.Fatal("empty document", page, err)
	}
}
