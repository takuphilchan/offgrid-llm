package server

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
)

func TestCapturedKnowledgeKeepsParserLocatorInSavedHistory(t *testing.T) {
	var passages []sessions.KnowledgePassage
	ctx := withKnowledgeScope(context.Background(), nil, &passages)
	result := &rag.RAGContext{Results: []rag.SearchResult{{DocumentID: "doc", DocName: "manual.pdf", Chunk: &rag.Chunk{ID: "chunk", Content: "café", StartChar: 11, EndChar: 16, Source: &rag.SourceRange{Kind: "page", Index: 3, Start: 10, End: 100, SHA256: strings.Repeat("a", 64), Revision: "parser"}}}}}
	captureKnowledge(ctx, result)
	if len(passages) != 1 || passages[0].Source == nil || passages[0].Source.Index != 3 || passages[0].Source.StartByte != 11 || passages[0].Source.EndByte != 16 {
		t.Fatal(passages)
	}
	data, err := json.Marshal(sessions.Message{Role: "assistant", Content: "answer", KnowledgeSources: passages})
	if err != nil {
		t.Fatal(err)
	}
	var saved sessions.Message
	if err = json.Unmarshal(data, &saved); err != nil || saved.KnowledgeSources[0].Source.SHA256 != strings.Repeat("a", 64) {
		t.Fatal("locator lost on reload", err)
	}
	result.Results[0].Chunk.Source = nil
	captureKnowledge(ctx, result)
	if passages[0].Source != nil {
		t.Fatal("fabricated locator for legacy evidence")
	}
}

func TestCapturedKnowledgeKeepsOnlyCitedCellsInSavedHistory(t *testing.T) {
	var passages []sessions.KnowledgePassage
	ctx := withKnowledgeScope(context.Background(), nil, &passages)
	result := &rag.RAGContext{Results: []rag.SearchResult{{DocumentID: "sheet", DocName: "budget.xlsx", Chunk: &rag.Chunk{
		ID: "chunk", Content: "B2: café", StartChar: 11, EndChar: 20,
		Source: &rag.SourceRange{Kind: "sheet", Index: 2, Name: "Budget", Start: 0, End: 40, SHA256: strings.Repeat("b", 64), Revision: "offgrid-xlsx-v3",
			Cells: []rag.SourceCell{{Reference: "A1", Start: 0, End: 10}, {Reference: "B2", Start: 11, End: 20}, {Reference: "C3", Start: 21, End: 40}}},
	}}}}
	captureKnowledge(ctx, result)
	data, err := json.Marshal(sessions.Message{Role: "assistant", Content: "answer", KnowledgeSources: passages})
	if err != nil {
		t.Fatal(err)
	}
	var saved sessions.Message
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	locator := saved.KnowledgeSources[0].Source
	if locator == nil || locator.Name != "Budget" || len(locator.Cells) != 1 || locator.Cells[0] != "B2" {
		t.Fatalf("citation lost or claims unrelated cells: %+v", locator)
	}
}
