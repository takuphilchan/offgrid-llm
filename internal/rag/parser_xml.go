package rag

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

func invalidDocumentEncoding() error {
	return &ParseError{Code: "document_invalid", Message: "This document is not valid UTF-8. Export it as UTF-8 before importing."}
}

// XML text is decoded structurally: entities are decoded once, CDATA survives,
// comments are not source text and malformed input is never regex-recovered.
func extractXMLDocument(data []byte, limit int64) (string, error) {
	if !utf8.Valid(data) {
		return "", invalidDocumentEncoding()
	}
	d := xml.NewDecoder(bytes.NewReader(data))
	out := parserOutput{limit: limit}
	roots, depth := 0, 0
	for {
		token, err := d.Token()
		if err == io.EOF {
			if roots != 1 || depth != 0 {
				return "", fmt.Errorf("invalid XML document structure")
			}
			return strings.TrimSpace(out.String()), nil
		}
		if err != nil {
			return "", err
		}
		switch t := token.(type) {
		case xml.Directive:
			return "", fmt.Errorf("XML document declarations are not supported")
		case xml.StartElement:
			if depth == 0 {
				roots++
			}
			depth++
			if depth > 256 {
				return "", &ParseError{Code: "document_limit", Message: "Document XML nesting exceeds the supported limit."}
			}
		case xml.EndElement:
			depth--
			if _, err := out.Write([]byte("\n")); err != nil {
				return "", err
			}
		case xml.CharData:
			if depth == 0 && strings.TrimSpace(string(t)) != "" {
				return "", fmt.Errorf("text outside XML root")
			}
			if depth > 0 {
				if _, err := out.Write(t); err != nil {
					return "", err
				}
			}
		}
	}
}
