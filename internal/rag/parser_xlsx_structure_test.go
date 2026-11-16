package rag

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

func fidelityWorkbook(t *testing.T) []byte {
	t.Helper()
	return parserZip(t, map[string]string{
		"xl/workbook.xml":            `<workbook xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><workbookPr date1904="0"/><sheets><sheet name="Budget" r:id="sheet"/><sheet name="Hidden blank" r:id="blank" state="veryHidden"/></sheets><calcPr fullCalcOnLoad="1"/></workbook>`,
		"xl/_rels/workbook.xml.rels": `<Relationships><Relationship Id="sheet" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/><Relationship Id="blank" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet2.xml"/><Relationship Id="styles" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/></Relationships>`,
		"xl/styles.xml":              `<styleSheet><numFmts><numFmt numFmtId="164" formatCode="&quot;USD &quot;#,##0.00"/><numFmt numFmtId="165" formatCode="yyyy-mm-dd"/><numFmt numFmtId="166" formatCode="0.0%"/></numFmts><cellXfs><xf numFmtId="0"/><xf numFmtId="164"/><xf numFmtId="165"/><xf numFmtId="166"/></cellXfs></styleSheet>`,
		"xl/worksheets/sheet1.xml": `<worksheet xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><cols><col min="5" max="5" hidden="1"/></cols><sheetData>
<row r="1"><c r="A1" t="inlineStr"><is><t>Category</t></is></c><c r="B1" t="inlineStr"><is><t>Amount (USD)</t></is></c><c r="C1" t="inlineStr"><is><t>Effective date</t></is></c><c r="D1" t="inlineStr"><is><t>Rate</t></is></c><c r="E1" t="inlineStr"><is><t>Stored total</t></is></c></row>
<row r="2"><c r="A2" t="inlineStr"><is><t>Clinic</t></is></c><c r="B2" s="1"><v>1234.567</v></c><c r="C2" s="2"><v>45292</v></c><c r="D2" s="3"><v>0.125</v></c><c r="E2"><f>SUM(B2:B3)</f><v>9.25</v></c></row>
<row r="3" hidden="1"><c r="A3" t="inlineStr"><is><t>Uncached</t></is></c><c r="B3" s="1"><f>B2*2</f></c><c r="C3"><f t="shared" si="7" ref="C3:C4">B2*2</f><v>2</v></c></row>
<row r="4"><c r="C4"><f t="shared" si="7"/><v>4</v></c></row>
<row r="5"><c r="A5" t="inlineStr"><is><t>Merged title</t></is></c></row></sheetData><mergeCells><mergeCell ref="A5:B5"/></mergeCells><tableParts><tablePart r:id="table"/></tableParts></worksheet>`,
		"xl/worksheets/sheet2.xml":            `<worksheet><sheetData/><mergeCells><mergeCell ref="A1:C1"/></mergeCells></worksheet>`,
		"xl/worksheets/_rels/sheet1.xml.rels": `<Relationships><Relationship Id="table" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/table" Target="../tables/table1.xml"/></Relationships>`,
		"xl/tables/table1.xml":                `<table ref="A1:E4" headerRowCount="1"><tableColumns><tableColumn name="Category"/><tableColumn name="Amount (USD)"/><tableColumn name="Effective date"/><tableColumn name="Rate"/><tableColumn name="Stored total"/></tableColumns></table>`,
	})
}

