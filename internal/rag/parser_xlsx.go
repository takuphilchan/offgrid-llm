package rag

import (
	"fmt"
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

type xlsxWorkbook struct {
	Properties struct {
		Date1904 bool `xml:"date1904,attr"`
	} `xml:"workbookPr"`
	Calculation struct {
		Mode  string `xml:"calcMode,attr"`
		Full  bool   `xml:"fullCalcOnLoad,attr"`
		Force bool   `xml:"forceFullCalc,attr"`
	} `xml:"calcPr"`
	Sheets []struct {
		Name  string `xml:"name,attr"`
		ID    string `xml:"id,attr"`
		State string `xml:"state,attr"`
	} `xml:"sheets>sheet"`
}

// No spreadsheet engine is started. Native values and cached formula results
// remain distinct; formatting is a deterministic display projection only.
func (p *DocumentParser) ParseXLSX(content []byte) (*ParseResult, error) {
	pkg, err := p.officePackage(content)
	if err != nil {
		return nil, err
	}
	var workbook xlsxWorkbook
	if err = pkg.decode("xl/workbook.xml", &workbook); err != nil {
		return nil, err
	}
	if len(workbook.Sheets) == 0 || len(workbook.Sheets) > 1000 {
		return nil, fmt.Errorf("invalid workbook sheet count")
	}
	relations, err := pkg.relations("xl/workbook.xml")
	if err != nil {
		return nil, err
	}
	var shared struct {
		Strings []xlsxString `xml:"si"`
	}
	styles := defaultXLSXStyles()
	seenParts := map[string]bool{}
	for id, r := range relations {
		kind := ""
		if strings.HasSuffix(r.Type, "/sharedStrings") {
			kind = "sharedStrings"
		}
		if strings.HasSuffix(r.Type, "/styles") {
			kind = "styles"
		}
		if kind == "" {
			continue
		}
		if seenParts[kind] {
			return nil, fmt.Errorf("ambiguous workbook %s", kind)
		}
		seenParts[kind] = true
		part, e := officePart("xl/workbook.xml", id, kind, relations)
		if e != nil {
			return nil, e
		}
		if kind == "sharedStrings" {
			err = pkg.decode(part, &shared)
		} else {
			styles, err = readXLSXStyles(pkg, part)
		}
		if err != nil {
			return nil, err
		}
	}
	result := &ParseResult{ContentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", Metadata: map[string]string{"format": "xlsx", "sheet_count": strconv.Itoa(len(workbook.Sheets)), "formulas": "not_evaluated", "limitations": "Native cells, stored styles and explicit table headers; charts, images, conditional formatting and macros are not interpreted. Formula caches are not recalculated."}}
	var out strings.Builder
	for i, sheet := range workbook.Sheets {
		if sheet.State != "" && sheet.State != "visible" && sheet.State != "hidden" && sheet.State != "veryHidden" {
			return nil, fmt.Errorf("invalid worksheet visibility")
		}
		part, e := officePart("xl/workbook.xml", sheet.ID, "worksheet", relations)
		if e != nil {
			return nil, e
		}
		data, e := pkg.read(part)
		if e != nil {
			return nil, e
		}
		options := xlsxOptions{styles: styles, date1904: workbook.Properties.Date1904, sheetHidden: sheet.State == "hidden" || sheet.State == "veryHidden", recalculate: workbook.Calculation.Full || workbook.Calculation.Force, manual: workbook.Calculation.Mode == "manual", maxText: p.maxExtractedSize}
		x, e := parseXLSXSheet(data, shared.Strings, options)
		if e != nil {
			return nil, e
		}
		if e = bindXLSXTables(pkg, part, x); e != nil {
			return nil, e
		}
		state := "ready"
		if strings.TrimSpace(x.text) == "" {
			state = "empty"
		}
		result.SourceUnits = append(result.SourceUnits, SourceUnit{Kind: "sheet", Index: i + 1, Name: sheet.Name, Text: x.text, State: state, Cells: x.cells, Hidden: options.sheetHidden, MergedRanges: x.merges})
		for _, warning := range x.warnings {
			warning.SourceIndex = i + 1
			result.Warnings = append(result.Warnings, warning)
		}
		if x.text != "" {
			fmt.Fprintf(&out, "--- Sheet: %s ---\n%s\n", sheet.Name, x.text)
		}
		if int64(out.Len()) > p.maxExtractedSize {
			return nil, &ParseError{Code: "document_limit", Message: "Extracted workbook text exceeds the supported limit."}
		}
	}
	result.Content = strings.TrimSpace(out.String())
	if result.Content == "" {
		return nil, fmt.Errorf("no readable workbook cells")
	}
	result.PageCount = len(workbook.Sheets)
	result.WordCount = countWords(result.Content)
	result.Warnings = append(result.Warnings, ExtractionWarning{Code: "document_text_only", Message: result.Metadata["limitations"]})
	bindSourceUnits(result, content)
	return result, nil
}
