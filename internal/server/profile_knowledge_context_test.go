package server

import (
	"context"
	"errors"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
)

type profileKnowledgeFixture struct {
	knowledgeFixture
	documents map[string]*rag.Document
	filter    []string
}

func (f *profileKnowledgeFixture) GetDocument(id string) *rag.Document { return f.documents[id] }
func (f *profileKnowledgeFixture) Search(_ context.Context, _ string, opts rag.SearchOptions) (*rag.RAGContext, error) {
	f.filter = opts.DocumentFilter
	return f.result, f.err
}

func TestProfileKnowledgeInventoryIsSelectedNotSimilarityMatched(t *testing.T) {
	for _, hasMatch := range []bool{false, true} {
		config := agents.DefaultAgentConfig()
		config.Profile = &agents.AgentProfile{Settings: agents.ProfileSettings{Knowledge: []string{"a", "b", "private"}}}
		selected := []string{"a", "b"}
		config.KnowledgeSelection = &selected
		source := &profileKnowledgeFixture{knowledgeFixture: knowledgeFixture{enabled: true}, documents: map[string]*rag.Document{
			"a": {ID: "a", Name: "Notes 日本語.txt"}, "b": {ID: "b", Name: "Brief.txt"}, "private": {ID: "private", Name: "Not selected"},
		}}
		if hasMatch {
			source.result = &rag.RAGContext{Results: []rag.SearchResult{{DocumentID: "a"}}, Context: "one matching excerpt"}
		}
		w := httptest.NewRecorder()
		if !attachProfileKnowledge(w, httptest.NewRequest("GET", "/", nil), "what documents are attached?", &config, source) {
			t.Fatal(w.Body.String())
		}
		if !reflect.DeepEqual(source.filter, selected) || len(config.KnowledgeDocuments) != 2 || config.KnowledgeDocuments[1].Name != "Brief.txt" {
			t.Fatalf("lost inventory or leaked restriction: %+v", config.KnowledgeDocuments)
		}
		if (config.KnowledgeContext != "") != hasMatch {
			t.Fatal("confused attachment inventory with retrieved evidence")
		}
	}
}

func TestProfileKnowledgeFailuresStillBlockAdmission(t *testing.T) {
	for _, kind := range []string{"missing source", "missing document", "retrieval error"} {
		config := agents.DefaultAgentConfig()
		config.Profile = &agents.AgentProfile{Settings: agents.ProfileSettings{Knowledge: []string{"a"}}}
		source := &profileKnowledgeFixture{knowledgeFixture: knowledgeFixture{enabled: true}, documents: map[string]*rag.Document{"a": {ID: "a", Name: "Notes"}}}
		var input profileKnowledgeSource = source
		switch kind {
		case "missing source":
			input = nil
		case "missing document":
			delete(source.documents, "a")
		case "retrieval error":
			source.err = errors.New("secret backend detail")
		}
		w := httptest.NewRecorder()
		if attachProfileKnowledge(w, httptest.NewRequest("GET", "/", nil), "question", &config, input) || w.Code < 400 || strings.Contains(w.Body.String(), "secret backend") {
			t.Fatalf("%s: %d %s", kind, w.Code, w.Body.String())
		}
	}
}
