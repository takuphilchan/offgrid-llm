package document

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// Correction applies only to exact leaf evidence. A displayed paragraph cannot
// overwrite a nested note/table silently. The parent remains immutable.
type Correction struct {
	BlockID        string `json:"block_id"`
	ExpectedSHA256 string `json:"expected_text_sha256"`
	Text           string `json:"text"`
}

type correctionSpan struct {
	Correction
	Span
}

func TextSHA256(text string) string {
	hash := sha256.Sum256([]byte(text))
	return hex.EncodeToString(hash[:])
}

func (e *Extraction) Correct(author string, when time.Time, edits []Correction) (*Extraction, error) {
	if err := e.Verify(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(author) == "" || len(author) > 256 || when.IsZero() || len(edits) == 0 || len(edits) > 256 {
		return nil, invalidExtraction("correction provenance/bounds")
	}
	blocks := map[string]Block{}
	for _, b := range e.Blocks {
		blocks[b.ID] = b
	}
	seen := map[string]bool{}
	changes := make([]correctionSpan, 0, len(edits))
	for _, edit := range edits {
		b, ok := blocks[edit.BlockID]
		if !ok || seen[b.ID] || b.Span.End <= b.Span.Start || !utf8.ValidString(edit.Text) || len(edit.Text) > 1<<20 || TextSHA256(e.Text[b.Span.Start:b.Span.End]) != edit.ExpectedSHA256 {
			return nil, invalidExtraction("stale correction target")
		}
		if !oneOf(b.Kind, "paragraph", "heading", "list", "cell", "footnote", "endnote", "form_field", "caption", "header", "footer") {
			return nil, invalidExtraction("non-text correction")
		}
		for _, other := range e.Blocks {
			if other.ID != b.ID && other.Span.Start < b.Span.End && other.Span.End > b.Span.Start && !(other.Span.Start <= b.Span.Start && other.Span.End >= b.Span.End) {
				return nil, invalidExtraction("correction crosses structured evidence")
			}
		}
		// Contained child evidence must be corrected independently, even if a
		// paragraph and its only note have identical byte ranges.
		for _, r := range e.Relationships {
			if r.To == b.ID && (r.Kind == "parent" || r.Kind == "footnote_for") {
				return nil, invalidExtraction("correction contains linked evidence")
			}
		}
		seen[b.ID] = true
		changes = append(changes, correctionSpan{Correction: edit, Span: b.Span})
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].Start < changes[j].Start })
	var out strings.Builder
	cursor := 0
	for _, change := range changes {
		if change.Start < cursor {
			return nil, invalidExtraction("overlapping corrections")
		}
		out.WriteString(e.Text[cursor:change.Start])
		out.WriteString(change.Text)
		cursor = change.End
		if out.Len() > 16<<20 {
			return nil, invalidExtraction("corrected text limit")
		}
	}
	out.WriteString(e.Text[cursor:])
	if out.Len() > 16<<20 {
		return nil, invalidExtraction("corrected text limit")
	}
	derived := *e
	derived.ID = ""
	derived.ParentID = e.ID
	derived.Method = "review"
	derived.Author = author
	stamp := when.UTC()
	derived.CreatedAt = &stamp
	derived.Text = out.String()
	derived.Units = append([]Unit(nil), e.Units...)
	derived.Blocks = append([]Block(nil), e.Blocks...)
	move := func(offset int) int {
		delta := 0
		for _, c := range changes {
			if offset >= c.End {
				delta += len(c.Text) - (c.End - c.Start)
			} else {
				break
			}
		}
		return offset + delta
	}
	for i, u := range derived.Units {
		derived.Units[i].Span = Span{Start: move(u.Span.Start), End: move(u.Span.End)}
	}
	for i, b := range derived.Blocks {
		derived.Blocks[i].Span = Span{Start: move(b.Span.Start), End: move(b.Span.End)}
		affected := false
		for _, change := range changes {
			if change.Start < b.Span.End && change.End > b.Span.Start {
				affected = true
				break
			}
		}
		if affected {
			derived.Blocks[i].FormField = nil
		}
		if seen[b.ID] {
			derived.Blocks[i].Method = "review"
			derived.Blocks[i].ReviewState = "corrected"
		} else if affected {
			derived.Blocks[i].Method = "mixed"
			derived.Blocks[i].ReviewState = "contains_corrections"
		}
		if affected && b.Cell != nil {
			// A reviewer changed displayed text, not a recalculated formula or
			// a type-checked numeric value. Preserve those on the parent only.
			derived.Blocks[i].Cell = &Cell{Reference: b.Cell.Reference, Display: derived.Text[derived.Blocks[i].Span.Start:derived.Blocks[i].Span.End], ResultState: "reviewed_text", DisplayState: "reviewed_text", Hidden: b.Cell.Hidden, HiddenReasons: append([]string(nil), b.Cell.HiddenReasons...), MergedRange: b.Cell.MergedRange, RowSpan: b.Cell.RowSpan, ColumnSpan: b.Cell.ColumnSpan, HorizontalMerge: b.Cell.HorizontalMerge, VerticalMerge: b.Cell.VerticalMerge}
		}
	}
	if err := derived.Seal(); err != nil {
		return nil, err
	}
	return &derived, nil
}
