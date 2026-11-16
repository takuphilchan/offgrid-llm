package main

import (
	"encoding/binary"
	"math"
	"unicode/utf16"
)

// Native AcroForm values are read from PDFium, never from appearance OCR.
// No form events, actions, scripts, calculations or external resources run.
type formField struct {
	Label   string       `json:"label,omitempty"`
	Options []formOption `json:"options,omitempty"`
	Index   int          `json:"index"`
	Name    string       `json:"name"`
	Value   string       `json:"value"`
	Kind    int          `json:"kind"`
	Checked *bool        `json:"checked,omitempty"`
	Hidden  bool         `json:"hidden,omitempty"`
	Bounds  *[4]float64  `json:"bounds,omitempty"`
}

func (d *pdfDocument) formEnvironment() (uint64, func(), string) {
	e := d.engine
	// Pinned wasm32 ABI: a zeroed v1 FPDF_FORMFILLINFO. This non-V8,
	// non-XFA engine only uses getters; host callback pointers remain null.
	info, err := e.allocate(256)
	if err != nil {
		return 0, nil, "document_limit"
	}
	if !e.module.Memory().Write(uint32(info), make([]byte, 256)) || !e.module.Memory().WriteUint32Le(uint32(info), 1) {
		e.call("free", info)
		return 0, nil, "document_invalid"
	}
	handle, err := e.call("FPDFDOC_InitFormFillEnvironment", d.handle, info)
	if err != nil || handle == 0 {
		e.call("free", info)
		return 0, nil, "document_invalid"
	}
	return handle, func() { e.call("FPDFDOC_ExitFormFillEnvironment", handle); e.call("free", info) }, ""
}

func readFormFields(e *pdfEngine, page, form uint64) ([]formField, string) {
	count, err := e.call("FPDFPage_GetAnnotCount", page)
	if err != nil || int32(count) < 0 {
		return nil, "document_invalid"
	}
	if count > 4096 {
		return nil, "document_limit"
	}
	fields := []formField{}
	total := 0
	for i := uint64(0); i < count; i++ {
		annot, err := e.call("FPDFPage_GetAnnot", page, i)
		if err != nil || annot == 0 {
			return nil, "document_invalid"
		}
		field, widget, code := readFormField(e, annot, form, int(i))
		e.call("FPDFPage_CloseAnnot", annot)
		if code != "" {
			return nil, code
		}
		total += len(field.Name) + len(field.Label) + len(field.Value) + 64
		for _, option := range field.Options {
			total += len(option.Label) + 32
		}
		if total > maxText {
			return nil, "document_limit"
		}
		if widget {
			fields = append(fields, field)
		}
	}
	return fields, ""
}

func readFormField(e *pdfEngine, annot, form uint64, index int) (formField, bool, string) {
	f := formField{Index: index}
	subtype, err := e.call("FPDFAnnot_GetSubtype", annot)
	if err != nil {
		return f, false, "document_invalid"
	}
	if subtype != 20 {
		return f, false, ""
	} // FPDF_ANNOT_WIDGET
	kind, err := e.call("FPDFAnnot_GetFormFieldType", form, annot)
	if err != nil || int32(kind) < 0 || kind > 7 {
		return f, true, "document_invalid"
	}
	f.Kind = int(kind)
	var code string
	f.Name, code = fieldString(e, "FPDFAnnot_GetFormFieldName", form, annot)
	if code != "" {
		return f, true, code
	}
	f.Label, code = fieldString(e, "FPDFAnnot_GetFormFieldAlternateName", form, annot)
	if code != "" {
		return f, true, code
	}
	if kind == 4 || kind == 5 {
		f.Options, code = fieldOptions(e, form, annot)
		if code != "" {
			return f, true, code
		}
	}
	// Signatures are not textual values and this reader cannot authenticate them.
	if kind != 7 {
		f.Value, code = fieldString(e, "FPDFAnnot_GetFormFieldValue", form, annot)
		if code != "" {
			return f, true, code
		}
	}
	if kind == 2 || kind == 3 {
		checked, err := e.call("FPDFAnnot_IsChecked", form, annot)
		if err != nil {
			return f, true, "document_invalid"
		}
		value := checked != 0
		f.Checked = &value
	}
	flags, err := e.call("FPDFAnnot_GetFlags", annot)
	if err != nil || int32(flags) < 0 {
		return f, true, "document_invalid"
	}
	f.Hidden = flags&(1|2|32) != 0
	ptr, err := e.allocate(16)
	if err != nil {
		return f, true, "document_limit"
	}
	defer e.call("free", ptr)
	ok, err := e.call("FPDFAnnot_GetRect", annot, ptr)
	if err == nil && ok != 0 {
		raw, valid := e.module.Memory().Read(uint32(ptr), 16)
		if valid {
			box := [4]float64{}
			for i := range box {
				box[i] = float64(math.Float32frombits(binary.LittleEndian.Uint32(raw[i*4:])))
			}
			// PDF coordinates are retained as left,bottom,right,top, not guessed
			// screen coordinates. Invalid rectangles leave the value readable.
			valid = true
			for _, n := range box {
				valid = valid && !math.IsInf(n, 0) && !math.IsNaN(n)
			}
			if valid && box[0] < box[2] && box[1] != box[3] {
				box[1], box[3] = math.Min(box[1], box[3]), math.Max(box[1], box[3])
				f.Bounds = &box
			}
		}
	}
	return f, true, ""
}

func fieldString(e *pdfEngine, function string, form, annot uint64) (string, string) {
	return formString(e, function, []uint64{form, annot})
}

func formString(e *pdfEngine, function string, args []uint64) (string, string) {
	size, err := e.call(function, append(args, 0, 0)...)
	if err != nil {
		return "", "document_invalid"
	}
	if size == 0 {
		return "", ""
	}
	if size > 128<<10 || size%2 != 0 {
		return "", "document_limit"
	}
	ptr, err := e.allocate(size)
	if err != nil {
		return "", "document_limit"
	}
	defer e.call("free", ptr)
	written, err := e.call(function, append(args, ptr, size)...)
	if err != nil || written != size {
		return "", "document_invalid"
	}
	raw, ok := e.module.Memory().Read(uint32(ptr), uint32(size))
	if !ok || binary.LittleEndian.Uint16(raw[len(raw)-2:]) != 0 {
		return "", "document_invalid"
	}
	units := make([]uint16, len(raw)/2-1)
	for i := range units {
		units[i] = binary.LittleEndian.Uint16(raw[i*2:])
	}
	for i := 0; i < len(units); i++ {
		if units[i] >= 0xd800 && units[i] <= 0xdbff {
			if i+1 >= len(units) || units[i+1] < 0xdc00 || units[i+1] > 0xdfff {
				return "", "document_invalid"
			}
			i++
		} else if units[i] >= 0xdc00 && units[i] <= 0xdfff {
			return "", "document_invalid"
		}
	}
	return string(utf16.Decode(units)), ""
}
