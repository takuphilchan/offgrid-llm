package sessions

import (
	"encoding/json"
	"sort"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

// Spans are covered by the union of selected passages, including adjacent
// chunks. No combined passage is assigned an invented contiguous source range.
func attachmentSpanCovered(span document.Span, excerpts []AttachmentExcerpt) bool {
	ranges := append([]AttachmentExcerpt(nil), excerpts...)
	sort.Slice(ranges, func(i, j int) bool { return ranges[i].StartByte < ranges[j].StartByte })
	start := span.Start
	for _, r := range ranges {
		if r.StartByte > start {
			break
		}
		if r.EndByte > start {
			start = r.EndByte
		}
		if start >= span.End {
			return true
		}
	}
	return false
}

func attachmentLinkReceipt(source AttachmentText, links *document.LinkIndex) *document.SelectionReceipt {
	if links == nil {
		return nil
	}
	r := &document.SelectionReceipt{Revision: source.Structure.ID, LinksComplete: true}
	seen := map[string]bool{}
	for _, excerpt := range source.Excerpts {
		linked := links.ForSpan(document.Span{Start: excerpt.StartByte, End: excerpt.EndByte})
		r.LimitReached = r.LimitReached || linked.LimitReached
		for _, block := range linked.Required {
			if seen[block.ID] {
				continue
			}
			seen[block.ID] = true
			if len(r.RequiredSpans) < 32 {
				r.RequiredSpans = append(r.RequiredSpans, block.Span)
			} else {
				r.LimitReached = true
			}
			if !attachmentSpanCovered(block.Span, source.Excerpts) {
				r.LinksComplete = false
				if len(r.OmittedBlocks) < 16 {
					r.OmittedBlocks = append(r.OmittedBlocks, block.ID)
				} else {
					r.LimitReached = true
				}
			}
		}
	}
	if r.LimitReached {
		r.LinksComplete = false
	}
	return r
}

func includeAttachmentLinks(source AttachmentText, text string, anchor AttachmentExcerpt, links *document.LinkIndex, budget int) AttachmentText {
	if links == nil {
		return source
	}
	linked := links.ForSpan(document.Span{Start: anchor.StartByte, End: anchor.EndByte})
	for _, block := range linked.Required {
		if len(source.Excerpts) >= 64 {
			break
		}
		if attachmentSpanCovered(block.Span, source.Excerpts) {
			continue
		}
		// The graph was verified against the retained text before selection.
		if block.Span.End-block.Span.Start > budget {
			continue
		}
		probe := source
		probe.Excerpts = append(append([]AttachmentExcerpt(nil), source.Excerpts...), AttachmentExcerpt{StartByte: block.Span.Start, EndByte: block.Span.End, Text: text[block.Span.Start:block.Span.End]})
		probe.LinkedEvidence = attachmentLinkReceipt(probe, links)
		encoded, _ := json.Marshal(probe)
		if len(encoded) <= budget {
			source = probe
		}
	}
	return source
}
