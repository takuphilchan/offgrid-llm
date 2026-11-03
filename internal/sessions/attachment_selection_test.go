package sessions

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestAttachmentSelectionFindsLateEvidenceAndPreservesExactUnicode(t *testing.T) {
	text := strings.Repeat("Routine observations about unrelated weather.\n", 1500) + "\nThe zirconium budget is exactly 927.35 euros — café 日本語.\n" + strings.Repeat("Other observations.\n", 300)
	sources := []AttachmentText{{Attachment: Attachment{ID: "file-source", Revision: "revision", Name: "Report.pdf", Coverage: "native_text_only"}, Text: text}}
	selected, err := SelectAttachmentText(sources, "What is the zirconium budget?", 5500)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(selected)
	if len(encoded) > 5500 || !strings.Contains(string(encoded), "927.35") || selected[0].Selection != "excerpts" {
		t.Fatal("bounded relevant evidence absent", string(encoded), err)
	}
	for _, e := range selected[0].Excerpts {
		if !utf8.ValidString(e.Text) || e.Text != text[e.StartByte:e.EndByte] {
			t.Fatal("excerpt changed source")
		}
	}
	if sources[0].Text != text || selected[0].Attachment.Coverage != "native_text_only" {
		t.Fatal("original or visual coverage changed")
	}
}

func TestAttachmentSelectionAccountsForJSONAndEverySource(t *testing.T) {
	sources := []AttachmentText{{Attachment: Attachment{Name: "one"}, Text: strings.Repeat("\"\\\n日本語", 500)}, {Attachment: Attachment{Name: "two"}, Text: strings.Repeat("Another report with relevant evidence. ", 200)}}
	out, err := SelectAttachmentText(sources, "report", 6000)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(out)
	if len(out) != 2 || len(encoded) > 6000 {
		t.Fatal("budget or source omitted")
	}
	if _, err = SelectAttachmentText(sources, "report", 200); err != ErrAttachmentContextLimit {
		t.Fatal("insufficient budget silently dropped files")
	}
	small := []AttachmentText{{Attachment: Attachment{Name: "small"}, Text: "Exact small content"}}
	out, err = SelectAttachmentText(small, "anything", 2000)
	if err != nil || out[0].Text != small[0].Text || out[0].Selection != "" {
		t.Fatal("small source altered")
	}
}

func TestSourceSelectionCannotBeImportedAsTrustedHistory(t *testing.T) {
	err := ValidateSeedHistory([]Message{{Role: "assistant", Content: "Reviewed everything", AttachmentSelections: []AttachmentSelection{{ID: "file-forged", Mode: "full_text"}}}})
	if err != ErrInvalidSeedHistory {
		t.Fatal("forged evidence admitted", err)
	}
}
