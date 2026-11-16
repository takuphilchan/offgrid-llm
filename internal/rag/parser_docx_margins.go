package rag

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

type wordMargin struct{ id, kind string }

// Only live section references authorize a part. Unused ZIP parts and historical
// section-property revisions must not silently become current document text.
func wordMarginReferences(data []byte) ([]wordMargin, error) {
	d := xml.NewDecoder(bytes.NewReader(data))
	var stack []xml.Name
	var refs []wordMargin
	skip := 0
	for {
		token, err := d.Token()
		if err == io.EOF {
			return refs, nil
		}
		if err != nil {
			return nil, err
		}
		switch t := token.(type) {
		case xml.Directive:
			return nil, fmt.Errorf("Word declarations are unsupported")
		case xml.StartElement:
			if len(stack) >= 256 {
				return nil, fmt.Errorf("Word nesting exceeds limit")
			}
			parent := xml.Name{}
			if len(stack) > 0 {
				parent = stack[len(stack)-1]
			}
			stack = append(stack, t.Name)
			if skip > 0 {
				continue
			}
			if wordName(t.Name) && (t.Name.Local == "del" || t.Name.Local == "moveFrom" || t.Name.Local == "sectPrChange" || t.Name.Local == "drawing") {
				skip = len(stack)
				continue
			}
			if !wordName(t.Name) || !wordName(parent) || parent.Local != "sectPr" || (t.Name.Local != "headerReference" && t.Name.Local != "footerReference") {
				continue
			}
			id := ""
			for _, a := range t.Attr {
				if a.Name.Local == "id" && (a.Name.Space == "http://schemas.openxmlformats.org/officeDocument/2006/relationships" || a.Name.Space == "http://purl.oclc.org/ooxml/officeDocument/relationships") {
					id = a.Value
				}
			}
			if id == "" {
				return nil, fmt.Errorf("missing Word margin relationship")
			}
			refs = append(refs, wordMargin{id, strings.TrimSuffix(t.Name.Local, "Reference")})
			if len(refs) > 1000 {
				return nil, &ParseError{Code: "document_limit", Message: "Too many document header/footer references."}
			}
		case xml.EndElement:
			if len(stack) == skip {
				skip = 0
			}
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}
}

func (p *DocumentParser) appendWordMargins(pkg *officePackage, data, original []byte, result *ParseResult, styles map[string]int) error {
	refs, err := wordMarginReferences(data)
	if err != nil {
		return err
	}
	if len(refs) == 0 {
		return nil
	}
	rels, err := pkg.relations("word/document.xml")
	if err != nil {
		return err
	}
	seen := map[string]string{}
	for _, ref := range refs {
		part, err := officePart("word/document.xml", ref.id, ref.kind, rels)
		if err != nil {
			return err
		}
		if kind, exists := seen[part]; exists {
			if kind != ref.kind {
				return fmt.Errorf("Word margin part has conflicting roles")
			}
			continue
		}
		seen[part] = ref.kind
		if len(result.SourceUnits) >= 1000 {
			return &ParseError{Code: "document_limit", Message: "Too many document source sections."}
		}
		raw, err := pkg.read(part)
		if err != nil {
			return err
		}
		collector := &wordStructure{styles: styles}
		raw, tracked, err := p.docxNotes(pkg, raw, collector)
		if err != nil {
			return err
		}
		root := "hdr"
		if ref.kind == "footer" {
			root = "ftr"
		}
		text, err := extractWordPart(raw, p.maxExtractedSize, root, collector)
		if err != nil {
			return err
		}
		if int64(len(result.Content)+len(text)+1) > p.maxExtractedSize {
			return &ParseError{Code: "document_limit", Message: "Extracted document text exceeds the supported limit."}
		}
		index := len(result.SourceUnits) + 1
		if tracked {
			result.Metadata["tracked_changes"] = "present_current_text_only"
			result.Warnings = append(result.Warnings, ExtractionWarning{Code: "document_tracked_changes", SourceIndex: index, Message: "Tracked changes are present in this header or footer; deleted text is excluded from the current reading. Text revision alternatives are retained separately for review; historical formatting is not rendered."})
		}
		result.Warnings = append(result.Warnings, ExtractionWarning{Code: "document_margin_layout", SourceIndex: index, Message: "This referenced header or footer is included once as a source section. First/even-page visibility, inherited section placement and rendered page fields are not resolved."})
		id := fmt.Sprintf("section-%d", index)
		offset := len(result.Content)
		if text != "" && offset > 0 {
			result.Content += "\n"
			offset++
		}
		state := "ready"
		if text == "" {
			state = "empty"
		}
		result.SourceUnits = append(result.SourceUnits, SourceUnit{Kind: "section", Index: index, Name: part, Text: text, State: state, SourceSHA256: result.Metadata["source_sha256"], ExtractorRevision: docxDocumentRevision})
		result.Structure.Units = append(result.Structure.Units, document.Unit{ID: id, Kind: "section", Index: index, Name: part, Span: document.Span{Start: offset, End: offset + len(text)}, State: state, ParserRevision: docxDocumentRevision})
		if text != "" || len(collector.revisions) > 0 {
			tmp := &ParseResult{Content: text, Metadata: map[string]string{"format": "docx"}}
			bindSourceUnits(tmp, original)
			if text == "" {
				tmp.Structure = &document.Extraction{Blocks: collector.attachRevisions(nil, "")}
			} else if err = collector.finish(text, tmp); err != nil {
				return err
			}
			container := id + "-" + ref.kind
			result.Structure.Blocks = append(result.Structure.Blocks, document.Block{ID: container, UnitID: id, Kind: ref.kind, Method: "native", Span: document.Span{Start: offset, End: offset + len(text)}, SourcePart: part})
			parents := map[string]bool{}
			for _, r := range tmp.Structure.Relationships {
				if r.Kind == "parent" {
					parents[r.From] = true
				}
				r.From = id + "-" + r.From
				r.To = id + "-" + r.To
				result.Structure.Relationships = append(result.Structure.Relationships, r)
			}
			for _, b := range tmp.Structure.Blocks {
				oldID := b.ID
				b.ID = id + "-" + b.ID
				b.UnitID = id
				b.Span.Start += offset
				b.Span.End += offset
				if b.SourcePart == "word/document.xml" {
					b.SourcePart = part
				}
				result.Structure.Blocks = append(result.Structure.Blocks, b)
				if !parents[oldID] {
					result.Structure.Relationships = append(result.Structure.Relationships, document.Relationship{From: b.ID, To: container, Kind: "parent"})
				}
			}
		}
		result.Content += text
	}
	result.Structure.Text = result.Content
	result.WordCount = countWords(result.Content)
	result.Structure.Coverage = result.Coverage()
	return result.Structure.Seal()
}
