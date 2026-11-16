package rag

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

type wordFrame struct{ depth, index int }
type wordStructure struct {
	noteRevisions                                 map[string][]document.RevisionChange
	revisions                                     []*wordRevisionCapture
	revisionStack                                 []*wordRevisionCapture
	revisionTextDepth, revisionBytes, leadingTrim int
	styles                                        map[string]int
	explicitOutline                               map[int]bool
	blocks                                        []document.Block
	relations                                     []document.Relationship
	stack                                         []wordFrame
	textStart, textEnd                            int
	headerRows                                    map[string]bool
}

func (s *wordStructure) start(t xml.StartElement, depth, offset int) {
	kind := ""
	switch t.Name.Local {
	case "p":
		kind = "paragraph"
	case "tbl":
		kind = "table"
	case "tr":
		kind = "row"
	case "tc":
		kind = "cell"
	case "t":
		s.textStart = offset
	}
	if kind != "" {
		b := document.Block{ID: fmt.Sprintf("word-block-%d", len(s.blocks)+1), UnitID: "section-1", Kind: kind, Method: "native", Span: document.Span{Start: offset}, SourcePart: "word/document.xml"}
		if kind == "paragraph" && s.styles[""] > 0 {
			b.Kind = "heading"
			b.HeadingLevel = s.styles[""]
		}
		if len(s.stack) > 0 {
			parent := s.blocks[s.stack[len(s.stack)-1].index]
			s.relations = append(s.relations, document.Relationship{From: b.ID, To: parent.ID, Kind: "parent"})
		}
		s.stack = append(s.stack, wordFrame{depth: depth, index: len(s.blocks)})
		s.blocks = append(s.blocks, b)
	}
	if len(s.stack) == 0 {
		return
	}
	current := &s.blocks[s.stack[len(s.stack)-1].index]
	if t.Name.Local == "tblHeader" && current.Kind == "row" {
		enabled := true
		for _, a := range t.Attr {
			if wordName(a.Name) && a.Name.Local == "val" {
				enabled = a.Value == "true" || a.Value == "1" || a.Value == "on"
			}
		}
		if s.headerRows == nil {
			s.headerRows = map[string]bool{}
		}
		s.headerRows[current.ID] = enabled
	}
	if t.Name.Local == "pStyle" && depth == s.stack[len(s.stack)-1].depth+2 && (current.Kind == "paragraph" || current.Kind == "heading") && !s.explicitOutline[s.stack[len(s.stack)-1].index] {
		current.Kind = "paragraph"
		current.HeadingLevel = 0
		level, exists := s.styles[wordAttribute(t, "val")]
		if !exists {
			level = s.styles[""]
		}
		if level > 0 {
			current.Kind = "heading"
			current.HeadingLevel = level
		}
	}
	if depth == s.stack[len(s.stack)-1].depth+2 && (current.Kind == "paragraph" || current.Kind == "heading" || current.Kind == "list") && (t.Name.Local == "outlineLvl" || t.Name.Local == "numPr") {
		if t.Name.Local == "numPr" {
			if current.Kind != "heading" {
				current.Kind = "list"
			}
			return
		}
		for _, a := range t.Attr {
			if wordName(a.Name) && a.Name.Local == "val" {
				if level, err := strconv.Atoi(a.Value); err == nil && level >= 0 && level <= 9 {
					if s.explicitOutline == nil {
						s.explicitOutline = map[int]bool{}
					}
					s.explicitOutline[s.stack[len(s.stack)-1].index] = true
					current.Kind = "heading"
					current.HeadingLevel = level + 1
					if level == 9 {
						current.Kind = "paragraph"
						current.HeadingLevel = 0
					}
				}
			}
		}
	}
	if t.Name.Local == "footnoteReference" || t.Name.Local == "endnoteReference" {
		kind := strings.TrimSuffix(t.Name.Local, "Reference")
		// docxNotes inserts the resolved note as the immediately preceding text
		// run. Positions come from the text writer, not a search for repeated prose.
		if s.textEnd != offset || s.textStart >= s.textEnd {
			return
		}
		b := document.Block{ID: fmt.Sprintf("word-block-%d", len(s.blocks)+1), UnitID: "section-1", Kind: kind, Method: "native", Span: document.Span{Start: s.textStart, End: s.textEnd}, SourcePart: "word/" + kind + "s.xml"}
		b.Revisions = s.noteRevisions[kind+":"+wordAttribute(t, "id")]
		if len(b.Revisions) > 0 {
			b.ReviewState = "tracked_changes"
		}
		s.relations = append(s.relations, document.Relationship{From: b.ID, To: current.ID, Kind: "footnote_for"})
		s.blocks = append(s.blocks, b)
	}
}

func (s *wordStructure) end(t xml.EndElement, depth, offset int) {
	if t.Name.Local == "t" {
		s.textEnd = offset
	}
	if len(s.stack) > 0 && s.stack[len(s.stack)-1].depth == depth {
		frame := s.stack[len(s.stack)-1]
		s.stack = s.stack[:len(s.stack)-1]
		s.blocks[frame.index].Span.End = offset
	}
}

func (s *wordStructure) finish(raw string, r *ParseResult) error {
	if err := r.bindStructure(); err != nil {
		return err
	}
	trim := len(raw) - len(strings.TrimLeftFunc(raw, unicode.IsSpace))
	blocks := make([]document.Block, 0, len(s.blocks))
	kept := map[string]bool{}
	for _, b := range s.blocks {
		b.Span.Start = max(0, b.Span.Start-trim)
		b.Span.End = min(len(r.Content), b.Span.End-trim)
		if b.Span.End <= b.Span.Start {
			continue
		}
		kept[b.ID] = true
		blocks = append(blocks, b)
	}
	rels := []document.Relationship{}
	for _, rel := range s.relations {
		if kept[rel.From] && kept[rel.To] {
			rels = append(rels, rel)
		}
	}
	// Explicit outline levels establish ancestry; paragraph order alone cannot
	// invent a heading or a table-header association.
	headings := []document.Block{}
	for _, b := range blocks {
		if b.Kind == "heading" {
			for len(headings) > 0 && headings[len(headings)-1].HeadingLevel >= b.HeadingLevel {
				headings = headings[:len(headings)-1]
			}
		}
		if len(headings) > 0 {
			rels = append(rels, document.Relationship{From: headings[len(headings)-1].ID, To: b.ID, Kind: "heading_for"})
		}
		if b.Kind == "heading" {
			headings = append(headings, b)
		}
	}
	blocks = s.attachRevisions(blocks, r.Content)
	r.Structure.Blocks = blocks
	// Only explicit repeating-header rows establish table-wide header context.
	// Keep nested tables isolated through their direct parent relationships.
	parents := map[string]string{}
	for _, rel := range rels {
		if rel.Kind == "parent" {
			parents[rel.From] = rel.To
		}
	}
	headers := map[string][]string{}
	for _, b := range blocks {
		if b.Kind == "row" && s.headerRows[b.ID] {
			headers[parents[b.ID]] = append(headers[parents[b.ID]], b.ID)
		}
	}
	for _, b := range blocks {
		if b.Kind == "row" && !s.headerRows[b.ID] {
			for _, header := range headers[parents[b.ID]] {
				if len(rels) >= 400000 {
					return &ParseError{Code: "document_limit", Message: "Document table relationships exceed the supported limit."}
				}
				rels = append(rels, document.Relationship{From: header, To: b.ID, Kind: "header_for"})
			}
		}
	}
	r.Structure.Relationships = rels
	return r.Structure.Seal()
}
