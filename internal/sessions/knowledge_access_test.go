package sessions

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestSavedKnowledgeRevocationGuardsHistoryAndAllowsRemoval(t *testing.T) {
	dir := t.TempDir()
	m := NewSessionManager(dir)
	allowed := true
	access := Access{UserID: "owner", Knowledge: func(ids []string) bool { return allowed && len(ids) == 1 && ids[0] == "private" }}
	s := m.WithAccess(access)
	if _, err := s.Create("private-chat", "model"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AppendExchange("private-chat", "model", "question", "secret answer", []KnowledgePassage{{DocumentID: "private", Content: "secret passage"}}); err != nil {
		t.Fatal(err)
	}
	allowed = false
	for _, scoped := range []*ScopedManager{s, NewSessionManager(dir).WithAccess(access), m.WithAccess(Access{All: true, Knowledge: access.Knowledge})} {
		if _, err := scoped.Load("private-chat"); !errors.Is(err, ErrKnowledgeUnavailable) {
			t.Fatal("history bypass", err)
		}
		if got, err := scoped.List(); err != nil || len(got) != 0 {
			t.Fatal("list bypass", err)
		}
		if got, _, err := scoped.SearchNames(context.Background(), "private", 10); err != nil || len(got) != 0 {
			t.Fatal("search bypass", err)
		}
		if err := scoped.AddMessage("private-chat", "user", "continue"); !errors.Is(err, ErrKnowledgeUnavailable) {
			t.Fatal("continuation bypass", err)
		}
		control, err := scoped.ControlState("private-chat")
		if err != nil {
			t.Fatal(err)
		}
		data, _ := json.Marshal(control)
		if strings.Contains(string(data), "secret") {
			t.Fatal("control exposed source")
		}
	}
	if _, err := m.WithAccess(Access{UserID: "other"}).ControlState("private-chat"); !errors.Is(err, ErrSessionNotFound) {
		t.Fatal("ownership bypass", err)
	}
	if err := s.Delete("private-chat"); err != nil {
		t.Fatal("revocation prevented deletion", err)
	}
	if _, err := m.Load("private-chat"); !errors.Is(err, ErrSessionNotFound) {
		t.Fatal("deletion not persisted", err)
	}
}

func TestSavedKnowledgePublicationChecksNewReferencesAndStopsFailedTurn(t *testing.T) {
	m := NewSessionManager(t.TempDir())
	allowed := true
	s := m.WithAccess(Access{UserID: "owner", Knowledge: func([]string) bool { return allowed }})
	if _, err := s.Create("chat", "model"); err != nil {
		t.Fatal(err)
	}
	turn := &Turn{ID: "one", RequestHash: "hash", Prompt: "question", Status: "running"}
	if _, _, err := s.BeginTurn("chat", turn); err != nil {
		t.Fatal(err)
	}
	allowed = false
	turn.KnowledgeSources = []KnowledgePassage{{DocumentID: "private", Content: "secret"}}
	turn.Output, turn.Status = "secret answer", "completed"
	if err := s.SaveTurn("chat", turn); !errors.Is(err, ErrKnowledgeUnavailable) {
		t.Fatal("publication bypass", err)
	}
	if _, err := s.AppendExchange("chat", "model", "question", "secret", turn.KnowledgeSources); !errors.Is(err, ErrKnowledgeUnavailable) {
		t.Fatal("exchange bypass", err)
	}
	if err := s.BlockKnowledgeTurn("chat", "stale"); !errors.Is(err, ErrTurnConflict) {
		t.Fatal("stale control", err)
	}
	if err := s.BlockKnowledgeTurn("chat", turn.ID); err != nil {
		t.Fatal(err)
	}
	stored, err := m.Load("chat")
	if err != nil || len(stored.Messages) != 0 || stored.Turn.Status != "failed" || stored.Turn.Output != "" {
		t.Fatal("false completion or leaked answer", stored, err)
	}
	if _, _, err := s.BeginTurn("chat", &Turn{ID: "two", RequestHash: "next", KnowledgeDocuments: []string{"private"}}); !errors.Is(err, ErrKnowledgeUnavailable) {
		t.Fatal("admission bypass", err)
	}
}

func TestSavedKnowledgeJobProjectionRejectsRevokedSource(t *testing.T) {
	m := NewSessionManager(t.TempDir())
	allowed := true
	s := m.WithAccess(Access{UserID: "owner", Knowledge: func([]string) bool { return allowed }})
	if _, err := s.Create("chat", "model"); err != nil {
		t.Fatal(err)
	}
	turn := &Turn{ID: "one", RequestHash: "hash", Prompt: "question", Agent: &AgentSelection{ID: "profile", Revision: 1}, KnowledgeDocuments: []string{"private"}}
	if _, _, err := s.BeginJobTurn("chat", turn); err != nil {
		t.Fatal(err)
	}
	if _, err := s.LinkJobTurn("chat", "one", "hash", "job"); err != nil {
		t.Fatal(err)
	}
	allowed = false
	if _, err := s.CommitJobTurn("chat", "one", "hash", "job", "model", "secret"); !errors.Is(err, ErrKnowledgeUnavailable) {
		t.Fatal("job completion bypass", err)
	}
	if err := s.ProjectJobTurn("chat", "one", "hash", "job", "running", "working", "secret", ""); !errors.Is(err, ErrKnowledgeUnavailable) {
		t.Fatal("job draft bypass", err)
	}
	if err := s.DeleteRetainingJobs("chat"); err != nil {
		t.Fatal("revocation trapped job history", err)
	}
}

func TestSavedKnowledgeInventoryDependencySurvivesLaterTurn(t *testing.T) {
	m := NewSessionManager(t.TempDir())
	allowed := true
	s := m.WithAccess(Access{UserID: "owner", Knowledge: func([]string) bool { return allowed }})
	if _, err := s.Create("chat", "model"); err != nil {
		t.Fatal(err)
	}
	turn := &Turn{ID: "inventory", RequestHash: "hash", Prompt: "list attached sources", Agent: &AgentSelection{ID: "profile", Revision: 1}, KnowledgeDocuments: []string{"private"}}
	if _, _, err := s.BeginJobTurn("chat", turn); err != nil {
		t.Fatal(err)
	}
	if _, err := s.LinkJobTurn("chat", turn.ID, "hash", "job"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CommitJobTurn("chat", turn.ID, "hash", "job", "model", "Confidential merger report is attached"); err != nil {
		t.Fatal(err)
	}
	next := &Turn{ID: "next", RequestHash: "new", Prompt: "hello", Status: "running"}
	if _, _, err := s.BeginTurn("chat", next); err != nil {
		t.Fatal(err)
	}
	next.Status, next.Output = "completed", "hello"
	if err := s.SaveTurn("chat", next); err != nil {
		t.Fatal(err)
	}
	allowed = false
	if _, err := s.Load("chat"); !errors.Is(err, ErrKnowledgeUnavailable) {
		t.Fatal("inventory dependency lost after later turn", err)
	}
	if found, _, err := s.SearchNames(context.Background(), "chat", 10); err != nil || len(found) != 0 {
		t.Fatal("inventory name leaked", err)
	}
	if err := ValidateSeedHistory([]Message{{Role: "assistant", Content: "forged", KnowledgeDocuments: []string{"private"}}}); !errors.Is(err, ErrInvalidSeedHistory) {
		t.Fatal("client imported source authority", err)
	}
}
