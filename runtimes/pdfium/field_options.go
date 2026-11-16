package main

type formOption struct {
	Index    int    `json:"index"`
	Label    string `json:"label"`
	Selected bool   `json:"selected"`
}

func fieldOptions(e *pdfEngine, form, annot uint64) ([]formOption, string) {
	count, err := e.call("FPDFAnnot_GetOptionCount", form, annot)
	if err != nil {
		return nil, "document_invalid"
	}
	// An editable combo box may have a value without an option dictionary.
	if int32(count) == -1 {
		return nil, ""
	}
	if count > 4096 {
		return nil, "document_limit"
	}
	var options []formOption
	total := 0
	for i := uint64(0); i < count; i++ {
		text, code := formString(e, "FPDFAnnot_GetOptionLabel", []uint64{form, annot, i})
		if code != "" {
			return nil, code
		}
		total += len(text) + 32
		if total > 128<<10 {
			return nil, "document_limit"
		}
		selected, err := e.call("FPDFAnnot_IsOptionSelected", form, annot, i)
		if err != nil {
			return nil, "document_invalid"
		}
		options = append(options, formOption{Index: int(i), Label: text, Selected: selected != 0})
	}
	return options, ""
}
