package computer

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func documentFixture() (DocumentInventory, DocumentSnapshot) {
	consent, _ := desktopConsentFixture()
	target := consent.Binding.Target
	target.ID = "window"
	target.Surface = "window"
	observation := ControlObservation{ID: "observation", Target: target, Sequence: 1, CapturedAt: time.Now(), StateDigest: strings.Repeat("a", 64)}
	binding := DocumentBinding{Target: target, Document: "document", Generation: "generation", Container: "tab-1", Editor: "reused-editor", Observation: observation.ID, Revision: "revision"}
	doc := NativeDocument{ID: binding.Document, Generation: binding.Generation, Container: binding.Container, Title: "Untitled", Selected: true, Capabilities: DocumentCapabilities{Read: true, Select: true, SelectText: true, InsertText: true, ReplaceSelection: true, ReplaceValue: true}}
	return DocumentInventory{Version: 1, Observation: observation, Documents: []NativeDocument{doc}, Active: &binding, Complete: true, Commands: []DocumentCreationCommand{{ID: "create", Element: "button", Kind: "new_document", Profile: "owned-editor", Revision: "v1"}}}, DocumentSnapshot{ID: "snapshot", Binding: binding, Text: "", Complete: true, Selection: &DocumentTextRange{}, Composition: "stable"}
}

func TestDocumentInventoryUsesIdentityNotCaptionOrPosition(t *testing.T) {
	v, _ := documentFixture()
	other := v.Documents[0]
	other.ID, other.Container, other.Selected = "second", "tab-2", false
	v.Documents = append(v.Documents, other) // same caption, separate tab
	if err := v.Validate(v.Observation.Target, time.Now()); err != nil {
		t.Fatal(err)
	}
	v.Documents[0], v.Documents[1] = v.Documents[1], v.Documents[0]
	if err := v.Validate(v.Observation.Target, time.Now()); err != nil {
		t.Fatal("reorder changed identity", err)
	}
	v.Documents[0].ID = "document"
	if v.Validate(v.Observation.Target, time.Now()) == nil {
		t.Fatal("duplicate identities admitted")
	}
}

func TestDocumentInventoryRejectsUnknownVersionAndIncompleteBinding(t *testing.T) {
	for _, change := range []func(*DocumentInventory){
		func(v *DocumentInventory) { v.Version = 2 },
		func(v *DocumentInventory) { v.Active.Editor = "" },
		func(v *DocumentInventory) { v.Active.Generation = "different-document" },
		func(v *DocumentInventory) { v.Active.Container = "different-tab" },
		func(v *DocumentInventory) { v.Active = nil },
		func(v *DocumentInventory) { v.Commands[0].Kind = "shortcut" },
		func(v *DocumentInventory) { v.Documents[0].Capabilities.Read = false },
		func(v *DocumentInventory) { v.Documents = append(v.Documents, v.Documents[0]) },
	} {
		v, _ := documentFixture()
		change(&v)
		if v.Validate(v.Observation.Target, time.Now()) == nil {
			t.Fatal("invalid inventory accepted")
		}
	}
	v, _ := documentFixture()
	v.Complete = false
	v.Active = nil
	if err := v.Validate(v.Observation.Target, time.Now()); err != nil {
		t.Fatal("honest partial inventory should be representable", err)
	}
}

func TestCreatedDocumentMustBeDistinctBlankAndPreserveEarlierTabs(t *testing.T) {
	before, _ := documentFixture()
	after, text := documentFixture()
	after.Observation.ID = "after"
	after.Active.Observation = "after"
	after.Active.Document = "new-document"
	after.Active.Container = "new-tab"
	after.Active.Generation = "new-generation"
	doc := after.Documents[0]
	doc.ID, doc.Container, doc.Generation = after.Active.Document, after.Active.Container, after.Active.Generation
	after.Documents[0].Selected = false
	after.Documents = append(after.Documents, doc)
	text.Binding = *after.Active
	if err := ValidateCreatedDocument(before, after, text, time.Now()); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*DocumentInventory, *DocumentSnapshot){
		func(_ *DocumentInventory, s *DocumentSnapshot) { s.Text = "existing content" },
		func(_ *DocumentInventory, s *DocumentSnapshot) { s.Complete = false; s.Selection = nil },
		func(_ *DocumentInventory, s *DocumentSnapshot) { s.Binding.Document = "another-empty-document" },
		func(_ *DocumentInventory, s *DocumentSnapshot) { s.Composition = "unknown" },
		func(v *DocumentInventory, _ *DocumentSnapshot) { v.Complete = false },
		func(v *DocumentInventory, _ *DocumentSnapshot) { v.Documents = v.Documents[1:] },
	} {
		copyAfter, copyText := after, text
		change(&copyAfter, &copyText)
		if ValidateCreatedDocument(before, copyAfter, copyText, time.Now()) == nil {
			t.Fatal("unsafe blankness evidence accepted")
		}
	}
}

