package rag

import (
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

type wordRevisionCapture struct {
	depth  int
	offset int
	anchor string
	change document.RevisionChange
	text   strings.Builder
}

// Observe all XML tokens, including those excluded from the current reading.
// Captures are metadata only: deleted/moved-from text never enters Text.
func (s *wordStructure) revisionToken(token xml.Token, depth, offset int) error {
	switch t := token.(type) {
	case xml.StartElement:
		depth++
		if wordName(t.Name) {
			kind := ""
			switch t.Name.Local {
			case "ins":
				kind = "insertion"
			case "del":
				kind = "deletion"
			case "moveFrom":
				kind = "move_from"
			case "moveTo":
				kind = "move_to"
			case "pPrChange", "rPrChange", "sectPrChange", "tblPrChange", "trPrChange", "tcPrChange":
				kind = "properties"
			}
			if kind != "" {
				if len(s.revisions)+len(s.revisionStack) >= 2048 {
					return fmt.Errorf("Word revision count limit")
				}
				change := document.RevisionChange{ID: wordAttribute(t, "id"), Kind: kind, Author: wordAttribute(t, "author"), Date: wordAttribute(t, "date")}
				if len(change.ID) > 256 || len(change.Author) > 512 || len(change.Date) > 128 {
					return fmt.Errorf("Word revision metadata limit")
				}
				anchor := ""
				if len(s.stack) > 0 {
					anchor = s.blocks[s.stack[len(s.stack)-1].index].ID
				}
				s.revisionStack = append(s.revisionStack, &wordRevisionCapture{depth: depth, offset: offset, anchor: anchor, change: change})
			}
			if t.Name.Local == "t" || t.Name.Local == "delText" {
				s.revisionTextDepth = depth
			}
			if t.Name.Local == "tab" {
				return s.writeRevisionText("\t")
			}
			if t.Name.Local == "br" || t.Name.Local == "cr" {
				return s.writeRevisionText("\n")
			}
		}
	case xml.CharData:
		if s.revisionTextDepth > 0 {
			return s.writeRevisionText(string(t))
		}
	case xml.EndElement:
		if wordName(t.Name) && t.Name.Local == "p" {
			if err := s.writeRevisionText("\n"); err != nil {
				return err
			}
		}
		if depth == s.revisionTextDepth {
			s.revisionTextDepth = 0
		}
		if len(s.revisionStack) > 0 && s.revisionStack[len(s.revisionStack)-1].depth == depth {
			capture := s.revisionStack[len(s.revisionStack)-1]
			s.revisionStack = s.revisionStack[:len(s.revisionStack)-1]
			capture.change.Text = capture.text.String()
			s.revisions = append(s.revisions, capture)
		}
	}
	return nil
}

func (s *wordStructure) writeRevisionText(text string) error {
	for _, capture := range s.revisionStack {
		if capture.text.Len()+len(text) > 1<<20 || s.revisionBytes+len(text) > 4<<20 {
			return fmt.Errorf("Word revision text limit")
		}
		capture.text.WriteString(text)
		s.revisionBytes += len(text)
	}
	return nil
}

func (s *wordStructure) attachRevisions(blocks []document.Block, text string) []document.Block {
	indices := map[string]int{}
	for i, b := range blocks {
		indices[b.ID] = i
	}
	for i, capture := range s.revisions {
		index, ok := indices[capture.anchor]
		if !ok {
			// A fully deleted paragraph has no current-text span. Keep a zero-width
			// review block, rather than appending deleted text to the current version.
			offset := max(0, min(len(text), capture.offset-s.leadingTrim))
			id := fmt.Sprintf("word-revision-%d", i+1)
			blocks = append(blocks, document.Block{ID: id, UnitID: "section-1", Kind: "paragraph", Method: "native", Span: document.Span{Start: offset, End: offset}, SourcePart: "word/document.xml"})
			index = len(blocks) - 1
		}
		blocks[index].Revisions = append(blocks[index].Revisions, capture.change)
		blocks[index].ReviewState = "tracked_changes"
	}
	return blocks
}
