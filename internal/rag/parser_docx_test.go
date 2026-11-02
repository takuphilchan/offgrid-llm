package rag

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const docxTestStart = `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>`
const docxTestEnd = `</w:body></w:document>`

func TestParserDOCXPreservesDocumentOrderAndRunBoundaries(t *testing.T) {
	body := `<w:p><w:r><w:t>Before</w:t></w:r></w:p>` +
		`<w:tbl><w:tr><w:tc><w:p><w:r><w:t>café 日本語</w:t></w:r></w:p></w:tc></w:tr></w:tbl>` +
		`<w:p><w:r><w:t>A</w:t><w:tab/><w:t>B</w:t><w:br/><w:t>C</w:t></w:r></w:p>` +
		`<w:p><w:hyperlink><w:r><w:t>After &amp; linked text</w:t></w:r></w:hyperlink></w:p>` +
		`<w:del><w:r><w:delText>Deleted</w:delText><w:t>Also deleted</w:t></w:r></w:del>` +
		`<w:p><w:r><w:instrText>PRIVATE FIELD INSTRUCTION</w:instrText><w:t>Visible result</w:t></w:r></w:p>`
	result, err := NewDocumentParser().Parse(parserZip(t, map[string]string{"word/document.xml": docxTestStart + body + docxTestEnd}), "ordered.docx", ".docx")
	if err != nil {
		t.Fatal(err)
	}
	remaining := result.Content
	for _, want := range []string{"Before", "café 日本語", "A\tB\nC", "After & linked text", "Visible result"} {
		i := strings.Index(remaining, want)
		if i < 0 {
			t.Fatalf("missing or reordered %q: %q", want, result.Content)
		}
		remaining = remaining[i+len(want):]
	}
	if strings.Contains(result.Content, "deleted") || strings.Contains(result.Content, "PRIVATE") {
		t.Fatal("hidden content became visible body text")
	}
	if result.PageCount != 0 || len(result.SourceUnits) != 1 || result.SourceUnits[0].Kind != "section" {
		t.Fatal("invented rendered pagination")
	}
}

func TestParserDOCXRejectsMalformedInsteadOfRegexRecovering(t *testing.T) {
	for _, source := range []string{
		docxTestStart + `<w:p><w:r><w:t>plausible text</w:t></w:p>` + docxTestEnd,
		`<document><body><p><t>wrong namespace</t></p></body></document>`,
		docxTestStart + `<w:t>first</w:t>` + docxTestEnd + docxTestStart + `<w:t>second</w:t>` + docxTestEnd,
		`<!DOCTYPE document [<!ENTITY private SYSTEM "file:///private">]>` + docxTestStart + `<w:t>&private;</w:t>` + docxTestEnd,
		docxTestStart + strings.Repeat(`<w:p>`, 256) + `<w:t>too deep</w:t>` + strings.Repeat(`</w:p>`, 256) + docxTestEnd,
	} {
		result, err := NewDocumentParser().Parse(parserZip(t, map[string]string{"word/document.xml": source}), "bad.docx", ".docx")
		if err == nil || result != nil {
			t.Fatalf("malformed document accepted: %+v %v", result, err)
		}
	}
}

func TestParserDOCXFixtureUsesActualSourceIdentity(t *testing.T) {
	for _, fixture := range readParserCorpus(t) {
		if !strings.HasSuffix(fixture.Path, ".docx") || fixture.ExpectedError != "" {
			continue
		}
		data, err := os.ReadFile(filepath.Join("testdata/documents/corpus", fixture.Path))
		if err != nil {
			t.Fatal(err)
		}
		result, err := NewDocumentParser().Parse(data, fixture.Path, ".docx")
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range fixture.ExpectedText {
			if !strings.Contains(result.Content, want) {
				t.Fatalf("fixture text lost: %q", want)
			}
		}
		if len(result.SourceUnits) != 1 || result.SourceUnits[0].SourceSHA256 != fixture.SHA256 {
			t.Fatalf("source digest lost: %+v", result.SourceUnits)
		}
	}
}
