package rag

import (
	"archive/zip"
	"bytes"
	"compress/zlib"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// Produce a complete, isolated PDF with a real cross-reference table. No user
// documents or external converter are required to exercise the missing-tool path.
func parserPDF(t *testing.T, compressed bool) []byte {
	t.Helper()
	stream := []byte("BT /F1 12 Tf 20 20 Td (OffGrid PDF regression 12345) Tj ET")
	filter := ""
	if compressed {
		var data bytes.Buffer
		w := zlib.NewWriter(&data)
		if _, err := w.Write(stream); err != nil {
			t.Fatal(err)
		}
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		stream = data.Bytes()
		filter = "/Filter /FlateDecode "
	}
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 200] /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>",
		fmt.Sprintf("<< %s/Length %d >>\nstream\n%s\nendstream", filter, len(stream), stream),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
	}
	var pdf bytes.Buffer
	pdf.WriteString("%PDF-1.4\n")
	offsets := []int{0}
	for i, object := range objects {
		offsets = append(offsets, pdf.Len())
		fmt.Fprintf(&pdf, "%d 0 obj\n%s\nendobj\n", i+1, object)
	}
	xref := pdf.Len()
	fmt.Fprintf(&pdf, "xref\n0 %d\n0000000000 65535 f \n", len(offsets))
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&pdf, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&pdf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), xref)
	return pdf.Bytes()
}

func TestParserPDFWithoutExternalTools(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	for _, compressed := range []bool{false, true} {
		t.Run(fmt.Sprint(compressed), func(t *testing.T) {
			defer func() {
				if p := recover(); p != nil {
					t.Fatalf("PDF upload panicked: %v", p)
				}
			}()
			p := NewDocumentParser()
			p.pdfWorker = ""
			result, err := p.Parse(parserPDF(t, compressed), "sample.pdf", ".pdf")
			var problem *ParseError
			if result != nil || !errors.As(err, &problem) || problem.Code != "document_parser_unavailable" {
				t.Fatalf("missing worker silently fell back: %+v %v", result, err)
			}
		})
	}
}

func TestParserMalformedPDFDoesNotPanic(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	defer func() {
		if p := recover(); p != nil {
			t.Fatalf("malformed document panicked: %v", p)
		}
	}()
	if _, err := NewDocumentParser().Parse([]byte("%PDF-1.7\nnot a document"), "bad.pdf", ".pdf"); err == nil {
		t.Fatal("malformed document accepted")
	}
}

func TestParserPreservesUnicodeScripts(t *testing.T) {
	for _, text := range []string{"Ελληνικά κείμενα", "हिन्दी दस्तावेज़", "עברית מסמך", "เอกสารภาษาไทย", "አማርኛ ሰነድ", "café 日本語 𝕏 😀 e\u0301"} {
		if got := cleanExtractedText(text); got != text {
			t.Errorf("Unicode text damaged: got %q, want %q", got, text)
		}
	}
}

func TestParserLegacyXLSIsNotXLSX(t *testing.T) {
	_, err := NewDocumentParser().Parse([]byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1}, "legacy.xls", ".xls")
	if err == nil || !strings.Contains(err.Error(), "legacy XLS") {
		t.Fatalf("expected precise unsupported legacy XLS error, got %v", err)
	}
}

func parserZip(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var out bytes.Buffer
	w := zip.NewWriter(&out)
	for name, content := range entries {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func TestParserXLSXInlineStrings(t *testing.T) {
	data := parserZip(t, map[string]string{
		"xl/workbook.xml":            `<workbook xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Budget" sheetId="1" r:id="rId1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<Relationships><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/></Relationships>`,
		"xl/worksheets/sheet1.xml":   `<worksheet><sheetData><row r="1"><c r="A1" t="inlineStr"><is><t>Budget café 日本語</t></is></c><c r="B1" t="inlineStr"><is><r><t>Rich </t></r><r><t>text</t></r></is></c></row></sheetData></worksheet>`,
	})
	result, err := NewDocumentParser().Parse(data, "inline.xlsx", ".xlsx")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Content, "Budget café 日本語") || !strings.Contains(result.Content, "Rich text") {
		t.Fatalf("inline cell content lost: %q", result.Content)
	}
}
