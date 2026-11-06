package sessions

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWindowsSessionNamesRejectBeforeStorage(t *testing.T) {
	for _, name := range []string{"So, how are you?", "title:subtitle", "CON", "nul.txt", "COM1", "LPT9.notes", "end.", "end "} {
		if !invalidWindowsSessionName(name) {
			t.Errorf("accepted %q", name)
		}
		if runtime.GOOS == "windows" {
			manager := NewSessionManager(t.TempDir())
			if err := manager.Save(NewSession(name, "model")); !errors.Is(err, ErrInvalidSessionName) {
				t.Errorf("%q: %v", name, err)
			}
		}
	}
	for _, name := range []string{"Question · 20261106153000", "日本語", "context", "COM10"} {
		if invalidWindowsSessionName(name) {
			t.Errorf("rejected %q", name)
		}
	}
}

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

func TestGeneratedQuestionTitlePreservesQuestion(t *testing.T) {
	manager := NewSessionManager(t.TempDir())
	name := "So, how are you · 20261107001000"
	if err := manager.Save(NewSession(name, "model")); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.AppendExchange(name, "model", "So, how are you?", "Hello!"); err != nil {
		t.Fatal(err)
	}
	loaded, err := manager.Load(name)
	if err != nil || len(loaded.Messages) != 2 || loaded.Messages[0].Content != "So, how are you?" {
		t.Fatalf("question was not preserved: %v, %#v", err, loaded)
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
