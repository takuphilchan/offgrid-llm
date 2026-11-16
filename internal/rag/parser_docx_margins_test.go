package rag

import (
	"strings"
	"testing"
)

func wordMarginFixture(body string) map[string]string {
	return map[string]string{
		"word/document.xml":            `<w:document xmlns:w="` + wordNamespace + `" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><w:body>` + body + `<w:sectPr><w:headerReference r:id="h"/><w:headerReference r:id="h"/><w:footerReference r:id="f"/></w:sectPr></w:body></w:document>`,
		"word/_rels/document.xml.rels": `<Relationships><Relationship Id="h" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/header" Target="header1.xml"/><Relationship Id="f" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/footer" Target="footer1.xml"/></Relationships>`,
		"word/header1.xml":             `<w:hdr xmlns:w="` + wordNamespace + `"><w:p><w:r><w:t>Effective 2026-11-13 · 分类</w:t></w:r></w:p></w:hdr>`,
		"word/footer1.xml":             `<w:ftr xmlns:w="` + wordNamespace + `"><w:p><w:r><w:t>Except rural applications: 31 July.</w:t></w:r></w:p></w:ftr>`,
		"word/header-unused.xml":       `this unused part must not be opened`,
	}
}

func TestDOCXReferencedMarginsRetainSourceAndDoNotInventPages(t *testing.T) {
	for _, body := range []string{`<w:p><w:r><w:t>Main policy.</w:t></w:r></w:p>`, ""} {
		r, err := NewDocumentParser().Parse(parserZip(t, wordMarginFixture(body)), "policy.docx", ".docx")
		if err != nil {
			t.Fatal(err)
		}
		if r.Structure.Verify() != nil || len(r.SourceUnits) != 3 || strings.Count(r.Content, "Effective") != 1 || !strings.Contains(r.Content, "31 July") {
			t.Fatal(r)
		}
		if _, _, err = parsedSourceRanges(r); err != nil {
			t.Fatal(err)
		}
		found := map[string]bool{}
		for _, b := range r.Structure.Blocks {
			if b.Kind == "header" || b.Kind == "footer" {
				found[b.Kind] = true
				if b.SourcePart != "word/"+b.Kind+"1.xml" || b.UnitID == "section-1" || b.Bounds != nil {
					t.Fatal("invented provenance", b)
				}
			}
		}
		if !found["header"] || !found["footer"] {
			t.Fatal("untyped margins")
		}
		if r.Coverage().Status != "text_only" {
			t.Fatal("layout limits lost")
		}
		if len(r.Coverage().ReviewUnits) != 2 || r.Coverage().ReviewUnits[0] != 2 || r.Coverage().ReviewUnits[1] != 3 {
			t.Fatal("margin placement review units lost", r.Coverage())
		}
	}
}

func TestDOCXMarginReferencesFailClosed(t *testing.T) {
	for _, mode := range []string{"external", "missing", "escape", "wrong-root", "conflicting-role"} {
		parts := wordMarginFixture(`<w:p><w:r><w:t>Policy</w:t></w:r></w:p>`)
		switch mode {
		case "external":
			parts["word/_rels/document.xml.rels"] = strings.Replace(parts["word/_rels/document.xml.rels"], `Target="header1.xml"`, `TargetMode="External" Target="https://example.invalid/private"`, 1)
		case "missing":
			delete(parts, "word/header1.xml")
		case "escape":
			parts["word/_rels/document.xml.rels"] = strings.Replace(parts["word/_rels/document.xml.rels"], `header1.xml`, `../../outside.xml`, 1)
		case "wrong-root":
			parts["word/header1.xml"] = parts["word/footer1.xml"]
		case "conflicting-role":
			parts["word/_rels/document.xml.rels"] = strings.Replace(parts["word/_rels/document.xml.rels"], `Target="footer1.xml"`, `Target="header1.xml"`, 1)
		}
		if _, err := NewDocumentParser().Parse(parserZip(t, parts), "bad.docx", ".docx"); err == nil {
			t.Fatal("accepted", mode)
		}
	}
}

func TestDOCXMarginsExcludeHistoricalReferencesAndDeletedText(t *testing.T) {
	parts := wordMarginFixture(`<w:p><w:r><w:t>Policy</w:t></w:r></w:p>`)
	parts["word/document.xml"] = strings.Replace(parts["word/document.xml"], `</w:sectPr>`, `<w:sectPrChange><w:sectPr><w:headerReference r:id="missing-historical"/></w:sectPr></w:sectPrChange></w:sectPr>`, 1)
	parts["word/header1.xml"] = strings.Replace(parts["word/header1.xml"], `</w:hdr>`, `<w:del><w:p><w:r><w:delText>Old classification</w:delText></w:r></w:p></w:del><w:ins><w:p><w:r><w:t>Current classification</w:t></w:r></w:p></w:ins></w:hdr>`, 1)
	r, err := NewDocumentParser().Parse(parserZip(t, parts), "revised.docx", ".docx")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(r.Content, "Old classification") || !strings.Contains(r.Content, "Current classification") || r.Metadata["tracked_changes"] != "present_current_text_only" {
		t.Fatal(r)
	}
	found := false
	for _, w := range r.Warnings {
		found = found || (w.Code == "document_tracked_changes" && w.SourceIndex == 2)
	}
	if !found {
		t.Fatal("missing tracked margin warning")
	}
}
