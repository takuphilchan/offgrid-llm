package rag

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
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
	text, err := extractDOCXBody(data, p.maxExtractedSize)
	if err != nil {
		return nil, err
	}
	if text == "" {
		return nil, &ParseError{Code: "document_empty", Message: "This document has no readable body text."}
	}
	result := &ParseResult{Content: text, ContentType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		WordCount: countWords(text), Metadata: map[string]string{"format": "docx", "limitations": "Body text and tables only; rendered page numbers, headers, notes, deleted text and drawing content are not available."}}
	bindSourceUnits(result, content)
	return result, nil
}

func wordName(name xml.Name) bool {
	return name.Space == "http://schemas.openxmlformats.org/wordprocessingml/2006/main" || name.Space == "http://purl.oclc.org/ooxml/wordprocessingml/main"
}

func extractDOCXBody(data []byte, limit int64) (string, error) {
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
			return strings.TrimSpace(out.String()), nil
		}
		if err != nil {
			return "", err
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
				if roots != 1 || !wordName(t.Name) || t.Name.Local != "document" {
					return "", fmt.Errorf("invalid Word document root")
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
			switch t.Name.Local {
			case "del", "drawing", "pict", "object", "txbxContent":
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
