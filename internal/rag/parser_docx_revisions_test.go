package rag

import (
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

func TestDOCXTrackedAlternativesStayOutOfCurrentEvidence(t *testing.T) {
	body := `<w:p><w:r><w:t>Amount </w:t></w:r><w:del w:id="1" w:author="Reviewer" w:date="2026-01-02T03:04:05Z"><w:r><w:delText>999.99 superseded</w:delText></w:r></w:del><w:ins w:id="2" w:author="Editor"><w:r><w:t>17.25 current</w:t></w:r></w:ins></w:p>` +
		`<w:del w:id="3"><w:p><w:r><w:delText>Deleted paragraph</w:delText></w:r></w:p></w:del>` +
		`<w:p><w:moveFrom w:id="4"><w:r><w:delText>Old location</w:delText></w:r></w:moveFrom><w:moveTo w:id="5"><w:r><w:t>New location</w:t></w:r></w:moveTo></w:p>`
	r, err := NewDocumentParser().Parse(parserZip(t, map[string]string{"word/document.xml": docxTestStart + body + docxTestEnd}), "policy.docx", ".docx")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(r.Content, "999.99") || strings.Contains(r.Content, "Deleted paragraph") || strings.Contains(r.Content, "Old location") || !strings.Contains(r.Content, "17.25 current") {
		t.Fatal("alternatives contaminated current evidence", r.Content)
	}
	changes := []document.RevisionChange{}
	for _, b := range r.Structure.Blocks {
		changes = append(changes, b.Revisions...)
	}
	if len(changes) != 5 || changes[0].Author != "Reviewer" || changes[0].Date != "2026-01-02T03:04:05Z" || changes[0].Text != "999.99 superseded" {
		t.Fatal("lost revision provenance", changes)
	}
	if r.Structure.Verify() != nil || r.Coverage().Status != "text_only" || len(r.Coverage().ReviewUnits) != 1 {
		t.Fatal("tracked evidence overclaimed complete", r.Coverage())
	}
	view, err := document.ProjectStructure(r.Structure, "policy", "", 0, 100, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, b := range view.Blocks {
		for _, c := range b.Revisions {
			found = found || strings.Contains(c.Text, "Deleted paragraph")
		}
	}
	if !found {
		t.Fatal("excluded paragraph cannot be inspected")
	}
}

func TestDOCXReferencedNoteRevisionsStayAttached(t *testing.T) {
	parts := map[string]string{
		"word/document.xml":  docxTestStart + `<w:p><w:r><w:t>Rural exception</w:t><w:footnoteReference w:id="1"/></w:r></w:p>` + docxTestEnd,
		"word/footnotes.xml": `<w:footnotes xmlns:w="` + wordNamespace + `"><w:footnote w:id="1"><w:p><w:del w:id="7" w:author="Legal"><w:r><w:delText>June 17.25</w:delText></w:r></w:del><w:ins w:id="8"><w:r><w:t>July 18.25</w:t></w:r></w:ins></w:p></w:footnote></w:footnotes>`,
	}
	r, err := NewDocumentParser().Parse(parserZip(t, parts), "notes.docx", ".docx")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, b := range r.Structure.Blocks {
		if b.Kind == "footnote" {
			found = len(b.Revisions) == 2 && b.Revisions[0].Text == "June 17.25" && b.Revisions[0].Author == "Legal"
		}
	}
	if !found || strings.Contains(r.Content, "June") || !strings.Contains(r.Content, "July 18.25") || len(r.Coverage().ReviewUnits) != 1 {
		t.Fatal("note alternatives lost or searchable", r)
	}
}

func TestDOCXTrackedRevisionLimits(t *testing.T) {
	body := `<w:p><w:r><w:t>Current</w:t></w:r><w:del w:id="1"><w:r><w:delText>` + strings.Repeat("x", (1<<20)+1) + `</w:delText></w:r></w:del></w:p>`
	if _, err := NewDocumentParser().Parse(parserZip(t, map[string]string{"word/document.xml": docxTestStart + body + docxTestEnd}), "large.docx", ".docx"); err == nil {
		t.Fatal("unbounded alternatives retained")
	}
}
