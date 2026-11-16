package rag

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

const pptxNamespaces = `xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"`

func presentationFixtureParts() map[string]string {
	shape := func(text string) string {
		return `<p:sp><p:txBody><a:p><a:r><a:t>` + text + `</a:t></a:r></a:p></p:txBody></p:sp>`
	}
	cell := func(text string) string {
		return `<a:tc><a:txBody><a:p><a:r><a:t>` + text + `</a:t></a:r></a:p></a:txBody></a:tc>`
	}
	return map[string]string{
		"ppt/presentation.xml":            `<p:presentation ` + pptxNamespaces + `><p:sldIdLst><p:sldId id="901" r:id="s"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels": `<Relationships><Relationship Id="s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide.xml"/></Relationships>`,
		"ppt/slides/slide.xml": `<p:sld ` + pptxNamespaces + ` xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart"><p:cSld><p:spTree>` +
			`<p:sp><p:nvSpPr><p:nvPr><p:ph type="title"/></p:nvPr></p:nvSpPr><p:txBody><a:p><a:r><a:t>Clinic policy</a:t></a:r></a:p></p:txBody></p:sp>` +
			shape("Applications close in June.") + `<p:graphicFrame><a:graphic><a:graphicData><a:tbl><a:tblPr firstRow="1"/><a:tblGrid><a:gridCol/><a:gridCol/></a:tblGrid><a:tr>` + cell("Category") + cell("Annual cap (USD)") + `</a:tr><a:tr>` + cell("Rural clinics") + cell("17.250") + `</a:tr></a:tbl></a:graphicData></a:graphic></p:graphicFrame>` +
			`<p:pic><p:nvPicPr><p:cNvPr id="9" name="Diagram" descr="Authored allocation diagram" title="Diagram title"/></p:nvPicPr><p:blipFill><a:blip r:embed="image"/></p:blipFill></p:pic>` +
			`<p:graphicFrame><a:graphic><a:graphicData><c:chart r:id="chart"/></a:graphicData></a:graphic></p:graphicFrame></p:spTree></p:cSld></p:sld>`,
		"ppt/slides/_rels/slide.xml.rels": `<Relationships><Relationship Id="notes" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/notesSlide" Target="../notesSlides/notes.xml"/><Relationship Id="image" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="../media/image.png"/><Relationship Id="chart" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/chart" Target="../charts/chart.xml"/></Relationships>`,
		"ppt/notesSlides/notes.xml":       `<p:notes ` + pptxNamespaces + `><p:cSld><p:spTree><p:sp><p:nvSpPr><p:nvPr><p:ph type="sldNum"/></p:nvPr></p:nvSpPr><p:txBody><a:p><a:r><a:t>9999</a:t></a:r></a:p></p:txBody></p:sp>` + shape("Exception: island applicants may apply until July. 日本語 العربية") + `</p:spTree></p:cSld></p:notes>`,
		"ppt/media/image.png":             "uninterpreted binary fixture",
		"ppt/charts/chart.xml":            `<chart><externalData>UNVERIFIED CHART VALUE 98765</externalData></chart>`,
	}
}

func TestPPTXStructureNotesTablesAndVisualIdentity(t *testing.T) {
	r, err := NewDocumentParser().Parse(parserZip(t, presentationFixtureParts()), "policy.pptx", ".pptx")
	if err != nil {
		t.Fatal(err)
	}
	graph := r.Structure
	if graph == nil || graph.Verify() != nil {
		t.Fatal("missing graph")
	}
	for _, want := range []string{"Clinic policy", "17.250", "[Speaker note] Exception:", "日本語 العربية", "Authored allocation diagram"} {
		if !strings.Contains(r.Content, want) {
			t.Fatal("missing source", want)
		}
	}
	if strings.Contains(r.Content, "9999") || strings.Contains(r.Content, "98765") {
		t.Fatal("notes footer or unprocessed chart treated as evidence")
	}
	var clause document.Block
	cells, figures, notes := 0, 0, 0
	for _, block := range graph.Blocks {
		text := graph.Text[block.Span.Start:block.Span.End]
		if text == "Applications close in June." {
			clause = block
		}
		if block.Cell != nil {
			cells++
			if block.Cell.Value == "17.250" && block.Cell.Display != "17.250" {
				t.Fatal("native digits changed")
			}
		}
		if block.Table != nil && (!block.Table.FirstRowStyle || block.Table.Rows != 2 || block.Table.Columns != 2) {
			t.Fatal("lost native table metadata")
		}
		if block.Figure != nil {
			figures++
			if block.Figure.Part == "" || block.ReviewState != "unprocessed_visual" {
				t.Fatal("visual identity not retained")
			}
		}
		if block.ReviewState == "speaker_notes" {
			notes++
			if block.SourcePart != "ppt/notesSlides/notes.xml" {
				t.Fatal("notes attributed to slide text")
			}
		}
	}
	if cells != 4 || figures != 2 || notes < 1 || clause.ID == "" {
		t.Fatal("lost typed blocks", cells, figures, notes)
	}
	if len(r.Coverage().ReviewUnits) != 1 || r.Coverage().ReadableUnits != 1 {
		t.Fatal("visual/table review not surfaced independently of readable text")
	}
	for _, link := range graph.Relationships {
		if link.Kind == "header_for" {
			t.Fatal("styling fabricated semantic header")
		}
	}
	linked := document.IndexLinks(graph).ForSpan(clause.Span)
	found := false
	for _, required := range linked.Required {
		found = found || strings.Contains(graph.Text[required.Span.Start:required.Span.End], "island applicants")
	}
	if !found {
		t.Fatal("notes exception detached from slide clause")
	}
	data, _ := json.Marshal(graph)
	var restored document.Extraction
	if json.Unmarshal(data, &restored) != nil || restored.Verify() != nil {
		t.Fatal("structured evidence failed serialization")
	}
	text, ranges, err := parsedSourceRanges(r)
	if err != nil {
		t.Fatal(err)
	}
	d := &Document{ID: "presentation", RawContent: text, SourceRanges: ranges, Extraction: graph, SourceRetained: true}
	chunks, err := NewChunker(DefaultChunkingOptions()).chunkDocument(d)
	if err != nil {
		t.Fatal(err)
	}
	for _, chunk := range chunks {
		read, err := d.ReadSourcePassage(chunk.StartChar, chunk.EndChar, chunk.Source.SHA256, chunk.Source.Revision, chunk.ContentHash)
		if err != nil || read != chunk.Content {
			t.Fatal("source mismatch", err)
		}
	}
}