func TestDocumentPreconditionRejectsReusedEditorAndChangedRange(t *testing.T) {
	_, before := documentFixture()
	before.Text = "OffGrid test — café 日本語 12345"
	current := before
	current.ID = "fresh-read"
	current.Binding.Observation = "fresh-observation"
	if err := ValidateDocumentPrecondition(before, current); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*DocumentSnapshot){
		func(s *DocumentSnapshot) { s.Binding.Document = "other" },
		func(s *DocumentSnapshot) { s.Binding.Generation = "replaced" },
		func(s *DocumentSnapshot) { s.Binding.Editor = "different-editor" },
		func(s *DocumentSnapshot) { s.Binding.Container = "other-tab" },
		func(s *DocumentSnapshot) { s.Binding.Revision = "typed-then-undid" },
		func(s *DocumentSnapshot) { s.Text = "same editor, other text" },
		func(s *DocumentSnapshot) { s.Selection = &DocumentTextRange{Start: 1, End: 1} },
		func(s *DocumentSnapshot) { s.Composition = "active" },
		func(s *DocumentSnapshot) { s.Composition = "unknown" },
		func(s *DocumentSnapshot) { s.Complete = false; s.Selection = nil },
	} {
		current := before
		change(&current)
		if ValidateDocumentPrecondition(before, current) == nil {
			t.Fatal("stale document edit admitted")
		}
	}
}

func TestDocumentUnicodeRangesPreserveExactEncoding(t *testing.T) {
	for _, text := range []string{"", "OffGrid test — café 日本語 12345", "a😀b", "cafe\u0301", "👩‍👩‍👧‍👧!", "🇿🇼 café", "עברית العربية", "a\r\nb\nc", "क्\u200dष"} {
		for _, unit := range []NativeTextUnit{TextUnitScalar, TextUnitUTF16, TextUnitUTF8} {
			full := DocumentTextRange{End: utf8.RuneCountInString(text)}
			encoded, err := DocumentRangeToNative(text, full, unit)
			if err != nil {
				t.Fatal(text, err)
			}
			decoded, err := DocumentRangeFromNative(text, encoded, unit)
			if err != nil || decoded != full {
				t.Fatal(text, decoded, err)
			}
			for pos := 0; pos <= full.End; pos++ {
				r := DocumentTextRange{Start: pos, End: pos}
				native, err := DocumentRangeToNative(text, r, unit)
				if err != nil {
					continue
				}
				back, err := DocumentRangeFromNative(text, native, unit)
				if err != nil || back != r {
					t.Fatal(text, pos, err)
				}
			}
		}
	}
	for _, test := range []struct {
		text     string
		position int
		unit     NativeTextUnit
	}{
		{"e\u0301", 1, TextUnitScalar}, {"a\r\nb", 2, TextUnitScalar}, {"👩‍👩‍👧‍👧", 1, TextUnitScalar}, {"🇿🇼", 1, TextUnitScalar}, {"😀", 1, TextUnitUTF16}, {"é", 1, TextUnitUTF8},
	} {
		if _, err := DocumentRangeFromNative(test.text, DocumentTextRange{Start: test.position, End: test.position}, test.unit); err == nil {
			t.Fatalf("split encoding/grapheme: %+v", test)
		}
	}
	for _, r := range []DocumentTextRange{{Start: -1, End: 0}, {Start: 2, End: 1}, {End: 999}} {
		if _, err := DocumentRangeToNative("abc", r, TextUnitScalar); err == nil {
			t.Fatal("invalid range admitted")
		}
	}
}

func TestDocumentTextLimitsAreBytesNotTruncation(t *testing.T) {
	if ValidateDocumentInput(strings.Repeat("é", MaxDocumentInputBytes/2)) != nil {
		t.Fatal("valid byte limit rejected")
	}
	for _, text := range []string{strings.Repeat("é", MaxDocumentInputBytes/2+1), "bad\x00text", string([]byte{0xff})} {
		if ValidateDocumentInput(text) == nil {
			t.Fatal("invalid input accepted")
		}
	}
	_, s := documentFixture()
	s.Text = strings.Repeat("x", MaxDocumentSnapshotBytes)
	if s.Validate() != nil {
		t.Fatal("complete snapshot boundary rejected")
	}
	s.Text += "x"
	if s.Validate() == nil {
		t.Fatal("oversized snapshot silently accepted")
	}
	if _, err := DocumentRangeToNative("text", DocumentTextRange{}, NativeTextUnit("unknown")); err == nil {
		t.Fatal("unknown provider units accepted")
	}
	if MaxDocumentPageBytes != 4096 {
		t.Fatal("page bound drift")
	}
}
