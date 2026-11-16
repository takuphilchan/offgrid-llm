package rag

import (
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

// ParsePPTX uses the presentation's relationship order, which can differ from
// both ZIP order and slide filenames after a presentation has been rearranged.
func (p *DocumentParser) ParsePPTX(content []byte) (*ParseResult, error) {
	pkg, err := p.officePackage(content)
	if err != nil {
		return nil, err
	}
	// sldId has both an unqualified numeric id and a relationship r:id; decode
	// with the namespace-aware helper to avoid treating numeric IDs as paths.
	data, err := pkg.read("ppt/presentation.xml")
	if err != nil {
		return nil, err
	}
	ids, err := presentationSlideIDs(data)
	if err != nil {
		return nil, err
	}
	relations, err := pkg.relations("ppt/presentation.xml")
	if err != nil {
		return nil, err
	}
	b := &presentationBuilder{pkg: pkg, limit: p.maxExtractedSize}
	var units []SourceUnit
	var graphUnits []document.Unit
	seenParts := map[string]bool{}
	for i, id := range ids {
		part, err := officePart("ppt/presentation.xml", id, "slide", relations)
		if err != nil {
			return nil, err
		}
		if seenParts[part] {
			return nil, fmt.Errorf("duplicate presentation slide")
		}
		seenParts[part] = true
		data, err := pkg.read(part)
		if err != nil {
			return nil, err
		}
		node, err := presentationTree(data)
		if err != nil {
			return nil, err
		}
		if node.name.Local != "sld" || !node.presentation() {
			return nil, fmt.Errorf("invalid slide root")
		}
		b.index = i + 1
		b.unit = fmt.Sprintf("slide-%d", i+1)
		fmt.Fprintf(&b.out, "--- Slide %d ---\n", i+1)
		start := b.out.Len()
		bodyStart := len(b.blocks)
		linkStart := len(b.links)
		if err = b.loadRelations(part); err != nil {
			return nil, err
		}
		hidden := node.attr("show") == "0" || node.attr("show") == "false"
		if hidden {
			if _, err = b.add("paragraph", "[Hidden slide]"); err != nil {
				return nil, err
			}
			b.warn("document_hidden_content", "Hidden slide content is included and marked; inspect visibility before using it.")
		}
		if err = b.walk(node, false); err != nil {
			return nil, err
		}
		bodyEnd := len(b.blocks)
		var notesPart string
		for relID, r := range b.relations {
			if !strings.HasSuffix(r.Type, "/notesSlide") {
				continue
			}
			if notesPart != "" {
				return nil, fmt.Errorf("ambiguous slide notes")
			}
			notesPart, err = officePart(part, relID, "notesSlide", b.relations)
			if err != nil {
				return nil, err
			}
		}
		if notesPart != "" {
			notesData, err := pkg.read(notesPart)
			if err != nil {
				return nil, err
			}
			notes, err := presentationTree(notesData)
			if err != nil {
				return nil, err
			}
			if notes.name.Local != "notes" || !notes.presentation() {
				return nil, fmt.Errorf("invalid slide notes root")
			}
			if err = b.loadRelations(notesPart); err != nil {
				return nil, err
			}
			noteStart := b.out.Len()
			noteBlockStart := len(b.blocks)
			if err = b.walk(notes, true); err != nil {
				return nil, err
			}
			if b.out.Len() > noteStart {
				b.warn("document_speaker_notes", "Speaker notes are included with their slide and separate source-part identity; they are not text displayed on the slide.")
				container, err := b.add("footnote", "")
				if err != nil {
					return nil, err
				}
				b.blocks[container].Span = document.Span{Start: noteStart, End: b.out.Len()}
				b.blocks[container].ReviewState = "speaker_notes"
				children := map[string]bool{}
				for _, link := range b.links[linkStart:] {
					if link.Kind == "parent" {
						children[link.From] = true
					}
				}
				for at := noteBlockStart; at < container; at++ {
					if !children[b.blocks[at].ID] {
						b.parent(at, container)
					}
				}
				for at := bodyStart; at < bodyEnd; at++ {
					if !children[b.blocks[at].ID] {
						b.links = append(b.links, document.Relationship{From: b.blocks[container].ID, To: b.blocks[at].ID, Kind: "notes_for"})
					}
				}
			}
		}
		end := b.out.Len()
		text := strings.TrimSpace(b.out.String()[start:end])
		state := "ready"
		if text == "" {
			state = "empty"
		}
		units = append(units, SourceUnit{Kind: "slide", Index: i + 1, Text: text, State: state, Hidden: hidden})
		graphUnits = append(graphUnits, document.Unit{ID: b.unit, Kind: "slide", Index: i + 1, Span: document.Span{Start: start, End: end}, State: state, ParserRevision: pptxDocumentRevision, Hidden: hidden})
		if int64(b.out.Len()) > p.maxExtractedSize {
			return nil, &ParseError{Code: "document_limit", Message: "Extracted slide text exceeds the supported limit."}
		}
	}
	readable := false
	for _, u := range units {
		readable = readable || u.Text != ""
	}
	text := strings.TrimSpace(b.out.String())
	if !readable {
		return nil, fmt.Errorf("no readable slide text")
	}
	// Unit/block ends omit outer whitespace while retaining exact source spans.
	for i := range graphUnits {
		graphUnits[i].Span.End = min(graphUnits[i].Span.End, len(text))
		graphUnits[i].Span.Start = min(graphUnits[i].Span.Start, len(text))
	}
	for i := range b.blocks {
		b.blocks[i].Span.End = min(b.blocks[i].Span.End, len(text))
	}
	limitations := "Native slide text, speaker notes and table structure in XML order. Master/layout text, animations and visual reading order are not resolved. Images, charts and embedded objects are identified but not interpreted. Table styling is not proof of semantic headers."
	result := &ParseResult{Content: text, ContentType: "application/vnd.openxmlformats-officedocument.presentationml.presentation", Metadata: map[string]string{"format": "pptx", "slide_count": strconv.Itoa(len(ids)), "limitations": limitations}, PageCount: len(ids), WordCount: countWords(text), SourceUnits: units, Warnings: append([]ExtractionWarning{{Code: "document_text_only", Message: limitations}}, b.warnings...)}
	bindSourceUnits(result, content)
	result.Structure = &document.Extraction{SourceKind: "original_file", OriginalSHA256: result.Metadata["source_sha256"], Format: "pptx", Method: "native", Text: text, Units: graphUnits, Blocks: b.blocks, Relationships: b.links, Parsers: []document.RuntimeIdentity{{Name: "offgrid-document-parser", Revision: pptxDocumentRevision}}, Coverage: result.Coverage()}
	if err = result.Structure.Seal(); err != nil {
		return nil, err
	}
	return result, nil
}

func presentationSlideIDs(data []byte) ([]string, error) {
	d := xml.NewDecoder(strings.NewReader(string(data)))
	var ids []string
	for {
		token, err := d.Token()
		if err == io.EOF {
			return ids, nil
		}
		if err != nil {
			return nil, err
		}
		if start, ok := token.(xml.StartElement); ok && start.Name.Local == "sldId" {
			id := ""
			for _, a := range start.Attr {
				if a.Name.Local == "id" && strings.HasSuffix(a.Name.Space, "/relationships") {
					id = a.Value
				}
			}
			if id == "" {
				return nil, fmt.Errorf("missing slide relationship")
			}
			ids = append(ids, id)
			if len(ids) > 1000 {
				return nil, &ParseError{Code: "document_limit", Message: "Presentation exceeds the 1,000-slide limit."}
			}
		}
	}
}
