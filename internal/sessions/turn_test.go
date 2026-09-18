package sessions

import (
	"errors"
	"testing"
)

func TestDurableTurnAdmissionCompletionAndRecovery(t *testing.T) {
	dir := t.TempDir()
	manager := NewSessionManager(dir)
	scope := manager.WithAccess(Access{UserID: "alice"})
	if _, err := scope.Create("chat", "model"); err != nil {
		t.Fatal(err)
	}
	turn := &Turn{ID: "request-1", RequestHash: "hash", Prompt: "hello", Model: "model", Status: "pending", Phase: "queued"}
	if _, fresh, err := scope.BeginTurn("chat", turn); err != nil || !fresh {
		t.Fatal(fresh, err)
	}
	if _, fresh, err := scope.BeginTurn("chat", turn); err != nil || fresh {
		t.Fatal("retry duplicated", fresh, err)
	}
	conflicting := *turn
	conflicting.RequestHash = "different"
	if _, _, err := scope.BeginTurn("chat", &conflicting); !errors.Is(err, ErrTurnConflict) {
		t.Fatal("conflicting retry", err)
	}
	if _, _, err := manager.WithAccess(Access{UserID: "bob"}).BeginTurn("chat", turn); !errors.Is(err, ErrSessionNotFound) {
		t.Fatal("ownership bypass", err)
	}
	turn.Status = "running"
	turn.Output = "partial"
	if err := scope.SaveTurn("chat", turn); err != nil {
		t.Fatal(err)
	}
	restarted := NewSessionManager(dir)
	if err := restarted.RecoverTurns(); err != nil {
		t.Fatal(err)
	}
	saved, err := scope.Load("chat")
	if err != nil || saved.Turn.Status != "interrupted" || saved.Turn.Output != "partial" || len(saved.Messages) != 0 {
		t.Fatal(saved, err)
	}
	next := &Turn{ID: "request-2", RequestHash: "hash2", Prompt: "hello", Model: "model", Status: "pending"}
	if _, _, err := scope.BeginTurn("chat", next); err != nil {
		t.Fatal(err)
	}
	next.Status = "completed"
	next.Output = "answer"
	if err := scope.SaveTurn("chat", next); err != nil {
		t.Fatal(err)
	}
	if err := scope.SaveTurn("chat", next); !errors.Is(err, ErrTurnConflict) {
		t.Fatal("duplicate commit", err)
	}
	saved, _ = scope.Load("chat")
	if len(saved.Messages) != 2 || saved.Turn.Status != "completed" {
		t.Fatal(saved)
	}
	if _, _, err := scope.BeginTurn("chat", turn); !errors.Is(err, ErrTurnConflict) {
		t.Fatal("superseded replay", err)
	}
}