func TestXLSXStoredRevisionCorrectionAndSourceReadback(t *testing.T) {
	parsed, err := NewDocumentParser().Parse(fidelityWorkbook(t), "budget.xlsx", ".xlsx")
	if err != nil {
		t.Fatal(err)
	}
	text, ranges, err := parsedSourceRanges(parsed)
	if err != nil {
		t.Fatal(err)
	}
	d := &Document{ID: "budget", Name: "Budget", RawContent: text, Metadata: parsed.Metadata, SourceRanges: ranges, Extraction: parsed.Structure, SourceRetained: true, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	chunks, err := NewChunker(DefaultChunkingOptions()).chunkDocument(d)
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	s, err := NewSQLiteStore(directory)
	if err != nil {
		t.Fatal(err)
	}
	vectors := make([][]float32, len(chunks))
	for i := range vectors {
		vectors[i] = []float32{1, 0}
	}
	if err = s.AddDocumentWithChunks(d, chunks, vectors); err != nil {
		s.Close()
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = NewSQLiteStore(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	graph, err := s.extraction(d.ID, d.ExtractionRevision)
	if err != nil || graph.Verify() != nil {
		t.Fatal("lost persisted graph", err)
	}
	var cell document.Block
	for _, b := range graph.Blocks {
		if b.Cell != nil && b.Cell.Reference == "E2" {
			cell = b
		}
	}
	if cell.Cell == nil || cell.Cell.CachedValue == nil || !cell.Cell.Hidden {
		t.Fatal("cache/visibility lost after restart")
	}
	before := graph.Text[cell.Span.Start:cell.Span.End]
	derived, err := graph.Correct("reviewer", time.Now(), []document.Correction{{BlockID: cell.ID, ExpectedSHA256: document.TextSHA256(before), Text: "E2: reviewer transcription"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range derived.Blocks {
		if b.ID == cell.ID {
			if b.Cell.CachedValue != nil || b.Cell.Formula != "" || b.Cell.Value != "" || b.Cell.ResultState != "reviewed_text" || !b.Cell.Hidden || len(b.Cell.HiddenReasons) != 1 {
				t.Fatal("correction invents recalculation or erases visibility", b.Cell)
			}
		}
	}
	if graph.Verify() != nil || *cell.Cell.CachedValue != "9.25" {
		t.Fatal("parent revision mutated")
	}
	for _, c := range chunks {
		read, err := d.ReadSourcePassage(c.StartChar, c.EndChar, c.Source.SHA256, c.Source.Revision, c.ContentHash)
		if err != nil || read != c.Content {
			t.Fatal("XLSX citation mismatch", err)
		}
	}
}

func TestXLSXStructuredValuesHeadersAndHiddenEvidence(t *testing.T) {
	parsed, err := NewDocumentParser().Parse(fidelityWorkbook(t), "budget.xlsx", ".xlsx")
	if err != nil {
		t.Fatal(err)
	}
	graph := parsed.Structure
	if graph == nil || graph.Verify() != nil {
		t.Fatal("structured graph missing")
	}
	cells := map[string]document.Block{}
	for _, b := range graph.Blocks {
		if b.Cell != nil {
			cells[b.Cell.Reference] = b
		}
	}
	b := cells["B2"].Cell
	if b.Value != "1234.567" || b.Display != "USD 1,234.57" || b.ValueType != "number" || b.NumberFormat != `"USD "#,##0.00` {
		t.Fatalf("lost financial source: %+v", b)
	}
	if cells["C2"].Cell.Display != "2024-01-01" || cells["C2"].Cell.Value != "45292" || cells["D2"].Cell.Display != "12.5%" {
		t.Fatal("lost date or percentage")
	}
	if cells["E2"].Cell.ResultState != "cached_recalculation_requested" || *cells["E2"].Cell.CachedValue != "9.25" || !cells["E2"].Cell.Hidden {
		t.Fatal("cache was treated as recalculated or hidden metadata lost")
	}
	if cells["B3"].Cell.ResultState != "missing_cache" || cells["B3"].Cell.CachedValue != nil || !cells["B3"].Cell.Hidden {
		t.Fatal("missing cache became zero")
	}
	if cells["C4"].Cell.Formula != "" || cells["C4"].Cell.FormulaAnchor != "C3" || *cells["C4"].Cell.SharedFormulaIndex != 7 {
		t.Fatal("shared formula was silently rewritten")
	}
	if cells["A5"].Cell.MergedRange != "A5:B5" || !graph.Units[1].Hidden || len(graph.Units[1].MergedRanges) != 1 {
		t.Fatal("merged/empty/hidden worksheet evidence lost")
	}
	linked := document.IndexLinks(graph).ForSpan(cells["B2"].Span)
	header := false
	for _, required := range linked.Required {
		header = header || required.ID == cells["B1"].ID
	}
	if !header {
		t.Fatal("numeric value detached from explicit table header")
	}
	for _, want := range []string{"1234.567", "USD 1,234.57", "format: yyyy-mm-dd", "no cached value; not evaluated", "hidden: column", "hidden: row"} {
		if !strings.Contains(parsed.Content, want) {
			t.Fatal("model context lost", want)
		}
	}
	data, _ := json.Marshal(graph)
	var restored document.Extraction
	if json.Unmarshal(data, &restored) != nil || restored.Verify() != nil {
		t.Fatal("typed graph did not survive serialization")
	}
	_, ranges, err := parsedSourceRanges(parsed)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(ranges)
	if strings.Contains(string(encoded), "cached_value") {
		t.Fatal("cell bodies duplicated into retrieval locators")
	}
}
