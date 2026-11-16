package document

import (
	"encoding/hex"
	"fmt"
	"math"
	"unicode/utf8"
)

func invalidExtraction(field string) error {
	return fmt.Errorf("invalid structured extraction: %s", field)
}

func validSpan(text string, s Span) bool {
	return s.Start >= 0 && s.End >= s.Start && s.End <= len(text) && (s.Start == len(text) || utf8.RuneStart(text[s.Start])) && (s.End == len(text) || utf8.RuneStart(text[s.End]))
}

func oneOf(value string, allowed ...string) bool {
	for _, v := range allowed {
		if value == v {
			return true
		}
	}
	return false
}

func (e *Extraction) validate() error {
	if e == nil || e.Schema != ExtractionSchema || len(e.Text) > 16<<20 || !utf8.ValidString(e.Text) || len(e.Units) == 0 || len(e.Units) > 1000 || len(e.Blocks) > 200000 || len(e.Relationships) > 400000 {
		return invalidExtraction("bounds")
	}
	if !oneOf(e.SourceKind, "original_file", "legacy_text") || !oneOf(e.Method, "native", "ocr", "mixed", "legacy", "review") {
		return invalidExtraction("provenance")
	}
	if e.SourceKind == "original_file" {
		if hash, err := hex.DecodeString(e.OriginalSHA256); err != nil || len(hash) != 32 {
			return invalidExtraction("original digest")
		}
	}
	if e.ParentID != "" && (e.Author == "" || e.CreatedAt == nil || e.CreatedAt.IsZero() || e.Method != "review") {
		return invalidExtraction("derived provenance")
	}
	if e.Method == "review" && e.ParentID == "" {
		return invalidExtraction("missing parent revision")
	}
	units := map[string]Unit{}
	for _, u := range e.Units {
		if _, exists := units[u.ID]; exists || u.ID == "" || u.Index < 1 || !validSpan(e.Text, u.Span) || !oneOf(u.Kind, "page", "sheet", "slide", "section") {
			return invalidExtraction("source unit")
		}
		units[u.ID] = u
	}
	blocks := map[string]Block{}
	revisionCount, revisionBytes := 0, 0
	for _, b := range e.Blocks {
		for _, change := range b.Revisions {
			revisionCount++
			revisionBytes += len(change.Text)
			if revisionCount > 2048 || revisionBytes > 4<<20 || len(change.Text) > 1<<20 || len(change.ID) > 256 || len(change.Author) > 512 || len(change.Date) > 128 || !utf8.ValidString(change.Text) || !utf8.ValidString(change.Author) || !oneOf(change.Kind, "insertion", "deletion", "move_from", "move_to", "properties") {
				return invalidExtraction("tracked revisions")
			}
		}
		u, ok := units[b.UnitID]
		if _, exists := blocks[b.ID]; exists || b.ID == "" || !ok || !validSpan(e.Text, b.Span) || b.Span.Start < u.Span.Start || b.Span.End > u.Span.End {
			return invalidExtraction("block span")
		}
		if !oneOf(b.Kind, "heading", "paragraph", "list", "table", "row", "cell", "footnote", "endnote", "form_field", "figure", "caption", "header", "footer") || !oneOf(b.Method, "native", "ocr", "mixed", "legacy", "review", "model") {
			return invalidExtraction("block kind/method")
		}
		if b.HeadingLevel < 0 || b.HeadingLevel > 9 || (b.HeadingLevel > 0 && b.Kind != "heading") || (b.Cell != nil && b.Kind != "cell") {
			return invalidExtraction("block metadata")
		}
		if (b.Figure != nil && b.Kind != "figure") || (b.Table != nil && b.Kind != "table") {
			return invalidExtraction("visual/table metadata")
		}
		if f := b.FormField; f != nil {
			if b.Kind != "form_field" || b.Method != "native" || len(f.Name) > 128<<10 || len(f.Value) > 128<<10 || !utf8.ValidString(f.Name) || !utf8.ValidString(f.Value) || !oneOf(f.Kind, "unknown", "button", "checkbox", "radio", "combo", "list", "text", "signature") || (f.Kind == "checkbox" || f.Kind == "radio") != (f.Checked != nil) {
				return invalidExtraction("form field")
			}
			if err := ValidateFormOptions(f.Label, f.Options, f.Kind == "combo" || f.Kind == "list"); err != nil {
				return err
			}
			if f.NativeBounds != nil {
				r := f.NativeBounds
				for _, n := range r {
					if math.IsNaN(n) || math.IsInf(n, 0) {
						return invalidExtraction("form coordinates")
					}
				}
				if r[0] >= r[2] || r[1] >= r[3] {
					return invalidExtraction("form coordinates")
				}
			}
		}
		if b.Bounds != nil {
			p := b.Bounds
			if !oneOf(p.Unit, "points", "pixels", "normalized") || p.X < 0 || p.Y < 0 || p.Width <= 0 || p.Height <= 0 {
				return invalidExtraction("coordinates")
			}
			for _, n := range []float64{p.X, p.Y, p.Width, p.Height} {
				if math.IsNaN(n) || math.IsInf(n, 0) {
					return invalidExtraction("coordinates")
				}
			}
			if p.Unit == "normalized" && (p.X+p.Width > 1 || p.Y+p.Height > 1) {
				return invalidExtraction("normalized coordinates")
			}
		}
		blocks[b.ID] = b
	}
	parents := map[string]string{}
	for _, r := range e.Relationships {
		from, ok := blocks[r.From]
		to, found := blocks[r.To]
		if !ok || !found || r.From == r.To || !oneOf(r.Kind, "parent", "heading_for", "header_for", "footnote_for", "caption_for", "notes_for", "continuation", "alternative") {
			return invalidExtraction("relationship")
		}
		if r.Kind == "parent" {
			if parents[r.From] != "" || from.UnitID != to.UnitID || from.Span.Start < to.Span.Start || from.Span.End > to.Span.End {
				return invalidExtraction("parent relationship")
			}
			parents[r.From] = r.To
		}
	}
	// Validate each edge once. Deep or adversarial trees must not produce a
	// quadratic validation walk or consume the goroutine stack.
	visited := map[string]bool{}
	for id := range parents {
		path := []string{}
		seen := map[string]bool{}
		for p := id; p != "" && !visited[p]; p = parents[p] {
			if seen[p] {
				return invalidExtraction("parent cycle")
			}
			seen[p] = true
			path = append(path, p)
		}
		for _, p := range path {
			visited[p] = true
		}
	}
	return nil
}
