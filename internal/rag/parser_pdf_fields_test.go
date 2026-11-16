package rag

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

func TestPDFNativeFormFixtureIdentity(t *testing.T) {
	metadata, err := os.ReadFile("testdata/documents/native-form.manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct{ SHA256, License, Provenance string }
	if err = json.Unmarshal(metadata, &manifest); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("testdata/documents/native-form.pdf")
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	if manifest.SHA256 != hex.EncodeToString(digest[:]) || manifest.License == "" || manifest.Provenance == "" {
		t.Fatal("unverified native form fixture")
	}
}

func TestPDFFieldReceiptPreservesExactNativeEvidence(t *testing.T) {
	source := []byte("%PDF-fixture")
	r := validWorkerReceipt(source)
	checked := true
	r["pages"] = []map[string]any{{"page": 1, "text": "", "state": "empty", "fields": []PDFFormField{{Index: 0, Name: "Amount", Value: "0017.25", Kind: 6}, {Index: 1, Name: "Approved", Value: "Yes", Kind: 2, Checked: &checked}}}}
	raw, _ := json.Marshal(r)
	result, err := NewDocumentParser().decodePDFWorker(raw, source)
	if err != nil {
		t.Fatal(err)
	}
	if err = result.bindStructure(); err != nil {
		t.Fatal(err)
	}
	if result.Content != "Amount: 0017.25\nApproved: [x] Yes" || len(result.Structure.Blocks) != 2 || result.Structure.Blocks[0].FormField.Value != "0017.25" || len(result.Coverage().ReviewUnits) != 1 {
		t.Fatalf("lost form evidence: %+v", result)
	}
	block := result.Structure.Blocks[0]
	derived, err := result.Structure.Correct("reviewer", time.Now(), []document.Correction{{BlockID: block.ID, ExpectedSHA256: document.TextSHA256(result.Content[block.Span.Start:block.Span.End]), Text: "Amount: 18.25"}})
	if err != nil || derived.Blocks[0].FormField != nil || result.Structure.Blocks[0].FormField.Value != "0017.25" {
		t.Fatal("review silently overwrote native value", err)
	}
	// After OCR, append the unchanged field metadata to the new page text.
	unit := result.SourceUnits[0]
	fields := unit.FormFields
	if pdfPageNativeText(unit) != "" {
		t.Fatal("field was misclassified as page text")
	}
	unit.Text = "Recognized page heading"
	if err = appendPDFFields(&unit, fields); err != nil || !strings.Contains(unit.Text, "0017.25") || pdfPageNativeText(unit) != "Recognized page heading" {
		t.Fatal("OCR replaced native fields", err)
	}
}

func TestPDFFieldReceiptRejectsMalformedFields(t *testing.T) {
	for _, fields := range [][]PDFFormField{
		{{Index: 0, Kind: 2}}, {{Index: 0, Kind: 9}}, {{Index: 0, Kind: 6}, {Index: 0, Kind: 6}}, {{Index: 0, Kind: 6, Bounds: &[4]float64{20, 20, 10, 10}}},
	} {
		u := SourceUnit{Text: "native"}
		if appendPDFFields(&u, fields) == nil {
			t.Fatal("accepted malformed field", fields)
		}
	}
}

func TestPDFChoiceEvidenceAndRetainedSpans(t *testing.T) {
	u := SourceUnit{Text: "Native page\n", State: "ready"}
	fields := []PDFFormField{{Index: 0, Name: "Regions", Label: "Service areas", Kind: 5, Options: []document.FormOption{{Index: 0, Label: "North", Selected: true}, {Index: 1, Label: "South"}, {Index: 2, Label: "West", Selected: true}}}}
	if err := appendPDFFields(&u, fields); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(u.Text, "[x] North [x] West") || strings.Contains(u.Text, "[x] South") || validatePDFFieldSpans(u) != nil {
		t.Fatal("choice evidence lost", u)
	}
	u.FormFields[0].Start++
	if validatePDFFieldSpans(u) == nil {
		t.Fatal("tampered saved span accepted")
	}
}

func TestPDFNativeFormRealWorker(t *testing.T) {
	worker := os.Getenv("OFFGRID_TEST_DOCUMENT_WORKER")
	if worker == "" {
		t.Skip("requires packaged PDF worker")
	}
	p := NewDocumentParser()
	p.pdfWorker = worker
	p.pdfCacheDir = t.TempDir()
	data, err := os.ReadFile("testdata/documents/native-form.pdf")
	if err != nil {
		t.Fatal(err)
	}
	r, err := p.ParseContext(context.Background(), data, "form.pdf", ".pdf")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Structure.Blocks) != 4 || !strings.Contains(r.Content, "Amount: 0017.25") {
		t.Fatalf("missing native fields: %+v", r)
	}
	fields := r.Structure.Blocks
	if fields[1].FormField.Checked == nil || !*fields[1].FormField.Checked || fields[2].FormField.Checked == nil || *fields[2].FormField.Checked || !fields[3].FormField.Hidden {
		t.Fatal("field states changed")
	}
	if r.Coverage().Status != "text_only" || len(r.Coverage().ReviewUnits) != 1 {
		t.Fatal("form review requirements lost")
	}
}
