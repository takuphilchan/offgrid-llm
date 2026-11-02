package rag

import (
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type xlsxString struct {
	Text string `xml:"t"`
	Runs []struct {
		Text string `xml:"t"`
	} `xml:"r"`
}

func (s xlsxString) value() string {
	var b strings.Builder
	b.WriteString(s.Text)
	for _, r := range s.Runs {
		b.WriteString(r.Text)
	}
	return b.String()
}

// ParseXLSX follows workbook relationships, not ZIP entry order. Formulas are
// reported with their stored cached values; they are never executed or refreshed.
func (p *DocumentParser) ParseXLSX(content []byte) (*ParseResult, error) {
	pkg, err := p.officePackage(content)
	if err != nil {
		return nil, err
	}
	var workbook struct {
		Sheets []struct {
			Name string `xml:"name,attr"`
			ID   string `xml:"id,attr"`
		} `xml:"sheets>sheet"`
	}
	if err := pkg.decode("xl/workbook.xml", &workbook); err != nil {
		return nil, err
	}
	relations, err := pkg.relations("xl/workbook.xml")
	if err != nil {
		return nil, err
	}
	var shared struct {
		Strings []xlsxString `xml:"si"`
	}
	for id, r := range relations {
		if strings.HasSuffix(r.Type, "/sharedStrings") {
			part, err := officePart("xl/workbook.xml", id, "sharedStrings", relations)
			if err != nil {
				return nil, err
			}
			if err := pkg.decode(part, &shared); err != nil {
				return nil, err
			}
		}
	}
	var out strings.Builder
	formulas := false
	var units []SourceUnit
	for i, sheet := range workbook.Sheets {
		part, err := officePart("xl/workbook.xml", sheet.ID, "worksheet", relations)
		if err != nil {
			return nil, err
		}
		data, err := pkg.read(part)
		if err != nil {
			return nil, err
		}
		text, hasFormula, cells, err := extractWorkbookSheet(data, shared.Strings)
		if err != nil {
			return nil, err
		}
		formulas = formulas || hasFormula
		state := "ready"
		if strings.TrimSpace(text) == "" {
			state = "empty"
		}
		units = append(units, SourceUnit{Kind: "sheet", Index: i + 1, Name: sheet.Name, Text: text, State: state, Cells: cells})
		if text != "" {
			fmt.Fprintf(&out, "--- Sheet: %s ---\n%s\n", sheet.Name, text)
			if int64(out.Len()) > p.maxExtractedSize {
				return nil, &ParseError{Code: "document_limit", Message: "Extracted workbook text exceeds the supported limit."}
			}
		}
	}
	text := strings.TrimSpace(out.String())
	if text == "" {
		return nil, fmt.Errorf("no readable workbook cells")
	}
	metadata := map[string]string{"format": "xlsx", "sheet_count": strconv.Itoa(len(workbook.Sheets)), "formulas": "not_evaluated", "limitations": "Stored cell text and raw values only; formatting, charts, images and macros are not interpreted."}
	if formulas {
		metadata["note"] = "Formula results are cached values from the workbook and may be stale; formulas were not evaluated."
	}
	result := &ParseResult{Content: text, ContentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", Metadata: metadata, PageCount: len(workbook.Sheets), WordCount: countWords(text), SourceUnits: units}
	bindSourceUnits(result, content)
	return result, nil
}

func extractWorkbookSheet(data []byte, shared []xlsxString) (string, bool, []SourceCell, error) {
	var sheet struct {
		Rows []struct {
			Cells []struct {
				Ref     string `xml:"r,attr"`
				Type    string `xml:"t,attr"`
				Value   string `xml:"v"`
				Formula *struct {
					Text string `xml:",chardata"`
					Type string `xml:"t,attr"`
				} `xml:"f"`
				Inline xlsxString `xml:"is"`
			} `xml:"c"`
		} `xml:"sheetData>row"`
	}
	if err := xml.Unmarshal(data, &sheet); err != nil {
		return "", false, nil, err
	}
	var out strings.Builder
	formulas := false
	var cells []SourceCell
	seen := map[string]bool{}
	for _, row := range sheet.Rows {
		for _, cell := range row.Cells {
			value := cell.Value
			switch cell.Type {
			case "inlineStr":
				value = cell.Inline.value()
			case "s":
				i, err := strconv.Atoi(value)
				if err != nil || i < 0 || i >= len(shared) {
					return "", false, nil, fmt.Errorf("invalid shared string reference")
				}
				value = shared[i].value()
			case "b":
				if value == "1" {
					value = "TRUE"
				} else if value == "0" {
					value = "FALSE"
				}
			}
			if value == "" && cell.Formula == nil {
				continue
			}
			if !validCellReference(cell.Ref) || seen[cell.Ref] {
				return "", false, nil, fmt.Errorf("invalid or duplicate workbook cell identity")
			}
			if len(cells) >= 100000 {
				return "", false, nil, &ParseError{Code: "document_limit", Message: "This worksheet exceeds the 100,000 populated-cell limit."}
			}
			seen[cell.Ref] = true
			start := out.Len()
			fmt.Fprintf(&out, "%s: %s", cell.Ref, value)
			if cell.Formula != nil {
				formulas = true
				formula := cell.Formula.Text
				if formula == "" {
					formula = "shared or array formula; definition not expanded"
				} else {
					formula = "=" + formula
				}
				cache := "cached value; not recalculated"
				if value == "" {
					cache = "no cached value; not evaluated"
				}
				fmt.Fprintf(&out, " [formula: %s; %s]", formula, cache)
			}
			cells = append(cells, SourceCell{Reference: cell.Ref, Start: start, End: out.Len()})
			out.WriteByte('\n')
		}
	}
	text := strings.TrimSpace(out.String())
	if len(cells) > 0 {
		cells[len(cells)-1].End = min(cells[len(cells)-1].End, len(text))
	}
	return text, formulas, cells, nil
}

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
	var out strings.Builder
	var units []SourceUnit
	for i, id := range ids {
		part, err := officePart("ppt/presentation.xml", id, "slide", relations)
		if err != nil {
			return nil, err
		}
		data, err := pkg.read(part)
		if err != nil {
			return nil, err
		}
		text, err := officeSlideText(data)
		if err != nil {
			return nil, err
		}
		if text != "" {
			fmt.Fprintf(&out, "--- Slide %d ---\n%s\n\n", i+1, text)
		}
		state := "ready"
		if text == "" {
			state = "empty"
		}
		units = append(units, SourceUnit{Kind: "slide", Index: i + 1, Text: text, State: state})
		if int64(out.Len()) > p.maxExtractedSize {
			return nil, &ParseError{Code: "document_limit", Message: "Extracted slide text exceeds the supported limit."}
		}
	}
	text := strings.TrimSpace(out.String())
	if text == "" {
		return nil, fmt.Errorf("no readable slide text")
	}
	result := &ParseResult{Content: text, ContentType: "application/vnd.openxmlformats-officedocument.presentationml.presentation", Metadata: map[string]string{"format": "pptx", "slide_count": strconv.Itoa(len(ids)), "limitations": "Slide text only; speaker notes, charts, images and embedded documents are not interpreted."}, PageCount: len(ids), WordCount: countWords(text), SourceUnits: units}
	bindSourceUnits(result, content)
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
		}
	}
}

func officeSlideText(data []byte) (string, error) {
	d := xml.NewDecoder(strings.NewReader(string(data)))
	var out strings.Builder
	for {
		token, err := d.Token()
		if err == io.EOF {
			return strings.TrimSpace(out.String()), nil
		}
		if err != nil {
			return "", err
		}
		switch t := token.(type) {
		case xml.StartElement:
			if t.Name.Local == "t" {
				var text string
				if err := d.DecodeElement(&text, &t); err != nil {
					return "", err
				}
				out.WriteString(text)
			}
		case xml.EndElement:
			if t.Name.Local == "p" {
				out.WriteByte('\n')
			}
		}
	}
}
