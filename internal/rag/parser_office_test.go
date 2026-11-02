package rag

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParserOfficeDocumentFixtures(t *testing.T) {
	data, err := os.ReadFile("testdata/documents/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Files []struct{ Path, SHA256 string }
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	for _, entry := range manifest.Files {
		t.Run(entry.Path, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("testdata/documents", entry.Path))
			if err != nil {
				t.Fatal(err)
			}
			digest := sha256.Sum256(content)
			if hex.EncodeToString(digest[:]) != entry.SHA256 {
				t.Fatal("fixture differs from frozen manifest")
			}
			result, err := NewDocumentParser().Parse(content, entry.Path, filepath.Ext(entry.Path))
			if err != nil {
				t.Fatal(err)
			}
			if strings.HasSuffix(entry.Path, ".xlsx") {
				for _, required := range []string{"--- Sheet: First sheet ---", "--- Sheet: Budget café ---", "C3: 日本語 Ελληνικά हिन्दी", "D4: Rich cell text", "B6: 12", "formula: =B6*2; no cached value; not evaluated"} {
					if !strings.Contains(result.Content, required) {
						t.Errorf("missing %q in %q", required, result.Content)
					}
				}
				if strings.Index(result.Content, "First sheet") > strings.Index(result.Content, "Budget café") {
					t.Fatal("workbook order lost")
				}
			} else {
				if result.PageCount != 3 {
					t.Fatalf("slide count=%d", result.PageCount)
				}
				if !strings.HasPrefix(result.Content, "--- Slide 1 ---\nDisplayed first: café 日本語") || !strings.Contains(result.Content, "--- Slide 2 ---\nCreated first, displayed second") {
					t.Fatalf("slide order lost: %q", result.Content)
				}
			}
		})
	}
}

func TestParserOfficeRejectsBrokenRelationshipsAndValues(t *testing.T) {
	for _, test := range []struct{ name, target, mode, cell string }{
		{"external", "https://example.org/private", "External", ""},
		{"escape", "../../outside.xml", "", ""},
		{"missing", "worksheets/missing.xml", "", ""},
		{"bad string index", "worksheets/sheet1.xml", "", `<c r="A1" t="s"><v>not-an-index</v></c>`},
	} {
		t.Run(test.name, func(t *testing.T) {
			content := parserZip(t, map[string]string{
				"xl/workbook.xml":            `<workbook xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Budget" sheetId="1" r:id="rId1"/></sheets></workbook>`,
				"xl/_rels/workbook.xml.rels": `<Relationships><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="` + test.target + `" TargetMode="` + test.mode + `"/></Relationships>`,
				"xl/worksheets/sheet1.xml":   `<worksheet><sheetData><row>` + test.cell + `</row></sheetData></worksheet>`,
			})
			if result, err := NewDocumentParser().Parse(content, "bad.xlsx", ".xlsx"); err == nil || result != nil {
				t.Fatalf("invalid workbook accepted: %+v %v", result, err)
			}
		})
	}
}
