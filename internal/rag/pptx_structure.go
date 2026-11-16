package rag

import (
	"fmt"
	"path"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

type presentationBuilder struct {
	pkg        *officePackage
	limit      int64
	out        strings.Builder
	blocks     []document.Block
	links      []document.Relationship
	warnings   []ExtractionWarning
	unit, part string
	index      int
	relations  map[string]officeRelation
}

func (b *presentationBuilder) warn(code, message string) {
	for _, w := range b.warnings {
		if w.Code == code && w.SourceIndex == b.index {
			return
		}
	}
	b.warnings = append(b.warnings, ExtractionWarning{Code: code, Message: message, SourceIndex: b.index})
}
func (b *presentationBuilder) add(kind, text string) (int, error) {
	if len(b.blocks) >= 200000 || int64(b.out.Len()+len(text)+1) > b.limit {
		return 0, &ParseError{Code: "document_limit", Message: "Presentation extraction exceeds supported bounds."}
	}
	start := b.out.Len()
	b.out.WriteString(text)
	i := len(b.blocks)
	b.blocks = append(b.blocks, document.Block{ID: fmt.Sprintf("presentation-block-%d", i+1), UnitID: b.unit, Kind: kind, Method: "native", SourcePart: b.part, Span: document.Span{Start: start, End: b.out.Len()}})
	if text != "" {
		b.out.WriteByte('\n')
	}
	return i, nil
}
func (b *presentationBuilder) parent(child, parent int) {
	b.links = append(b.links, document.Relationship{From: b.blocks[child].ID, To: b.blocks[parent].ID, Kind: "parent"})
}
func (b *presentationBuilder) loadRelations(part string) error {
	b.part = part
	name := path.Join(path.Dir(part), "_rels", path.Base(part)+".rels")
	if b.pkg.files[name] == nil {
		b.relations = map[string]officeRelation{}
		return nil
	}
	r, err := b.pkg.relations(part)
	b.relations = r
	return err
}
func (b *presentationBuilder) walk(n *presentationNode, notes bool) error {
	if n.name.Local == "AlternateContent" {
		b.warn("document_alternate_content", "Alternate presentation content is not interpreted; inspect the original.")
		_, err := b.add("figure", "[Alternate presentation content not interpreted]")
		return err
	}
	if n.presentation() {
		switch n.name.Local {
		case "sp":
			return b.shape(n, notes)
		case "pic":
			return b.figure(n, "image")
		case "graphicFrame":
			if table := n.descendant("tbl"); table != nil && table.drawing() {
				return b.table(table)
			}
			if chart := n.descendant("chart"); chart != nil {
				return b.figure(n, "chart")
			}
			return b.figure(n, "embedded_object")
		case "oleObj":
			return b.figure(n, "embedded_object")
		}
	}
	for _, c := range n.children {
		if err := b.walk(c, notes); err != nil {
			return err
		}
	}
	return nil
}
func (b *presentationBuilder) shape(n *presentationNode, notes bool) error {
	ph := n.descendant("ph")
	kind := "paragraph"
	if ph != nil {
		t := ph.attr("type")
		if notes && (t == "sldImg" || t == "sldNum" || t == "dt" || t == "hdr" || t == "ftr") {
			return nil
		}
		if !notes && (t == "title" || t == "ctrTitle") {
			kind = "heading"
		}
	}
	body := n.child("txBody")
	if body == nil {
		return nil
	}
	for _, p := range body.children {
		if p.name.Local != "p" || !p.drawing() {
			continue
		}
		text := presentationText(p)
		if text == "" {
			continue
		}
		if notes {
			text = "[Speaker note] " + text
		}
		at, err := b.add(kind, text)
		if err != nil {
			return err
		}
		if kind == "heading" {
			b.blocks[at].HeadingLevel = 1
		}
		if notes {
			b.blocks[at].ReviewState = "speaker_notes"
		}
	}
	return nil
}
func (b *presentationBuilder) figure(n *presentationNode, kind string) error {
	f := &document.Figure{Kind: kind}
	if metadata := n.descendant("cNvPr"); metadata != nil {
		f.Name = metadata.attr("name")
		f.Description = metadata.attr("descr")
		f.Title = metadata.attr("title")
	}
	var reference *presentationNode
	if kind == "image" {
		reference = n.descendant("blip")
	} else if kind == "chart" {
		reference = n.descendant("chart")
	} else {
		reference = n.descendant("oleObj")
	}
	if reference != nil {
		id := reference.relationship("embed")
		if id == "" {
			id = reference.relationship("link")
		}
		if id == "" {
			id = reference.relationship("id")
		}
		f.RelationshipID = id
		if relation, ok := b.relations[id]; ok {
			f.External = relation.Mode == "External"
			if !f.External {
				expected := kind
				if kind == "embedded_object" {
					expected = "oleObject"
				}
				part, err := officePart(b.part, id, expected, b.relations)
				if err != nil {
					return err
				}
				if b.pkg.files[part] == nil {
					return fmt.Errorf("missing presentation visual part")
				}
				f.Part = part
			}
		} else if id != "" {
			return fmt.Errorf("missing presentation visual relationship")
		}
	}
	at, err := b.add("figure", "["+kind+" not interpreted]")
	if err != nil {
		return err
	}
	b.blocks[at].Figure = f
	b.blocks[at].ReviewState = "unprocessed_visual"
	b.warn("document_visual_content", "Images, charts and embedded objects are identified but not interpreted. Authored descriptions are not verified visual evidence.")
	if f.Description != "" || f.Title != "" {
		caption, err := b.add("caption", "[Authored visual description] "+strings.TrimSpace(f.Title+"\n"+f.Description))
		if err != nil {
			return err
		}
		b.blocks[caption].ReviewState = "authored_description"
		b.links = append(b.links, document.Relationship{From: b.blocks[caption].ID, To: b.blocks[at].ID, Kind: "caption_for"})
	}
	return nil
}
