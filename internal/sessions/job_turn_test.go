package sessions

import (
	"errors"
	"sync"
	"testing"
)

func TestJobTurnLinkAndFinalCommitAreAtomicAndIdempotent(t *testing.T) {
	dir := t.TempDir()
	m := NewSessionManager(dir)
	scope := m.WithAccess(Access{UserID: "alice"})
	if _, err := scope.Create("chat", "model"); err != nil {
		t.Fatal(err)
	}
	intent := &Turn{ID: "turn-1", RequestHash: "hash", Prompt: "hello", Model: "model", Agent: &AgentSelection{ID: "agent-1", Revision: 2}}
	if _, fresh, err := scope.BeginJobTurn("chat", intent); err != nil || !fresh {
		t.Fatal(fresh, err)
	}
	if intent.JobAdmission != "" {
		t.Fatal("caller draft mutated")
	}
	if _, fresh, err := scope.BeginJobTurn("chat", intent); err != nil || fresh {
		t.Fatal("duplicate admission", err)
	}
	if _, err := scope.LinkJobTurn("chat", intent.ID, "changed", "job-1"); !errors.Is(err, ErrTurnConflict) {
		t.Fatal("changed digest accepted", err)
	}
	if _, err := m.WithAccess(Access{UserID: "bob"}).LinkJobTurn("chat", intent.ID, "hash", "job-1"); !errors.Is(err, ErrSessionNotFound) {
		t.Fatal("cross-actor link", err)
	}
	if _, err := scope.LinkJobTurn("chat", intent.ID, "hash", "job-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := scope.LinkJobTurn("chat", intent.ID, "hash", "job-2"); !errors.Is(err, ErrTurnConflict) {
		t.Fatal("replaced canonical job", err)
	}
	// Restart interrupts observation, never discards accepted identity/linkage.
	m = NewSessionManager(dir)
	if err := m.RecoverTurns(); err != nil {
		t.Fatal(err)
	}
	scope = m.WithAccess(Access{UserID: "alice"})
	saved, _ := scope.Load("chat")
	if saved.Turn.JobID != "job-1" || !saved.Turn.Active() {
		t.Fatal("restart lost unresolved job", saved.Turn)
	}
	if _, _, err := scope.BeginTurn("chat", &Turn{ID: "turn-2", Status: "pending"}); !errors.Is(err, ErrTurnConflict) {
		t.Fatal("unresolved work replaced", err)
	}
	if err := scope.SaveTurn("chat", &Turn{ID: intent.ID, Status: "completed", Output: "bypass"}); !errors.Is(err, ErrTurnConflict) {
		t.Fatal("plain Chat committed linked work", err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := scope.CommitJobTurn("chat", intent.ID, "hash", "job-1", "model", "Hello"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	saved, _ = scope.Load("chat")
	if len(saved.Messages) != 2 || saved.Turn.Status != "completed" {
		t.Fatal("duplicate final response", saved)
	}
	if _, err := scope.CommitJobTurn("chat", intent.ID, "hash", "job-1", "model", "Changed"); !errors.Is(err, ErrTurnConflict) {
		t.Fatal("changed completed response", err)
	}
}

func TestJobTurnWriteFailureKeepsFinalRecoverable(t *testing.T) {
	dir := t.TempDir()
	m := NewSessionManager(dir)
	scope := m.WithAccess(Access{UserID: "alice"})
	if _, err := scope.Create("chat", "model"); err != nil {
		t.Fatal(err)
	}
	intent := &Turn{ID: "turn", RequestHash: "hash", Prompt: "hello", Agent: &AgentSelection{ID: "agent", Revision: 1}}
	if _, _, err := scope.BeginJobTurn("chat", intent); err != nil {
		t.Fatal(err)
	}
	if _, err := scope.LinkJobTurn("chat", "turn", "hash", "job"); err != nil {
		t.Fatal(err)
	}
	// Fail after loading and composing the final exchange, at the actual
	// atomic-write boundary. Neither memory nor disk may report completion.
	writes := 0
	m.writeSessionFile = func(string, []byte) error { writes++; return errors.New("fixture disk full") }
	if _, err := scope.CommitJobTurn("chat", "turn", "hash", "job", "model", "Hello"); err == nil {
		t.Fatal("storage outage acknowledged")
	}
	m.writeSessionFile = nil
	if writes != 1 {
		t.Fatal("fault did not reach final persistence")
	}
	saved, _ := scope.Load("chat")
	if len(saved.Messages) != 0 || saved.Turn.Status == "completed" {
		t.Fatal("failed commit changed history")
	}
	if _, err := scope.CommitJobTurn("chat", "turn", "hash", "job", "model", "Hello"); err != nil {
		t.Fatal(err)
	}
}
