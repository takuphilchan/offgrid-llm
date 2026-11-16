package rag

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

// ParseDOCX reads body content in document order. It never regex-recovers invalid
// XML or invents rendered pagination from paragraphs/page-break hints.
func (p *DocumentParser) ParseDOCX(content []byte) (*ParseResult, error) {
	pkg, err := p.officePackage(content)
	if err != nil {
		return nil, err
	}
	data, err := pkg.read("word/document.xml")
	if err != nil {
		return nil, err
	}
	structure := &wordStructure{}
	data, tracked, err := p.docxNotes(pkg, data, structure)
	if err != nil {
		return nil, err
	}
	styles, err := wordStyleLevels(pkg)
	if err != nil {
		return nil, err
	}
	structure.styles = styles
	text, err := extractDOCXBody(data, p.maxExtractedSize, structure)
	if err != nil {
		return nil, err
	}
	result := &ParseResult{Content: text, ContentType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		WordCount: countWords(text), Metadata: map[string]string{"format": "docx", "limitations": "Body text, tables, referenced notes, headers and footers. Page placement and fields are not recalculated; drawings and embedded objects are not interpreted."}}
	if tracked {
		result.Metadata["tracked_changes"] = "present_current_text_only"
		result.Metadata["limitations"] += " Tracked changes are present; current text is searchable and revision alternatives are retained separately for review."
		result.Warnings = append(result.Warnings, ExtractionWarning{Code: "document_tracked_changes", SourceIndex: 1, Message: "Tracked changes are present. Current visible text is searchable; revision alternatives are retained separately for review. Historical formatting is not rendered."})
	}
	bindSourceUnits(result, content)
	// The collector's positions already account for leading/trailing trim.
	if text != "" {
		if err := structure.finish(text, result); err != nil {
			return nil, err
		}
	} else {
		result.SourceUnits[0].State = "empty"
		result.Structure = &document.Extraction{SourceKind: "original_file", OriginalSHA256: result.Metadata["source_sha256"], Format: "docx", Method: "native", Parsers: []document.RuntimeIdentity{{Name: "offgrid-document-parser", Revision: docxDocumentRevision}}, Units: []document.Unit{{ID: "section-1", Kind: "section", Index: 1, State: "empty", ParserRevision: docxDocumentRevision}}}
		result.Structure.Blocks = structure.attachRevisions(nil, "")
	}
	if err := p.appendWordMargins(pkg, data, content, result, styles); err != nil {
		return nil, err
	}
	if result.Content == "" {
		return nil, &ParseError{Code: "document_empty", Message: "This document has no readable text."}
	}
	return result, nil
}

func wordName(name xml.Name) bool {
	return name.Space == "http://schemas.openxmlformats.org/wordprocessingml/2006/main" || name.Space == "http://purl.oclc.org/ooxml/wordprocessingml/main"
}

func extractDOCXBody(data []byte, limit int64, collectors ...*wordStructure) (string, error) {
	return extractWordPart(data, limit, "document", collectors...)
}

func extractWordPart(data []byte, limit int64, root string, collectors ...*wordStructure) (string, error) {
	var structure *wordStructure
	if len(collectors) > 0 {
		structure = collectors[0]
	}
	d := xml.NewDecoder(bytes.NewReader(data))
	out := parserOutput{limit: limit}
	depth, bodyDepth, bodyCount, roots := 0, 0, 0, 0
	skipDepth, textDepth := 0, 0
	for {
		token, err := d.Token()
		if err == io.EOF {
			if roots != 1 || depth != 0 || bodyCount != 1 {
				return "", fmt.Errorf("invalid Word document structure")
			}
			raw := out.String()
			if structure != nil {
				trim := len(raw) - len(strings.TrimLeftFunc(raw, unicode.IsSpace))
				structure.leadingTrim = trim
				for i := range structure.blocks {
					structure.blocks[i].Span.Start -= trim
					structure.blocks[i].Span.End -= trim
				}
			}
			return strings.TrimSpace(raw), nil
		}
		if err != nil {
			return "", err
		}
		if structure != nil {
			if err := structure.revisionToken(token, depth, out.Len()); err != nil {
				return "", err
			}
		}
		var text string
		switch t := token.(type) {
		case xml.Directive:
			return "", fmt.Errorf("document declarations are not supported")
		case xml.CharData:
			if depth == 0 && strings.TrimSpace(string(t)) != "" {
				return "", fmt.Errorf("text outside Word document")
			}
			if textDepth > 0 && skipDepth == 0 {
				text = string(t)
			}
		case xml.StartElement:
			depth++
			if depth > 256 {
				return "", &ParseError{Code: "document_limit", Message: "Document XML nesting exceeds the supported limit."}
			}
			if textDepth > 0 {
				return "", fmt.Errorf("unexpected markup inside Word text")
			}
			if depth == 1 {
				roots++
				if roots != 1 || !wordName(t.Name) || t.Name.Local != root {
					return "", fmt.Errorf("invalid Word document root")
				}
				if root != "document" {
					bodyDepth, bodyCount = 1, 1
				}
			}
			if skipDepth > 0 || !wordName(t.Name) {
				continue
			}
			if t.Name.Local == "body" {
				bodyCount++
				if depth != 2 || bodyCount != 1 {
					return "", fmt.Errorf("invalid Word document body")
				}
				bodyDepth = depth
			}
			if bodyDepth == 0 {
				continue
			}
			if structure != nil {
				structure.start(t, depth, out.Len())
			}
			switch t.Name.Local {
			case "del", "moveFrom", "drawing", "pict", "object", "txbxContent", "sectPrChange", "pPrChange", "rPrChange", "tblPrChange", "trPrChange", "tcPrChange":
				skipDepth = depth
			case "t":
				textDepth = depth
			case "tab":
				text = "\t"
			case "br", "cr":
				text = "\n"
			}
		case xml.EndElement:
			if bodyDepth > 0 && skipDepth == 0 && wordName(t.Name) {
				if structure != nil {
					structure.end(t, depth, out.Len())
				}
				switch t.Name.Local {
				case "p", "tr":
					text = "\n"
				case "tc":
					text = "\t"
				}
			}
			if depth == textDepth {
				textDepth = 0
			}
			if depth == skipDepth {
				skipDepth = 0
			}
			if depth == bodyDepth {
				bodyDepth = 0
			}
			depth--
		}
		if _, err := out.Write([]byte(text)); err != nil {
			return "", err
		}
	}
}
