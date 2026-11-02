package rag

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

// SourceRange identifies extractor text within retained RawContent. Offsets are
// UTF-8 bytes, not rendered document coordinates. No document text is duplicated
// in list/search metadata.
type SourceRange struct {
	Cells    []SourceCell `json:"cells,omitempty"`
	Kind     string       `json:"kind"`
	Index    int          `json:"index"`
	Name     string       `json:"name,omitempty"`
	Start    int          `json:"start_byte"`
	End      int          `json:"end_byte"`
	SHA256   string       `json:"source_sha256"`
	Revision string       `json:"extractor_revision"`
}

func (e *Engine) IngestParsed(ctx context.Context, name string, result *ParseResult, metadata map[string]string) (*Document, error) {
	content, ranges, err := parsedSourceRanges(result)
	if err != nil {
		return nil, err
	}
	return e.ingestText(ctx, name, content, metadata, ranges, result.ContentType, nil)
}

func parsedSourceRanges(result *ParseResult) (string, []SourceRange, error) {
	if result == nil || result.State != ExtractionComplete {
		return "", nil, &ParseError{Code: "document_invalid", Message: "A complete extraction is required before indexing."}
	}
	content := strings.TrimSpace(result.Content)
	ranges := make([]SourceRange, 0, len(result.SourceUnits))
	cursor := 0
	for _, unit := range result.SourceUnits {
		if unit.State == "empty" && strings.TrimSpace(unit.Text) == "" {
			continue
		}
		if unit.State != "ready" {
			return "", nil, fmt.Errorf("incomplete source unit cannot be indexed as complete")
		}
		text := strings.TrimSpace(unit.Text)
		if text == "" {
			return "", nil, fmt.Errorf("ready source unit contains no text")
		}
		rel := strings.Index(content[cursor:], text)
		if rel < 0 {
			return "", nil, fmt.Errorf("extracted source unit does not match retained text")
		}
		start := cursor + rel
		source := SourceRange{Kind: unit.Kind, Index: unit.Index, Name: unit.Name, Start: start, End: start + len(text), SHA256: unit.SourceSHA256, Revision: unit.ExtractorRevision}
		for _, cell := range unit.Cells {
			if cell.Start < 0 || cell.End <= cell.Start || cell.End > len(text) {
				return "", nil, fmt.Errorf("invalid extracted cell range")
			}
			source.Cells = append(source.Cells, SourceCell{Reference: cell.Reference, Start: start + cell.Start, End: start + cell.End})
		}
		ranges = append(ranges, source)
		cursor = start + len(text)
	}
	if len(ranges) == 0 {
		return "", nil, &ParseError{Code: "document_empty", Message: "The document contains no usable text."}
	}
	if err := validateSourceRanges(content, ranges); err != nil {
		return "", nil, err
	}
	return content, ranges, nil
}

func validateSourceRanges(content string, ranges []SourceRange) error {
	end := 0
	for _, r := range ranges {
		_, err := hex.DecodeString(r.SHA256)
		if r.Start < end || r.End <= r.Start || r.End > len(content) || r.Index < 1 || len(r.SHA256) != 64 || err != nil || r.Revision == "" || !utf8.ValidString(content[r.Start:r.End]) {
			return fmt.Errorf("invalid retained source range")
		}
		if r.SHA256 != ranges[0].SHA256 {
			return fmt.Errorf("source ranges refer to different original documents")
		}
		if !validateSourceCells(content, r) {
			return fmt.Errorf("invalid retained cell range")
		}
		switch r.Kind {
		case "page", "slide", "sheet", "section":
		default:
			return fmt.Errorf("unsupported retained source unit")
		}
		end = r.End
	}
	return nil
}

func sourceRangesJSON(ranges []SourceRange) string {
	data, _ := json.Marshal(ranges)
	return string(data)
}

func bindChunkSource(chunk *Chunk, ranges []SourceRange) {
	for _, r := range ranges {
		if chunk.StartChar >= r.Start && chunk.EndChar <= r.End {
			chunk.Source = sourceForSpan(r, chunk.StartChar, chunk.EndChar)
			return
		}
	}
}

func (c *Chunker) chunkDocument(doc *Document) ([]*Chunk, error) {
	if len(doc.SourceRanges) == 0 {
		return c.ChunkText(doc.ID, doc.RawContent), nil
	}
	if err := validateSourceRanges(doc.RawContent, doc.SourceRanges); err != nil {
		return nil, err
	}
	var chunks []*Chunk
	for _, r := range doc.SourceRanges {
		for _, chunk := range c.exactChunks(doc.ID, doc.RawContent[r.Start:r.End]) {
			chunk.Index = len(chunks)
			chunk.ID = GenerateChunkID(doc.ID, chunk.Index)
			chunk.StartChar += r.Start
			chunk.EndChar += r.Start
			if r.Kind == "page" {
				chunk.Page = r.Index
			} else {
				chunk.Section = fmt.Sprintf("%s %d", r.Kind, r.Index)
				if r.Name != "" {
					chunk.Section += " · " + r.Name
				}
			}
			chunk.Source = sourceForSpan(r, chunk.StartChar, chunk.EndChar)
			chunks = append(chunks, chunk)
		}
	}
	return chunks, nil
}

// Keep exact substrings, including whitespace, so offsets can be independently
// read back. A rune is never divided, even when its bytes exceed a tiny budget.
func (c *Chunker) exactChunks(id, text string) []*Chunk {
	var chunks []*Chunk
	for start := 0; start < len(text); {
		end := min(start+c.options.ChunkSize, len(text))
		for end < len(text) && !utf8.RuneStart(text[end]) {
			end++
		}
		if strings.TrimSpace(text[start:end]) != "" {
			index := len(chunks)
			content := text[start:end]
			chunks = append(chunks, &Chunk{ID: GenerateChunkID(id, index), DocumentID: id, Index: index, Content: content, ContentHash: GenerateContentHash(content), StartChar: start, EndChar: end})
		}
		if end == len(text) {
			break
		}
		next := end - c.options.ChunkOverlap
		for next < end && !utf8.RuneStart(text[next]) {
			next++
		}
		if next <= start {
			next = end
		}
		start = next
	}
	return chunks
}
