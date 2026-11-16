package rag

import (
	"sort"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

// Prefer real paragraph/row boundaries. Chunks remain exact contiguous spans;
// linked headings/notes are separate evidence references, never text prepended
// under an unrelated source offset. Oversized blocks use bounded exact slices.
func (c *Chunker) structuredChunks(doc *Document, source SourceRange, blocks []document.Block) []*Chunk {
	if doc.Extraction == nil {
		return c.offsetChunks(doc.ID, doc.RawContent, source.Start, source.End)
	}
	var spans []document.Span
	for _, b := range blocks {
		if b.Span.Start < source.Start || b.Span.End > source.End {
			continue
		}
		switch b.Kind {
		case "paragraph", "heading", "list", "row", "cell":
			spans = append(spans, b.Span)
		}
	}
	sort.Slice(spans, func(i, j int) bool {
		if spans[i].Start == spans[j].Start {
			return spans[i].End > spans[j].End
		}
		return spans[i].Start < spans[j].Start
	})
	// Outer row/cell spans take precedence over their nested paragraphs.
	boundaries := []int{}
	last := source.Start
	for _, span := range spans {
		if span.End > last {
			boundaries = append(boundaries, span.End)
			last = span.End
		}
	}
	if len(boundaries) == 0 || last < source.End {
		boundaries = append(boundaries, source.End)
	}
	chunks := []*Chunk{}
	start, end := source.Start, source.Start
	flush := func() {
		if end > start {
			chunks = append(chunks, c.offsetChunks(doc.ID, doc.RawContent, start, end)...)
		}
		start = end
	}
	// A single paragraph can exceed the target but is capped at 4 KiB. This
	// keeps ordinary clauses and short linked notes together on CPU profiles.
	maximum := min(4096, max(c.options.ChunkSize, 1024))
	for _, next := range boundaries {
		if next-start > c.options.ChunkSize && end > start {
			flush()
		}
		end = next
		if end-start > maximum {
			flush()
		}
	}
	flush()
	return chunks
}

func (c *Chunker) offsetChunks(id, text string, start, end int) []*Chunk {
	maximum := min(4096, max(c.options.ChunkSize, 1024))
	if end-start <= maximum {
		if strings.TrimSpace(text[start:end]) == "" {
			return nil
		}
		return []*Chunk{{DocumentID: id, Content: text[start:end], StartChar: start, EndChar: end, ContentHash: GenerateContentHash(text[start:end])}}
	}
	chunks := c.exactChunks(id, text[start:end])
	for _, chunk := range chunks {
		chunk.StartChar += start
		chunk.EndChar += start
	}
	return chunks
}
