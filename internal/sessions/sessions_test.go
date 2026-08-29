package sessions

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSessionManagerRejectsUnsafeNames(t *testing.T) {
	root := t.TempDir()
	manager := NewSessionManager(filepath.Join(root, "sessions"))

	unsafeNames := []string{"", ".", "..", "../data/users", `..\data\users`, "nested/session", "line\nbreak"}
	for _, name := range unsafeNames {
		t.Run(name, func(t *testing.T) {
			err := manager.Save(NewSession(name, "model"))
			if !errors.Is(err, ErrInvalidSessionName) {
				t.Fatalf("Save(%q) error = %v, want ErrInvalidSessionName", name, err)
			}
		})
	}

	if _, err := os.Stat(filepath.Join(root, "data", "users.json")); !os.IsNotExist(err) {
		t.Fatalf("unsafe session path was created: %v", err)
	}
}

func TestSessionManagerSafeNameRoundTrip(t *testing.T) {
	manager := NewSessionManager(t.TempDir())
	session := NewSession("Project notes 2026", "model")
	if err := manager.Save(session); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := manager.Load(session.Name)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Name != session.Name {
		t.Fatalf("loaded name = %q, want %q", loaded.Name, session.Name)
	}
}

func TestAppendExchangePersistsCompleteTurn(t *testing.T) {
	manager := NewSessionManager(t.TempDir())
	if err := manager.Save(NewSession("chat", "model-a")); err != nil {
		t.Fatal(err)
	}
	updated, err := manager.AppendExchange("chat", "model-b", "hello", "hi there")
	if err != nil {
		t.Fatal(err)
	}
	if updated.ModelID != "model-b" || len(updated.Messages) != 2 {
		t.Fatalf("unexpected updated session: %#v", updated)
	}
	loaded, err := manager.Load("chat")
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Messages) != 2 || loaded.Messages[0].Content != "hello" || loaded.Messages[1].Content != "hi there" {
		t.Fatalf("exchange was not persisted: %#v", loaded.Messages)
	}
}
