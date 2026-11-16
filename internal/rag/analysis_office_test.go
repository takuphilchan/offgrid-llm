package rag

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

func TestAnalysisNativeOfficeExceptionAndFormulaReadback(t *testing.T) {
	for _, format := range []string{"docx", "xlsx"} {
		t.Run(format, func(t *testing.T) {
			e := NewEngine(nil, t.TempDir())
			defer e.Close()
			s, err := e.ImportStore()
			if err != nil {
				t.Fatal(err)
			}
			refs := []document.SourceRef{}
			for side := 0; side < 2; side++ {
				var data []byte
				if format == "docx" {
					parts := wordStyleFixture(`<w:style w:type="paragraph" w:styleId="clause"><w:pPr><w:outlineLvl w:val="0"/></w:pPr></w:style>`, `<w:p><w:pPr><w:pStyle w:val="clause"/></w:pPr><w:r><w:t>Policy</w:t></w:r></w:p><w:p><w:r><w:t>Deadline 30 June.</w:t><w:footnoteReference w:id="1"/></w:r></w:p>`)
					parts["word/footnotes.xml"] = `<w:footnotes xmlns:w="` + wordNamespace + `"><w:footnote w:id="1"><w:p><w:r><w:t>Rural exception: ` + []string{"31 July", "31 August"}[side] + `.</w:t></w:r></w:p></w:footnote></w:footnotes>`
					data = parserZip(t, parts)
				} else {
					original := fidelityWorkbook(t)
					z, err := zip.NewReader(bytes.NewReader(original), int64(len(original)))
					if err != nil {
						t.Fatal(err)
					}
					parts := map[string]string{}
					for _, f := range z.File {
						r, err := f.Open()
						if err != nil {
							t.Fatal(err)
						}
						raw, err := io.ReadAll(r)
						r.Close()
						if err != nil {
							t.Fatal(err)
						}
						parts[f.Name] = string(raw)
					}
					if side == 1 {
						parts["xl/worksheets/sheet1.xml"] = strings.Replace(parts["xl/worksheets/sheet1.xml"], "SUM(B2:B3)", "SUM(B2:B4)", 1)
					}
					data = parserZip(t, parts)
				}
				parsed, err := NewDocumentParser().Parse(data, "policy."+format, "."+format)
				if err != nil {
					t.Fatal(err)
				}
				text, ranges, err := parsedSourceRanges(parsed)
				if err != nil {
					t.Fatal(err)
				}
				d := &Document{ID: []string{"before", "after"}[side], Name: "Policy", RawContent: text, Metadata: parsed.Metadata, SourceRanges: ranges, Extraction: parsed.Structure, CreatedAt: time.Now(), UpdatedAt: time.Now()}
				if err = s.AddDocumentWithChunks(d, nil, nil); err != nil {
					t.Fatal(err)
				}
				refs = append(refs, document.SourceRef{DocumentID: d.ID, Revision: d.ExtractionRevision, OriginalSHA256: parsed.Structure.OriginalSHA256})
			}
			r, err := s.CreateComparison("owner", "office", refs)
			if err != nil {
				t.Fatal(err)
			}
			if err = e.StartComparison("owner", r.ID); err != nil {
				t.Fatal(err)
			}
			deadline := time.Now().Add(5 * time.Second)
			for time.Now().Before(deadline) {
				r, err = s.Analysis("owner", r.ID)
				if err != nil {
					t.Fatal(err)
				}
				if r.State == "complete" || r.State == "incomplete" {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			if r.State != "incomplete" || r.CompletedUnits != len(r.Plan.Units) {
				t.Fatal("Office coverage limitations lost", r)
			}
			found := false
			for step := range r.Pairs {
				page, err := s.AnalysisResults("owner", r.ID, step, 0, 100)
				if err != nil {
					t.Fatal(err)
				}
				for i, change := range page.Result.Changes {
					if format == "docx" && change.Kind == "text_added" {
						source, err := s.ReadAnalysisSource("owner", r.ID, step, i, "after")
						if err != nil {
							t.Fatal(err)
						}
						found = found || strings.Contains(source.Text, "31 August")
					}
					if format == "xlsx" && change.Kind == "cell_modified" {
						before, err := s.ReadAnalysisSource("owner", r.ID, step, i, "before")
						if err != nil {
							t.Fatal(err)
						}
						after, err := s.ReadAnalysisSource("owner", r.ID, step, i, "after")
						if err != nil {
							t.Fatal(err)
						}
						if before.Cell != nil && after.Cell != nil && before.Cell.Reference == "E2" {
							found = before.Cell.Formula == "SUM(B2:B3)" && after.Cell.Formula == "SUM(B2:B4)" && before.Cell.CachedValue != nil && after.Cell.CachedValue != nil && *before.Cell.CachedValue == *after.Cell.CachedValue
						}
					}
				}
			}
			if !found {
				t.Fatal("native changed exception/formula was not recoverable")
			}
			for _, exportFormat := range []string{"json", "csv", "markdown"} {
				artifact, err := s.ExportComparison(context.Background(), "owner", r.ID, exportFormat)
				if err != nil {
					t.Fatal("native export", err)
				}
				expected := []string{"incomplete", refs[0].Revision, refs[1].Revision}
				if format == "docx" {
					expected = append(expected, "31 July", "31 August")
				} else {
					expected = append(expected, "SUM(B2:B3)", "SUM(B2:B4)", "cached_value", "E2")
				}
				for _, value := range expected {
					if !strings.Contains(artifact.Content, value) {
						t.Fatalf("%s export lost %s", exportFormat, value)
					}
				}
			}
		})
	}
}
