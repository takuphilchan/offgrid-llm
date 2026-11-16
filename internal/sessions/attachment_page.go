package sessions

import (
	"encoding/json"
	"github.com/takuphilchan/offgrid-llm/internal/document"
	"unicode/utf8"
)

// SelectAttachmentPage retains original byte coordinates and linked exceptions.
// It never falls back to a different page when the requested page is unread.
func SelectAttachmentPage(source AttachmentText, page, budget int) (AttachmentText, error) {
	if source.Structure == nil || page < 1 {
		return AttachmentText{}, ErrAttachmentUnavailable
	}
	for _, unit := range source.Structure.Units {
		if unit.Kind != "page" || unit.Index != page {
			continue
		}
		span := unit.Span
		if span.Start < 0 || span.End <= span.Start || span.End > len(source.Text) || !utf8.ValidString(source.Text[span.Start:span.End]) {
			return AttachmentText{}, ErrAttachmentUnavailable
		}
		out := AttachmentText{Attachment: source.Attachment, Structure: source.Structure, Selection: "excerpts", TotalBytes: len(source.Text), Excerpts: []AttachmentExcerpt{{StartByte: span.Start, EndByte: span.End, Text: source.Text[span.Start:span.End]}}}
		links := document.IndexLinks(source.Structure)
		out = includeAttachmentLinks(out, source.Text, out.Excerpts[0], links, max(0, budget-(span.End-span.Start)))
		out.LinkedEvidence = attachmentLinkReceipt(out, links)
		encoded, err := json.Marshal(out)
		if err != nil || len(encoded) > budget {
			return AttachmentText{}, ErrAttachmentContextLimit
		}
		return out, nil
	}
	return AttachmentText{}, ErrAttachmentUnavailable
}
