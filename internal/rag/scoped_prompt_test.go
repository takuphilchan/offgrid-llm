package rag

import (
	"context"
	"testing"
)

func TestSelectedDocumentsRejectUnavailableBeforeInference(t *testing.T) {
	e := NewEngine(nil, t.TempDir())
	defer e.Close()
	for _, status := range []string{"", "failed", "pending"} {
		id := "doc-" + status
		if err := e.store.AddDocument(&Document{ID: id, Name: id, IndexStatus: status}); err != nil {
			t.Fatal(err)
		}
		if _, material, err := e.EnhancePromptForDocuments(context.Background(), "question", []string{id}); err == nil || material != nil {
			t.Fatal("unavailable document accepted")
		}
	}
	for _, ids := range [][]string{nil, {}, {"missing"}, make([]string, 17)} {
		if _, material, err := e.EnhancePromptForDocuments(context.Background(), "question", ids); err == nil || material != nil {
			t.Fatal("invalid selection accepted")
		}
	}
}
