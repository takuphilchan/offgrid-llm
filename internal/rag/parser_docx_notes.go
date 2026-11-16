package rag

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

const wordNamespace = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"

// Referenced notes are inserted beside the reference, not appended to an unrelated
// end-of-document chunk. The original ZIP is retained unchanged.
func (p *DocumentParser) docxNotes(pkg *officePackage, data []byte, collectors ...*wordStructure) ([]byte, bool, error) {
	notes := map[string]map[string]string{}
	revisions := map[string]map[string][]document.RevisionChange{}
	for _, kind := range []string{"footnote", "endnote"} {
		part := "word/" + kind + "s.xml"
		if pkg.files[part] == nil {
			continue
		}
		raw, err := pkg.read(part)
		if err != nil {
			return nil, false, err
		}
		revisions[kind] = map[string][]document.RevisionChange{}
		values, err := readWordNotes(raw, kind, p.maxExtractedSize, revisions[kind])
		if err != nil {
			return nil, false, err
		}
		notes[kind] = values
	}
	d := xml.NewDecoder(bytes.NewReader(data))
	var out bytes.Buffer
	last := 0
	tracked := false
	depth, skip := 0, 0
	for {
		before := int(d.InputOffset())
		token, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, false, err
		}
		switch t := token.(type) {
		case xml.Directive:
			return nil, false, fmt.Errorf("Word declarations are unsupported")
		case xml.StartElement:
			depth++
			if depth > 256 {
				return nil, false, fmt.Errorf("Word nesting exceeds limit")
			}
			if !wordName(t.Name) {
				continue
			}
			propertyChange := t.Name.Local == "sectPrChange" || t.Name.Local == "pPrChange" || t.Name.Local == "rPrChange" || t.Name.Local == "tblPrChange" || t.Name.Local == "trPrChange" || t.Name.Local == "tcPrChange"
			if t.Name.Local == "ins" || t.Name.Local == "del" || t.Name.Local == "moveFrom" || t.Name.Local == "moveTo" || propertyChange {
				tracked = true
			}
			if skip > 0 {
				continue
			}
			if t.Name.Local == "del" || t.Name.Local == "moveFrom" || t.Name.Local == "drawing" || propertyChange {
				skip = depth
				continue
			}
			kind := strings.TrimSuffix(t.Name.Local, "Reference")
			if kind != "footnote" && kind != "endnote" {
				continue
			}
			id := ""
			for _, a := range t.Attr {
				if a.Name.Local == "id" {
					id = a.Value
				}
			}
			value, ok := notes[kind][id]
			if !ok {
				return nil, false, fmt.Errorf("referenced %s %s is missing", kind, id)
			}
			if changes := revisions[kind][id]; len(changes) > 0 {
				tracked = true
				if len(collectors) > 0 {
					collector := collectors[0]
					if collector.noteRevisions == nil {
						collector.noteRevisions = map[string][]document.RevisionChange{}
					}
					collector.noteRevisions[kind+":"+id] = changes
				}
			}
			out.Write(data[last:before])
			out.WriteString(`<w:t xmlns:w="` + wordNamespace + `" xml:space="preserve"> [` + kind + " " + id + ": ")
			xml.EscapeText(&out, []byte(value))
			out.WriteString("] </w:t>")
			last = before
			if int64(out.Len()) > p.maxExtractedSize {
				return nil, false, &ParseError{Code: "document_limit", Message: "Expanded document notes exceed the supported limit."}
			}
		case xml.EndElement:
			if depth == skip {
				skip = 0
			}
			depth--
		}
	}
	out.Write(data[last:])
	return out.Bytes(), tracked, nil
}

func readWordNotes(data []byte, kind string, limit int64, revisionMaps ...map[string][]document.RevisionChange) (map[string]string, error) {
	d := xml.NewDecoder(bytes.NewReader(data))
	out := map[string]string{}
	total := int64(0)
	for {
		t, err := d.Token()
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		if _, ok := t.(xml.Directive); ok {
			return nil, fmt.Errorf("Word declarations are unsupported")
		}
		start, ok := t.(xml.StartElement)
		if !ok || !wordName(start.Name) || start.Name.Local != kind {
			continue
		}
		identity, noteType := "", ""
		for _, attr := range start.Attr {
			if wordName(attr.Name) && attr.Name.Local == "id" {
				identity = attr.Value
			}
			if wordName(attr.Name) && attr.Name.Local == "type" {
				noteType = attr.Value
			}
		}
		id, err := strconv.Atoi(identity)
		if err != nil {
			return nil, fmt.Errorf("invalid note identity")
		}
		if noteType == "separator" || noteType == "continuationSeparator" || noteType == "continuationNotice" || id < 0 {
			if err := d.Skip(); err != nil {
				return nil, err
			}
			continue
		}
		if _, ok := out[identity]; ok {
			return nil, fmt.Errorf("duplicate note identity")
		}
		// Re-encode resolved namespace names instead of copying inner XML that
		// may depend on prefixes declared on a parent part.
		var body bytes.Buffer
		enc := xml.NewEncoder(&body)
		depth := 1
		for depth > 0 {
			token, err := d.Token()
			if err != nil {
				return nil, err
			}
			switch v := token.(type) {
			case xml.Directive:
				return nil, fmt.Errorf("Word declarations are unsupported")
			case xml.StartElement:
				depth++
				if depth > 256 {
					return nil, fmt.Errorf("Word notes nesting exceeds limit")
				}
				attrs := v.Attr[:0]
				for _, a := range v.Attr {
					if a.Name.Space != "xmlns" && a.Name.Local != "xmlns" {
						attrs = append(attrs, a)
					}
				}
				v.Attr = attrs
				token = v
			case xml.EndElement:
				depth--
			}
			if depth > 0 {
				if err := enc.EncodeToken(token); err != nil {
					return nil, err
				}
			}
			if int64(body.Len()) > limit {
				return nil, fmt.Errorf("document notes exceed limit")
			}
		}
		if err := enc.Flush(); err != nil {
			return nil, err
		}
		wrapped := `<w:document xmlns:w="` + wordNamespace + `"><w:body>` + body.String() + `</w:body></w:document>`
		collector := &wordStructure{}
		value, err := extractDOCXBody([]byte(wrapped), limit, collector)
		if err != nil {
			return nil, err
		}
		if len(revisionMaps) > 0 {
			for _, capture := range collector.revisions {
				revisionMaps[0][identity] = append(revisionMaps[0][identity], capture.change)
				total += int64(len(capture.change.Text))
			}
		}
		total += int64(len(value))
		if total > limit || len(out) >= 10000 {
			return nil, fmt.Errorf("document notes exceed limit")
		}
		out[identity] = value
	}
}
