package sessions

import (
	"github.com/takuphilchan/offgrid-llm/internal/document"
	"testing"
)

func TestKnowledgePassagesSurviveJobCommitAndIdempotentReplay(t *testing.T) {
	m := NewSessionManager(t.TempDir())
	s := m.WithAccess(Access{All: true})
	if _, err := s.Create("chat", "model"); err != nil {
		t.Fatal(err)
	}
	turn := &Turn{ID: "turn", RequestHash: "hash", Prompt: "question", Agent: &AgentSelection{ID: "profile", Revision: 1}, KnowledgeDocuments: []string{"a"}, KnowledgeSources: []KnowledgePassage{{DocumentID: "a", Content: "café 日本語"}}}
	turn.KnowledgeSources[0].EvidenceReceipt = &document.EvidenceReceipt{Revision: "immutable-extraction", GroupID: "clause", RequiredChunks: []string{"clause", "exception"}, LinksComplete: false, LimitReached: true}
	if _, _, err := s.BeginJobTurn("chat", turn); err != nil {
		t.Fatal(err)
	}
	if _, err := s.LinkJobTurn("chat", "turn", "hash", "job"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err := s.CommitJobTurn("chat", "turn", "hash", "job", "model", "Answer"); err != nil {
			t.Fatal(err)
		}
	}
	saved, err := s.Load("chat")
	if err != nil || len(saved.Messages) != 2 || len(saved.Messages[1].KnowledgeSources) != 1 || saved.Messages[1].KnowledgeSources[0].Content != "café 日本語" {
		t.Fatalf("evidence lost or replayed: %+v %v", saved, err)
	}
	receipt := saved.Messages[1].KnowledgeSources[0].EvidenceReceipt
	if receipt == nil || receipt.Revision != "immutable-extraction" || receipt.LinksComplete || !receipt.LimitReached || len(receipt.RequiredChunks) != 2 {
		t.Fatal("linked evidence receipt lost", receipt)
	}
}
