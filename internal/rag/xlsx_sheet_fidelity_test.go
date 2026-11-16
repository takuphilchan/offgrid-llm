package rag

import (
	"strings"
	"testing"
)

func TestXLSXCellCacheTypesAndStylePrecedence(t *testing.T) {
	styles := defaultXLSXStyles()
	styles.cells = []int{0, 2, 10}
	xml := `<worksheet><cols><col min="1" max="4" style="1" hidden="1"/></cols><sheetData>
<row r="1" s="2" customFormat="1"><c r="A1"><v>0.125</v></c><c r="B1" s="1"><v>0.125</v></c><c r="C1" t="b"><v>0</v></c><c r="D1" t="str"><f>IF(1, &quot;&quot;)</f><v></v></c></row>
<row r="2"><c r="A2"><f>1-1</f><v>0</v></c><c r="B2"><f>1-1</f></c><c r="C2" t="e"><f>1/0</f><v>#DIV/0!</v></c><c r="D2"><v>12345678901234567890.25</v></c></row></sheetData></worksheet>`
	r, err := parseXLSXSheet([]byte(xml), nil, xlsxOptions{styles: styles, manual: true, maxText: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	if r.cells[0].Data.Display != "12.50%" || r.cells[1].Data.Display != "0.13" || r.cells[2].Data.Display != "FALSE" {
		t.Fatal("lost style precedence or boolean type")
	}
	if cache := r.cells[3].Data.CachedValue; cache == nil || *cache != "" {
		t.Fatal("cached empty text became absent")
	}
	if cache := r.cells[4].Data.CachedValue; cache == nil || *cache != "0" || r.cells[4].Data.ResultState != "cached_manual_calculation" {
		t.Fatal("cached zero changed")
	}
	if r.cells[5].Data.CachedValue != nil || r.cells[5].Data.ResultState != "missing_cache" {
		t.Fatal("missing cache became zero")
	}
	if r.cells[6].Data.ResultState != "cached_error_unverified" {
		t.Fatal("error result lost")
	}
	if r.cells[7].Data.Value != "12345678901234567890.25" || r.cells[7].Data.Display != r.cells[7].Data.Value {
		t.Fatal("native precision lost")
	}
}

func TestXLSXMalformedStructuresAndUninterpretedDisplays(t *testing.T) {
	for _, xml := range []string{
		`<worksheet><mergeCells><mergeCell ref="A10:C12"/><mergeCell ref="B11:D13"/></mergeCells></worksheet>`,
		`<worksheet><mergeCells><mergeCell ref="XFE1:XFE2"/></mergeCells></worksheet>`,
		`<worksheet><cols><col min="0" max="1"/></cols></worksheet>`,
		`<worksheet><cols><col min="1" max="2"/><col min="2" max="3"/></cols></worksheet>`,
		`<worksheet><sheetData><row r="2"><c r="A1"><v>1</v></c></row></sheetData></worksheet>`,
		`<worksheet><sheetData><row><c r="A1"><v>1</v></c><c r="A1"><v>2</v></c></row></sheetData></worksheet>`,
		`<worksheet><sheetData><row><c r="A1" s="99"><v>1</v></c></row></sheetData></worksheet>`,
	} {
		if _, err := parseXLSXSheet([]byte(xml), nil, xlsxOptions{styles: defaultXLSXStyles(), maxText: 1 << 20}); err == nil {
			t.Fatal("ambiguous structure admitted", xml)
		}
	}
	styles := defaultXLSXStyles()
	styles.cells = []int{99}
	r, err := parseXLSXSheet([]byte(`<worksheet><sheetData><row><c r="A1"><v>45292</v></c></row></sheetData><conditionalFormatting/></worksheet>`), nil, xlsxOptions{styles: styles, sheetHidden: true, maxText: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	if r.cells[0].Data.DisplayState != "unrendered" || r.cells[0].Data.Value != "45292" || !strings.Contains(r.text, "unresolved-numFmtId:99") {
		t.Fatal("unknown locale format was guessed")
	}
	if len(r.warnings) != 3 {
		t.Fatal("hidden/format/conditional warnings missing", r.warnings)
	}
	blank, err := parseXLSXSheet([]byte(`<worksheet/>`), nil, xlsxOptions{styles: defaultXLSXStyles(), sheetHidden: true, maxText: 1 << 20})
	if err != nil || len(blank.warnings) != 1 {
		t.Fatal("hidden empty worksheet not disclosed", err)
	}
}
