package sessions

import (
	"errors"
	"github.com/takuphilchan/offgrid-llm/internal/document"
	"strings"
	"testing"
)

func TestSelectAttachmentPagePinsOriginalUTF8Offsets(t *testing.T) {
	prefix, selected := "Unrelated first page\n", "政策例外: 17.25.\n"
	text := prefix + selected + "Unrelated last page"
	source := AttachmentText{Text: text, Structure: &document.Extraction{Units: []document.Unit{
		{ID: "page-1", Kind: "page", Index: 1, Span: document.Span{Start: 0, End: len(prefix)}},
		{ID: "page-2", Kind: "page", Index: 2, Span: document.Span{Start: len(prefix), End: len(prefix) + len(selected)}},
	}}}
	got, err := SelectAttachmentPage(source, 2, 6000)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Excerpts) != 1 || got.Excerpts[0].Text != selected || got.Excerpts[0].StartByte != len(prefix) || got.TotalBytes != len(text) {
		t.Fatalf("wrong page evidence: %+v", got)
	}
	if strings.Contains(AttachmentPrompt("this page", []AttachmentText{got}), "Unrelated") {
		t.Fatal("page scope broadened")
	}
	for _, page := range []int{0, 3} {
		if _, err = SelectAttachmentPage(source, page, 6000); !errors.Is(err, ErrAttachmentUnavailable) {
			t.Fatalf("missing page %d: %v", page, err)
		}
	}
	if _, err = SelectAttachmentPage(source, 2, 10); !errors.Is(err, ErrAttachmentContextLimit) {
		t.Fatal("silently truncated page", err)
	}
	source.Structure.Units[1].Span.Start++
	if _, err = SelectAttachmentPage(source, 2, 6000); !errors.Is(err, ErrAttachmentUnavailable) {
		t.Fatal("invalid UTF8 boundary accepted", err)
	}
}
