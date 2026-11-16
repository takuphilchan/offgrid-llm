package rag

import (
	"fmt"
	"math"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

// Fields carry native values independently of appearance text and OCR. Spans
// are assigned by the parent, never trusted from a worker receipt.
type PDFFormField struct {
	Label   string                `json:"label,omitempty"`
	Options []document.FormOption `json:"options,omitempty"`
	Index   int                   `json:"index"`
	Name    string                `json:"name"`
	Value   string                `json:"value"`
	Kind    int                   `json:"kind"`
	Checked *bool                 `json:"checked,omitempty"`
	Hidden  bool                  `json:"hidden,omitempty"`
	Bounds  *[4]float64           `json:"bounds,omitempty"`
	Start   int                   `json:"start_byte,omitempty"`
	End     int                   `json:"end_byte,omitempty"`
}

func pdfPageNativeText(unit SourceUnit) string {
	if len(unit.FormFields) == 0 {
		return unit.Text
	}
	start := unit.FormFields[0].Start
	if start < 0 || start > len(unit.Text) {
		return unit.Text
	}
	return strings.TrimSuffix(unit.Text[:start], "\n")
}

// Partial extraction receipts can survive a restart. Validate the parent-owned
// offsets and rendered field text before any stored offset is used again.
func validatePDFFieldSpans(unit SourceUnit) error {
	if len(unit.FormFields) == 0 {
		return nil
	}
	start := unit.FormFields[0].Start
	if start < 0 || start > len(unit.Text) {
		return fmt.Errorf("invalid retained form span")
	}
	copy := unit
	copy.Text = pdfPageNativeText(unit)
	if err := appendPDFFields(&copy, unit.FormFields); err != nil {
		return err
	}
	if copy.Text != unit.Text {
		return fmt.Errorf("retained form text changed")
	}
	for i, f := range copy.FormFields {
		if f.Start != unit.FormFields[i].Start || f.End != unit.FormFields[i].End {
			return fmt.Errorf("retained form position changed")
		}
	}
	return nil
}

func appendPDFFields(unit *SourceUnit, fields []PDFFormField) error {
	if len(fields) > 4096 {
		return fmt.Errorf("PDF form limit")
	}
	text := strings.Builder{}
	text.WriteString(unit.Text)
	unit.FormFields = nil
	last := -1
	for _, field := range fields {
		if field.Index <= last || field.Index >= 4096 || field.Kind < 0 || field.Kind > 7 || len(field.Name) > 128<<10 || len(field.Value) > 128<<10 || !utf8.ValidString(field.Name) || !utf8.ValidString(field.Value) {
			return fmt.Errorf("invalid PDF field")
		}
		last = field.Index
		if err := document.ValidateFormOptions(field.Label, field.Options, field.Kind == 4 || field.Kind == 5); err != nil {
			return err
		}
		if (field.Kind == 2 || field.Kind == 3) != (field.Checked != nil) {
			return fmt.Errorf("invalid PDF checkbox state")
		}
		if field.Bounds != nil {
			b := field.Bounds
			for _, n := range b {
				if math.IsNaN(n) || math.IsInf(n, 0) {
					return fmt.Errorf("invalid PDF field rectangle")
				}
			}
			if b[0] >= b[2] || b[1] >= b[3] {
				return fmt.Errorf("invalid PDF field rectangle")
			}
		}
		if text.Len() > 0 {
			text.WriteByte('\n')
		}
		field.Start = text.Len()
		// JSON quoting is unnecessary for plain evidence, but labels and states
		// remain separate structured attributes so prose never defines authority.
		name := field.Name
		if name == "" {
			name = fmt.Sprintf("[field #%d]", field.Index+1)
		}
		text.WriteString(name)
		if field.Label != "" && field.Label != name {
			text.WriteString(" (" + field.Label + ")")
		}
		text.WriteString(": ")
		if field.Checked != nil {
			if *field.Checked {
				text.WriteString("[x] ")
			} else {
				text.WriteString("[ ] ")
			}
		}
		if field.Kind == 7 {
			text.WriteString("[signature: unverified]")
		} else {
			text.WriteString(field.Value)
		}
		for _, option := range field.Options {
			if option.Selected {
				text.WriteString(" [x] " + option.Label)
			}
		}
		if field.Hidden {
			text.WriteString(" [hidden]")
		}
		field.End = text.Len()
		unit.FormFields = append(unit.FormFields, field)
		if text.Len() > 16<<20 {
			return fmt.Errorf("PDF field text limit")
		}
	}
	unit.Text = text.String()
	if unit.State == "empty" && len(fields) > 0 {
		unit.State = "ready"
	}
	return nil
}

func (r *ParseResult) enrichPDFFields() error {
	if r.Metadata["format"] != "pdf" {
		return nil
	}
	units := map[string]document.Unit{}
	for _, u := range r.Structure.Units {
		units[u.ID] = u
	}
	for _, source := range r.SourceUnits {
		if len(source.FormFields) == 0 {
			continue
		}
		if err := validatePDFFieldSpans(source); err != nil {
			return err
		}
		unit := units[fmt.Sprintf("page-%d", source.Index)]
		// parsedSourceRanges trims each source unit; retain exact byte positions.
		trim := len(source.Text) - len(strings.TrimLeftFunc(source.Text, unicode.IsSpace))
		start := unit.Span.Start + source.FormFields[0].Start - trim
		kept := r.Structure.Blocks[:0]
		for _, b := range r.Structure.Blocks {
			if b.UnitID != unit.ID || b.Span.End <= start {
				kept = append(kept, b)
			}
		}
		r.Structure.Blocks = kept
		for _, field := range source.FormFields {
			if field.Kind < 0 || field.Kind > 7 {
				return fmt.Errorf("invalid retained PDF field kind")
			}
			a, b := max(unit.Span.Start, unit.Span.Start+field.Start-trim), unit.Span.Start+field.End-trim
			// Only trailing whitespace can disappear during the source projection.
			b = min(b, unit.Span.End)
			if a < unit.Span.Start || b <= a || b > len(r.Structure.Text) {
				return fmt.Errorf("invalid retained PDF field span")
			}
			kinds := []string{"unknown", "button", "checkbox", "radio", "combo", "list", "text", "signature"}
			value := &document.FormField{Label: field.Label, Options: field.Options, Name: field.Name, Value: field.Value, Kind: kinds[field.Kind], Checked: field.Checked, Hidden: field.Hidden, NativeBounds: field.Bounds}
			r.Structure.Blocks = append(r.Structure.Blocks, document.Block{ID: fmt.Sprintf("%s-field-%d", unit.ID, field.Index), UnitID: unit.ID, Kind: "form_field", Method: "native", Span: document.Span{Start: a, End: b}, FormField: value, ReviewState: "needs_review", SourcePart: fmt.Sprintf("page/%d/annotation/%d", source.Index, field.Index)})
		}
	}
	return nil
}
