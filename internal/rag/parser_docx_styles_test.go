package rag

import (
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

func wordStyleFixture(styles, body string) map[string]string {
	return map[string]string{
		"word/document.xml": docxTestStart + body + docxTestEnd,
		"word/styles.xml":   `<w:styles xmlns:w="` + wordNamespace + `">` + styles + `</w:styles>`,
	}
}

func TestDOCXStyleInheritanceAndCurrentOutline(t *testing.T) {
	styles := `<w:style w:type="paragraph" w:styleId="base"><w:pPr><w:outlineLvl w:val="1"/></w:pPr></w:style>` +
		`<w:style w:type="paragraph" w:styleId="条款"><w:basedOn w:val="base"/></w:style>` +
		`<w:style w:type="paragraph" w:styleId="body"><w:basedOn w:val="base"/><w:pPr><w:outlineLvl w:val="9"/></w:pPr></w:style>` +
		`<w:style w:type="character" w:styleId="character"><w:pPr><w:outlineLvl w:val="0"/></w:pPr></w:style>` +
		`<w:style w:type="paragraph" w:styleId="wrongParent"><w:basedOn w:val="character"/></w:style>` +
		`<w:style w:type="paragraph" w:styleId="missingParent"><w:basedOn w:val="absent"/></w:style>`
	tests := []struct {
		name, properties string
		level            int
	}{
		{"政策 17.25", `<w:pStyle w:val="条款"/>`, 2},
		{"explicit body", `<w:pStyle w:val="条款"/><w:outlineLvl w:val="9"/>`, 0},
		{"inherited body", `<w:pStyle w:val="body"/>`, 0},
		{"current heading", `<w:pStyle w:val="条款"/><w:pPrChange><w:pPr><w:outlineLvl w:val="9"/></w:pPr></w:pPrChange>`, 2},
		{"historical heading", `<w:pStyle w:val="body"/><w:pPrChange><w:pPr><w:outlineLvl w:val="0"/></w:pPr></w:pPrChange>`, 0},
		{"different type", `<w:pStyle w:val="wrongParent"/>`, 0},
		{"missing parent", `<w:pStyle w:val="missingParent"/>`, 0},
		{"no name heuristic", `<w:pStyle w:val="Heading1"/>`, 0},
		{"direct heading", `<w:pStyle w:val="body"/><w:outlineLvl w:val="3"/>`, 4},
		{"numbered heading", `<w:pStyle w:val="条款"/><w:numPr><w:numId w:val="1"/></w:numPr>`, 2},
	}
	body := ""
	for _, tc := range tests {
		body += `<w:p><w:pPr>` + tc.properties + `</w:pPr><w:r><w:t>` + tc.name + `</w:t></w:r></w:p>`
	}
	r, err := NewDocumentParser().Parse(parserZip(t, wordStyleFixture(styles, body)), "styles.docx", ".docx")
	if err != nil {
		t.Fatal(err)
	}
	if r.Structure.Verify() != nil || len(r.Structure.Blocks) != len(tests) {
		t.Fatal("invalid structure", r.Structure)
	}
	if r.Metadata["tracked_changes"] != "present_current_text_only" {
		t.Fatal("historical paragraph properties were not disclosed")
	}
	for i, tc := range tests {
		b := r.Structure.Blocks[i]
		if b.HeadingLevel != tc.level || (b.Kind == "heading") != (tc.level > 0) || strings.TrimSpace(r.Content[b.Span.Start:b.Span.End]) != tc.name {
			t.Errorf("%s: unexpected block %+v", tc.name, b)
		}
	}
	page, err := document.ProjectStructure(r.Structure, "document", "", 0, 100, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if page.TotalNavigation != 5 || page.Navigation[0].Title != "政策 17.25" {
		t.Fatal("outline lost declared headings", page.Navigation)
	}
}

func TestDOCXStyleDefaultsAndRelationshipTarget(t *testing.T) {
	for _, tc := range []struct {
		name, styles, properties string
		level                    int
	}{
		{"document default", `<w:docDefaults><w:pPrDefault><w:pPr><w:outlineLvl w:val="2"/></w:pPr></w:pPrDefault></w:docDefaults>`, "", 3},
		{"default paragraph", `<w:style w:type="paragraph" w:default="1" w:styleId="normal"><w:pPr><w:outlineLvl w:val="1"/></w:pPr></w:style>`, "", 2},
		{"missing style uses default", `<w:style w:type="paragraph" w:default="1" w:styleId="normal"><w:pPr><w:outlineLvl w:val="1"/></w:pPr></w:style>`, `<w:pStyle w:val="missing"/>`, 2},
		{"explicit body overrides default", `<w:style w:type="paragraph" w:default="1" w:styleId="normal"><w:pPr><w:outlineLvl w:val="1"/></w:pPr></w:style>`, `<w:outlineLvl w:val="9"/>`, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parts := wordStyleFixture(tc.styles, `<w:p><w:pPr>`+tc.properties+`</w:pPr><w:r><w:t>Native 0017.25</w:t></w:r></w:p>`)
			parts["word/custom/styles.xml"] = parts["word/styles.xml"]
			parts["word/styles.xml"] = "unused malformed part"
			parts["word/_rels/document.xml.rels"] = `<Relationships><Relationship Id="s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="custom/styles.xml"/></Relationships>`
			r, err := NewDocumentParser().Parse(parserZip(t, parts), "style.docx", ".docx")
			if err != nil {
				t.Fatal(err)
			}
			if r.Structure.Blocks[0].HeadingLevel != tc.level || r.Content != "Native 0017.25" {
				t.Fatal("default style or native text lost", r)
			}
		})
	}
}

func TestDOCXStylesRejectUnsafeOrAmbiguousParts(t *testing.T) {
	for _, mode := range []string{"cycle", "duplicate", "invalid-level", "duplicate-level", "wrong-root", "directive", "external", "escape", "missing", "duplicate-relation", "deep"} {
		t.Run(mode, func(t *testing.T) {
			styles := `<w:style w:type="paragraph" w:styleId="a"><w:pPr><w:outlineLvl w:val="0"/></w:pPr></w:style>`
			switch mode {
			case "cycle":
				styles = `<w:style w:type="paragraph" w:styleId="a"><w:basedOn w:val="b"/></w:style><w:style w:type="paragraph" w:styleId="b"><w:basedOn w:val="a"/></w:style>`
			case "duplicate":
				styles += styles
			case "invalid-level":
				styles = strings.Replace(styles, `val="0"`, `val="10"`, 1)
			case "duplicate-level":
				styles = strings.Replace(styles, `</w:pPr>`, `<w:outlineLvl w:val="1"/></w:pPr>`, 1)
			case "deep":
				styles = strings.Repeat("<w:pPr>", 65) + strings.Repeat("</w:pPr>", 65)
			}
			parts := wordStyleFixture(styles, `<w:p><w:r><w:t>Policy</w:t></w:r></w:p>`)
			relation := `<Relationship Id="s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>`
			switch mode {
			case "wrong-root":
				parts["word/styles.xml"] = `<w:document xmlns:w="` + wordNamespace + `"/>`
			case "directive":
				parts["word/styles.xml"] = `<!DOCTYPE styles>` + parts["word/styles.xml"]
			case "external":
				relation = strings.Replace(relation, `Target="styles.xml"`, `TargetMode="External" Target="https://example.invalid/styles.xml"`, 1)
			case "escape":
				relation = strings.Replace(relation, `styles.xml`, `../../outside.xml`, 1)
			case "missing":
				delete(parts, "word/styles.xml")
			case "duplicate-relation":
				relation += strings.Replace(relation, `Id="s"`, `Id="s2"`, 1)
			}
			parts["word/_rels/document.xml.rels"] = `<Relationships>` + relation + `</Relationships>`
			if _, err := NewDocumentParser().Parse(parserZip(t, parts), "bad.docx", ".docx"); err == nil {
				t.Fatal("accepted", mode)
			}
		})
	}
}
