package rag

import (
	"encoding/xml"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type xlsxFormula struct {
	Text      string `xml:",chardata"`
	Type      string `xml:"t,attr"`
	Reference string `xml:"ref,attr"`
	Shared    *int   `xml:"si,attr"`
}
type xlsxCell struct {
	Ref       string       `xml:"r,attr"`
	Type      string       `xml:"t,attr"`
	Style     *int         `xml:"s,attr"`
	Value     *string      `xml:"v"`
	Formula   *xlsxFormula `xml:"f"`
	Inline    xlsxString   `xml:"is"`
	rowHidden bool
	rowStyle  *int
}
type xlsxSheetXML struct {
	Rows []struct {
		Number       int        `xml:"r,attr"`
		Hidden       bool       `xml:"hidden,attr"`
		Style        *int       `xml:"s,attr"`
		CustomFormat bool       `xml:"customFormat,attr"`
		Cells        []xlsxCell `xml:"c"`
	} `xml:"sheetData>row"`
	Columns []struct {
		Min    int  `xml:"min,attr"`
		Max    int  `xml:"max,attr"`
		Hidden bool `xml:"hidden,attr"`
		Style  *int `xml:"style,attr"`
	} `xml:"cols>col"`
	Merges []struct {
		Ref string `xml:"ref,attr"`
	} `xml:"mergeCells>mergeCell"`
	Tables []struct {
		ID string `xml:"id,attr"`
	} `xml:"tableParts>tablePart"`
	Conditional []struct{} `xml:"conditionalFormatting"`
}
type xlsxOptions struct {
	styles                                     xlsxStyles
	date1904, sheetHidden, recalculate, manual bool
	maxText                                    int64
}
type xlsxSheetResult struct {
	text     string
	cells    []SourceCell
	merges   []string
	tables   []string
	warnings []ExtractionWarning
}

func (s *xlsxSheetResult) warn(code, message string) {
	for _, w := range s.warnings {
		if w.Code == code {
			return
		}
	}
	s.warnings = append(s.warnings, ExtractionWarning{Code: code, Message: message})
}

func extractWorkbookSheet(data []byte, shared []xlsxString) (string, bool, []SourceCell, error) {
	x, err := parseXLSXSheet(data, shared, xlsxOptions{styles: defaultXLSXStyles(), maxText: 16 << 20})
	if err != nil {
		return "", false, nil, err
	}
	formulas := false
	for _, c := range x.cells {
		formulas = formulas || c.Data.FormulaType != ""
	}
	return x.text, formulas, x.cells, nil
}

func parseXLSXSheet(data []byte, shared []xlsxString, options xlsxOptions) (*xlsxSheetResult, error) {
	var sheet xlsxSheetXML
	if err := xml.Unmarshal(data, &sheet); err != nil {
		return nil, err
	}
	result := &xlsxSheetResult{}
	if options.sheetHidden {
		result.warn("document_hidden_content", "Hidden worksheet, row or column content is included and marked; inspect visibility before using it.")
	}
	if len(sheet.Merges) > 10000 || len(sheet.Tables) > 1000 || len(sheet.Columns) > 16384 {
		return nil, &ParseError{Code: "document_limit", Message: "Worksheet structure exceeds supported bounds."}
	}
	var cells []xlsxCell
	seen := map[string]bool{}
	for _, row := range sheet.Rows {
		for _, c := range row.Cells {
			if !validCellReference(c.Ref) || seen[c.Ref] {
				return nil, fmt.Errorf("invalid or duplicate workbook cell identity")
			}
			seen[c.Ref] = true
			_, r := xlsxCellPosition(c.Ref)
			if row.Number != 0 && row.Number != r {
				return nil, fmt.Errorf("worksheet row disagrees with cell reference")
			}
			if c.Value == nil && c.Formula == nil && c.Type != "inlineStr" {
				continue
			}
			if len(cells) >= 100000 {
				return nil, &ParseError{Code: "document_limit", Message: "This worksheet exceeds the 100,000 populated-cell limit."}
			}
			c.rowHidden = row.Hidden
			if row.CustomFormat {
				c.rowStyle = row.Style
			}
			cells = append(cells, c)
		}
	}
	sort.Slice(cells, func(i, j int) bool {
		ci, ri := xlsxCellPosition(cells[i].Ref)
		cj, rj := xlsxCellPosition(cells[j].Ref)
		if ri == rj {
			return ci < cj
		}
		return ri < rj
	})
	columnStyles := make([]*int, 16385)
	hiddenColumns := make([]bool, 16385)
	knownColumns := make([]bool, 16385)
	for _, col := range sheet.Columns {
		if col.Min < 1 || col.Max > 16384 || col.Min > col.Max {
			return nil, fmt.Errorf("invalid column range")
		}
		for n := col.Min; n <= col.Max; n++ {
			if knownColumns[n] {
				return nil, fmt.Errorf("overlapping column definitions")
			}
			knownColumns[n] = true
			columnStyles[n] = col.Style
			hiddenColumns[n] = col.Hidden
		}
	}
	var mergeAreas []xlsxArea
	for _, m := range sheet.Merges {
		area, ok := xlsxRange(m.Ref)
		if !ok {
			return nil, fmt.Errorf("invalid merged cell range")
		}
		mergeAreas = append(mergeAreas, area)
		result.merges = append(result.merges, m.Ref)
	}
	references := make([]string, len(cells))
	for i, cell := range cells {
		references[i] = cell.Ref
	}
	mergeOwners, err := xlsxAreaOwners(mergeAreas, references)
	if err != nil {
		return nil, err
	}
	for _, table := range sheet.Tables {
		result.tables = append(result.tables, table.ID)
	}
	anchors := map[int]xlsxCell{}
	for _, c := range cells {
		if c.Formula != nil && c.Formula.Type == "shared" && c.Formula.Shared != nil && c.Formula.Text != "" {
			id := *c.Formula.Shared
			if _, exists := anchors[id]; exists || id < 0 {
				return nil, fmt.Errorf("ambiguous shared formula")
			}
			anchors[id] = c
		}
	}
	var out strings.Builder
	for cellIndex, c := range cells {
		col, _ := xlsxCellPosition(c.Ref)
		style := 0
		for _, candidate := range []*int{columnStyles[col], c.rowStyle, c.Style} {
			if candidate != nil {
				style = *candidate
			}
		}
		formatID, format, err := options.styles.format(style)
		if err != nil {
			return nil, err
		}
		value, err := xlsxTypedCell(c, shared, formatID, format, options, anchors)
		if err != nil {
			return nil, err
		}
		if options.sheetHidden {
			value.HiddenReasons = append(value.HiddenReasons, "sheet")
		}
		if c.rowHidden {
			value.HiddenReasons = append(value.HiddenReasons, "row")
		}
		if hiddenColumns[col] {
			value.HiddenReasons = append(value.HiddenReasons, "column")
		}
		value.Hidden = len(value.HiddenReasons) > 0
		if owner := mergeOwners[cellIndex]; owner >= 0 {
			value.MergedRange = result.merges[owner]
		}
		if value.Hidden {
			result.warn("document_hidden_content", "Hidden worksheet, row or column content is included and marked; inspect visibility before using it.")
		}
		if value.FormulaType != "" {
			result.warn("document_formula_cache", "Formula results are stored caches that may be stale or missing. No formulas or external links were evaluated.")
		}
		if value.DisplayState == "unrendered" {
			result.warn("document_number_format", "Some number formats are not rendered. Raw values and format codes are retained; inspect the original display before interpreting units or dates.")
		}
		if value.DisplayState == "excel_1900_leap_day" {
			result.warn("document_excel_leap_day", "The workbook uses Excel's fictitious 1900-02-29 date. Its serial and displayed date are retained for review.")
		}
		start := out.Len()
		fmt.Fprintf(&out, "%s: %s", c.Ref, value.Display)
		if value.Display != value.Value && value.ValueType == "number" {
			fmt.Fprintf(&out, " [stored value: %s; format: %s]", value.Value, value.NumberFormat)
		}
		if value.DisplayState == "unrendered" {
			fmt.Fprintf(&out, " [format not rendered: %s]", value.NumberFormat)
		}
		if value.FormulaType != "" {
			formula := "=" + value.Formula
			if value.Formula == "" {
				formula = "shared or array formula; definition not expanded"
			}
			cache := "cached value; not recalculated"
			if value.CachedValue == nil {
				cache = "no cached value; not evaluated"
			}
			fmt.Fprintf(&out, " [formula: %s; %s]", formula, cache)
		}
		if value.Hidden {
			fmt.Fprintf(&out, " [hidden: %s]", strings.Join(value.HiddenReasons, ", "))
		}
		if value.MergedRange != "" {
			fmt.Fprintf(&out, " [merged: %s]", value.MergedRange)
		}
		result.cells = append(result.cells, SourceCell{Reference: c.Ref, Start: start, End: out.Len(), Data: value})
		out.WriteByte('\n')
		if int64(out.Len()) > options.maxText {
			return nil, &ParseError{Code: "document_limit", Message: "Extracted worksheet text exceeds the supported limit."}
		}
	}
	if len(sheet.Conditional) > 0 {
		result.warn("document_conditional_format", "Conditional formatting is retained only in the original workbook and is not evaluated.")
	}
	result.text = strings.TrimSpace(out.String())
	if len(result.cells) > 0 {
		last := &result.cells[len(result.cells)-1]
		last.End = min(last.End, len(result.text))
	}
	return result, nil
}

func xlsxCellPosition(ref string) (col, row int) {
	i := 0
	for i < len(ref) && ref[i] >= 'A' && ref[i] <= 'Z' {
		col = col*26 + int(ref[i]-'A') + 1
		i++
	}
	row, _ = strconv.Atoi(ref[i:])
	return
}
