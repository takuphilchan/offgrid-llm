package rag

import (
	"encoding/json"
	"testing"
)

func TestNativeVisualTextAdmissionPreservesPartialCoverage(t *testing.T) {
	source := []byte("source")
	receipt := validWorkerReceipt(source)
	receipt["pages"] = []any{map[string]any{"page": 1, "text": "Native evidence", "state": "native_text_with_visuals"}}
	raw, _ := json.Marshal(receipt)
	result, err := NewDocumentParser().decodePDFWorker(raw, source)
	if err == nil || !NativeTextUsable(result) || result.State != ExtractionPartial || len(result.Warnings) != 1 || result.Warnings[0].Code != "document_visual_content_unread" {
		t.Fatalf("%+v %v", result, err)
	}
	for _, state := range []string{"ocr_required", "ocr_partial", "failed", "unknown"} {
		copy := *result
		copy.SourceUnits = append(append([]SourceUnit{}, result.SourceUnits...), SourceUnit{Index: 2, State: state, Text: "Header"})
		if NativeTextUsable(&copy) {
			t.Fatalf("unread %s page admitted", state)
		}
	}
	result.SourceUnits[0].State = "ready"
	if NativeTextUsable(result) {
		t.Fatal("ordinary full extraction mislabeled partial-use")
	}
}
