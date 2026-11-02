package rag

import (
	"strconv"
	"strings"
	"testing"
)

func TestParserWorksheetCellsComeFromXMLNotText(t *testing.T) {
	data := parserZip(t, map[string]string{
		"xl/workbook.xml":            `<workbook xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Budget café" sheetId="1" r:id="sheet"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<Relationships><Relationship Id="sheet" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet.xml"/></Relationships>`,
		"xl/worksheets/sheet.xml": `<worksheet><sheetData><row><c r="A1" t="inlineStr"><is><t>日本語
Z99: this is inside A1</t></is></c><c r="B2"><f>A1*2</f><v>42</v></c><c r="C3" t="inlineStr"><is><t> </t></is></c></row></sheetData></worksheet>`,
	})
	parsed, err := NewDocumentParser().Parse(data, "book.xlsx", ".xlsx")
	if err != nil {
		t.Fatal(err)
	}
	text, ranges, err := parsedSourceRanges(parsed)
	if err != nil {
		t.Fatal(err)
	}
	if len(ranges) != 1 || len(ranges[0].Cells) != 3 || ranges[0].Revision != xlsxDocumentRevision {
		t.Fatal(ranges)
	}
	for i, ref := range []string{"A1", "B2", "C3"} {
		if ranges[0].Cells[i].Reference != ref {
			t.Fatal(ranges[0].Cells)
		}
	}
	fake := strings.Index(text, "Z99:")
	refs := ranges[0].CellReferences(fake, fake+3)
	if len(refs) != 1 || refs[0] != "A1" {
		t.Fatal("inferred cell identity from cell text", refs)
	}
	doc := &Document{ID: "cells", RawContent: text, SourceRetained: true, SourceRanges: ranges}
	chunks, err := NewChunker(ChunkingOptions{ChunkSize: 32, ChunkOverlap: 5}).chunkDocument(doc)
	if err != nil {
		t.Fatal(err)
	}
	for _, chunk := range chunks {
		if chunk.Source == nil || len(chunk.Source.Cells) == 0 {
			t.Fatal("cell identity lost", chunk)
		}
		for _, cell := range chunk.Source.Cells {
			if cell.Start >= chunk.EndChar || cell.End <= chunk.StartChar {
				t.Fatal("unrelated cell attached", chunk)
			}
		}
		got, err := doc.ReadSourcePassage(chunk.StartChar, chunk.EndChar, chunk.Source.SHA256, chunk.Source.Revision, chunk.ContentHash)
		if err != nil || got != chunk.Content {
			t.Fatal("cell citation readback", got, err)
		}
	}
	// Large chunks still cover multiple adjacent cells; no per-cell embedding explosion.
	chunks, err = NewChunker(ChunkingOptions{ChunkSize: 4096}).chunkDocument(doc)
	if err != nil || len(chunks) != 1 || len(chunks[0].Source.Cells) != 3 {
		t.Fatal(chunks, err)
	}
	// Legacy indexes remain readable without fabricating new cell references.
	legacy := *doc
	legacy.SourceRanges = append([]SourceRange(nil), ranges...)
	legacy.SourceRanges[0].Cells = nil
	chunks, err = NewChunker(DefaultChunkingOptions()).chunkDocument(&legacy)
	if err != nil || len(chunks[0].Source.Cells) != 0 {
		t.Fatal("legacy cell identity invented", err)
	}
}

func TestParserWorksheetRejectsAmbiguousCellIdentities(t *testing.T) {
	for _, ref := range []string{"", "a1", "A0", "A01", "XFE1", "A1048577", "AAAA1", "A-1", "<script>"} {
		t.Run(ref, func(t *testing.T) {
			if validCellReference(ref) {
				t.Fatal("invalid cell identity accepted")
			}
		})
	}
	for _, ref := range []string{"A1", "Z99", "AA2", "XFD1048576"} {
		if !validCellReference(ref) {
			t.Fatal(ref)
		}
	}
	_, _, _, err := extractWorkbookSheet([]byte(`<worksheet><sheetData><row><c r="A1"><v>1</v></c><c r="A1"><v>2</v></c></row></sheetData></worksheet>`), nil)
	if err == nil {
		t.Fatal("duplicate conflicting cell accepted")
	}
	var rows strings.Builder
	rows.WriteString("<worksheet><sheetData><row>")
	for i := 1; i <= 100001; i++ {
		rows.WriteString(`<c r="A`)
		rows.WriteString(strconv.Itoa(i))
		rows.WriteString(`"><v>1</v></c>`)
	}
	rows.WriteString("</row></sheetData></worksheet>")
	if _, _, _, err := extractWorkbookSheet([]byte(rows.String()), nil); err == nil {
		t.Fatal("cell count bound ignored")
	}
}

func TestWorksheetCitationTruncationDropsOmittedCells(t *testing.T) {
	first := "A1: " + strings.Repeat("café 日本語 ", 100)
	text := first + "\nB2: second cell"
	unit := SourceRange{Kind: "sheet", Index: 1, Start: 0, End: len(text), SHA256: strings.Repeat("a", 64), Revision: xlsxDocumentRevision, Cells: []SourceCell{{Reference: "A1", Start: 0, End: len(first)}, {Reference: "B2", Start: len(first) + 1, End: len(text)}}}
	chunk := &Chunk{ID: "cell-chunk", Content: text, StartChar: 0, EndChar: len(text), Source: &unit}
	rc := &RAGContext{Results: []SearchResult{{DocumentID: "doc", DocName: "book.xlsx", Chunk: chunk}}}
	rc.FormatContext()
	rc.TruncateContext(600)
	if len(rc.Results[0].Chunk.Source.Cells) != 1 || rc.Results[0].Chunk.Source.Cells[0].Reference != "A1" || len(chunk.Source.Cells) != 2 {
		t.Fatal("truncated citation claims omitted cells or mutates original")
	}
}
