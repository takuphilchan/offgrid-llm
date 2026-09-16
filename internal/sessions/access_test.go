package sessions

import (
	"errors"
	"sync"
	"testing"
)

func TestScopedSessionsEnforceOwnershipOnEveryOperation(t *testing.T) {
	manager := NewSessionManager(t.TempDir())
	alice := manager.WithAccess(Access{UserID: "alice"})
	bob := manager.WithAccess(Access{UserID: "bob"})
	created, err := alice.Create("private", "model")
	if err != nil || created.OwnerID != "alice" {
		t.Fatalf("create: %+v %v", created, err)
	}
	if err := alice.AddMessage("private", "user", "private content"); err != nil {
		t.Fatal(err)
	}
	for name, operation := range map[string]func() error{
		"load":     func() error { _, err := bob.Load("private"); return err },
		"delete":   func() error { return bob.Delete("private") },
		"append":   func() error { return bob.AddMessage("private", "user", "overwrite") },
		"exchange": func() error { _, err := bob.AppendExchange("private", "model", "q", "a"); return err },
	} {
		t.Run(name, func(t *testing.T) {
			if err := operation(); !errors.Is(err, ErrSessionNotFound) {
				t.Fatalf("got %v", err)
			}
		})
	}
	list, err := bob.List()
	if err != nil || len(list) != 0 {
		t.Fatalf("other owner's list: %+v %v", list, err)
	}
	if _, err := bob.Create("private", "different"); !errors.Is(err, ErrSessionExists) {
		t.Fatalf("overwrite: %v", err)
	}
	stored, err := alice.Load("private")
	if err != nil || len(stored.Messages) != 1 || stored.Messages[0].Content != "private content" {
		t.Fatalf("unauthorized mutation: %+v %v", stored, err)
	}
	// Owner metadata must survive reopening storage.
	reopened := NewSessionManager(manager.sessionsDir).WithAccess(Access{UserID: "alice"})
	if session, err := reopened.Load("private"); err != nil || session.OwnerID != "alice" {
		t.Fatalf("reopen: %+v %v", session, err)
	}
}

func TestLegacySessionsRequireExplicitAllAccess(t *testing.T) {
	manager := NewSessionManager(t.TempDir())
	if err := manager.Save(NewSession("legacy", "model")); err != nil {
		t.Fatal(err)
	}
	ordinary := manager.WithAccess(Access{UserID: "alice"})
	if _, err := ordinary.Load("legacy"); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("legacy was claimed: %v", err)
	}
	for _, access := range []Access{{All: true}, {UserID: "admin", All: true}} {
		privileged := manager.WithAccess(access)
		if list, err := privileged.List(); err != nil || len(list) != 1 {
			t.Fatalf("list: %+v %v", list, err)
		}
		if _, err := privileged.AppendExchange("legacy", "model", "q", "a"); err != nil {
			t.Fatal(err)
		}
	}
	if session, err := manager.Load("legacy"); err != nil || session.OwnerID != "" {
		t.Fatalf("owner changed: %+v %v", session, err)
	}
	if _, err := manager.WithAccess(Access{}).Create("anonymous", "model"); !errors.Is(err, ErrAccessDenied) {
		t.Fatalf("zero access: %v", err)
	}
}

func TestScopedSessionCreationIsExclusiveAndAppendsAreAtomic(t *testing.T) {
	manager := NewSessionManager(t.TempDir())
	const workers = 12
	results := make(chan error, workers)
	var group sync.WaitGroup
	for i := 0; i < workers; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			_, err := manager.WithAccess(Access{UserID: "alice"}).Create("shared", "model")
			results <- err
		}()
	}
	group.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		} else if !errors.Is(err, ErrSessionExists) {
			t.Fatal(err)
		}
	}
	if success != 1 {
		t.Fatalf("created %d times", success)
	}
	for i := 0; i < workers; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			if err := manager.WithAccess(Access{UserID: "alice"}).AddMessage("shared", "user", "message"); err != nil {
				t.Error(err)
			}
		}()
	}
	group.Wait()
	stored, err := manager.Load("shared")
	if err != nil || len(stored.Messages) != workers {
		t.Fatalf("lost messages: %+v %v", stored, err)
	}
}