func TestPPTXHiddenAndExternalVisualsAreDisclosedWithoutFetching(t *testing.T) {
	parts := presentationFixtureParts()
	parts["ppt/slides/slide.xml"] = strings.Replace(parts["ppt/slides/slide.xml"], `<p:sld `, `<p:sld show="0" `, 1)
	parts["ppt/slides/_rels/slide.xml.rels"] = strings.Replace(parts["ppt/slides/_rels/slide.xml.rels"], `Target="../media/image.png"`, `Target="https://example.invalid/private.png" TargetMode="External"`, 1)
	delete(parts, "ppt/media/image.png")
	r, err := NewDocumentParser().Parse(parserZip(t, parts), "hidden.pptx", ".pptx")
	if err != nil {
		t.Fatal(err)
	}
	if !r.Structure.Units[0].Hidden || !r.SourceUnits[0].Hidden || !strings.Contains(r.Content, "[Hidden slide]") {
		t.Fatal("hidden slide undisclosed")
	}
	for _, b := range r.Structure.Blocks {
		if b.Figure != nil && b.Figure.Kind == "image" && (!b.Figure.External || b.Figure.Part != "") {
			t.Fatal("external visual treated as local")
		}
	}
	data, _ := json.Marshal(r.Structure)
	if strings.Contains(string(data), "https://example.invalid") {
		t.Fatal("external URI copied into source projection")
	}
}

func TestPPTXRejectsMissingExternalOrAmbiguousNotes(t *testing.T) {
	for _, mode := range []string{"missing", "external", "duplicate", "escape"} {
		t.Run(mode, func(t *testing.T) {
			parts := presentationFixtureParts()
			rels := parts["ppt/slides/_rels/slide.xml.rels"]
			switch mode {
			case "missing":
				delete(parts, "ppt/notesSlides/notes.xml")
			case "external":
				rels = strings.Replace(rels, `Target="../notesSlides/notes.xml"`, `Target="https://example.invalid/private" TargetMode="External"`, 1)
			case "duplicate":
				rels = strings.Replace(rels, `</Relationships>`, `<Relationship Id="second" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/notesSlide" Target="../notesSlides/notes.xml"/></Relationships>`, 1)
			case "escape":
				rels = strings.Replace(rels, `../notesSlides/notes.xml`, `../../../outside.xml`, 1)
			}
			parts["ppt/slides/_rels/slide.xml.rels"] = rels
			if result, err := NewDocumentParser().Parse(parserZip(t, parts), "bad.pptx", ".pptx"); err == nil || result != nil {
				t.Fatal("unread notes silently omitted")
			}
		})
	}
}

func TestPPTXBoundedXMLAndSlideCounts(t *testing.T) {
	if _, err := presentationTree([]byte(strings.Repeat("<x>", 130) + strings.Repeat("</x>", 130))); err == nil {
		t.Fatal("unbounded XML depth")
	}
	var xml strings.Builder
	xml.WriteString(`<p:presentation ` + pptxNamespaces + `><p:sldIdLst>`)
	for i := 0; i < 1001; i++ {
		fmt.Fprintf(&xml, `<p:sldId r:id="s%d"/>`, i)
	}
	xml.WriteString(`</p:sldIdLst></p:presentation>`)
	if _, err := presentationSlideIDs([]byte(xml.String())); err == nil {
		t.Fatal("unbounded slide count")
	}
}
