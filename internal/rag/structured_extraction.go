package rag

import (
	"fmt"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

// projectExtraction preserves source byte spans. It does not infer headings,
// table headers or page geometry from the appearance of plain text. Format
// adapters can supply richer verified blocks without changing this contract.
func projectExtraction(text, format string, ranges []SourceRange, coverage *document.Coverage) (*document.Extraction, error) {
	if err := validateSourceRanges(text, ranges); err != nil {
		return nil, err
	}
	e := &document.Extraction{SourceKind: "legacy_text", Format: format, Method: "legacy", Text: text, Coverage: coverage}
	if len(ranges) > 0 {
		e.SourceKind = "original_file"
		e.OriginalSHA256 = ranges[0].SHA256
		e.Method = "native"
	}
	parsers := map[string]bool{}
	for _, r := range ranges {
		id := fmt.Sprintf("%s-%d", r.Kind, r.Index)
		e.Units = append(e.Units, document.Unit{ID: id, Kind: r.Kind, Index: r.Index, Name: r.Name, Span: document.Span{Start: r.Start, End: r.End}, State: "ready", ParserRevision: r.Revision})
		if !parsers[r.Revision] {
			e.Parsers = append(e.Parsers, document.RuntimeIdentity{Name: "offgrid-document-parser", Revision: r.Revision})
			parsers[r.Revision] = true
		}
	}
	if len(e.Units) == 0 {
		e.Units = []document.Unit{{ID: "section-1", Kind: "section", Index: 1, Span: document.Span{End: len(text)}, State: "unknown", ParserRevision: "legacy"}}
	}
	for i, u := range e.Units {
		method := e.Method
		// Old receipts bind the combined OCR pipeline but not individual page
		// methods. Do not relabel its transcription as authoritative native text.
		if strings.Contains(u.ParserRevision, "ocr") {
			method = "mixed"
			e.Method = "mixed"
		}
		if i < len(ranges) && len(ranges[i].Cells) > 0 {
			for _, c := range ranges[i].Cells {
				e.Blocks = append(e.Blocks, document.Block{ID: fmt.Sprintf("%s-cell-%s", u.ID, c.Reference), UnitID: u.ID, Kind: "cell", Method: method, Span: document.Span{Start: c.Start, End: c.End}, Cell: &document.Cell{Reference: c.Reference, Display: text[c.Start:c.End]}})
			}
			continue
		}
		for start := u.Span.Start; start < u.Span.End; {
			end := u.Span.End
			if newline := strings.IndexByte(text[start:end], '\n'); newline >= 0 {
				end = start + newline + 1
			}
			if strings.TrimSpace(text[start:end]) != "" {
				e.Blocks = append(e.Blocks, document.Block{ID: fmt.Sprintf("%s-block-%d", u.ID, len(e.Blocks)+1), UnitID: u.ID, Kind: "paragraph", Method: method, Span: document.Span{Start: start, End: end}})
			}
			start = end
		}
	}
	if err := e.Seal(); err != nil {
		return nil, err
	}
	return e, nil
}

func (d *Document) ensureExtraction() error {
	if d.Extraction == nil {
		var err error
		d.Extraction, err = projectExtraction(d.RawContent, d.Metadata["format"], d.SourceRanges, d.ExtractionCoverage())
		if err != nil {
			return err
		}
	}
	if err := d.Extraction.Verify(); err != nil {
		return err
	}
	if d.Extraction.Text != d.RawContent {
		return fmt.Errorf("structured extraction does not match retained text")
	}
	if len(d.SourceRanges) > 0 && d.Extraction.OriginalSHA256 != d.SourceRanges[0].SHA256 {
		return fmt.Errorf("structured extraction does not match source identity")
	}
	d.ExtractionRevision = d.Extraction.ID
	return nil
}

func (d *Document) ExtractionCoverage() *document.Coverage {
	return (SearchResult{Metadata: d.Metadata}).ExtractionCoverage()
}

func (r *ParseResult) bindStructure() error {
	text, ranges, err := parsedSourceRanges(r)
	if err != nil {
		return err
	}
	if r.Structure == nil {
		r.Structure, err = projectExtraction(text, r.Metadata["format"], ranges, r.Coverage())
		if err != nil {
			return err
		}
		// Empty pages/sheets are accounted-for source units even though they
		// produce no searchable ranges. Retain their position and parser state.
		known := map[string]document.Unit{}
		for _, u := range r.Structure.Units {
			known[u.ID] = u
		}
		units := make([]document.Unit, 0, len(r.SourceUnits))
		cursor := 0
		for _, source := range r.SourceUnits {
			id := fmt.Sprintf("%s-%d", source.Kind, source.Index)
			u, ok := known[id]
			if !ok {
				if source.State != "empty" || strings.TrimSpace(source.Text) != "" {
					return fmt.Errorf("unmapped structured source unit")
				}
				u = document.Unit{ID: id, Kind: source.Kind, Index: source.Index, Name: source.Name, Span: document.Span{Start: cursor, End: cursor}, ParserRevision: source.ExtractorRevision}
			}
			u.State = source.State
			units = append(units, u)
			cursor = u.Span.End
		}
		if len(units) > 0 {
			r.Structure.Units = units
		}
		if err := r.enrichPDFFields(); err != nil {
			return err
		}
		if err := r.enrichXLSXStructure(); err != nil {
			return err
		}
	}
	if r.Structure.Text != text {
		return fmt.Errorf("structured parser text mismatch")
	}
	r.Structure.Coverage = r.Coverage()
	if digest := r.Metadata["ocr_language_sha256"]; digest != "" {
		r.Structure.LanguagePacks = ocrRuntimeIdentities(r)
	}
	return r.Structure.Seal()
}
