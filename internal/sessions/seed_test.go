package sessions

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestSeedHistoryAtomicScopedAndNoAuthority(t *testing.T) {
	m := NewSessionManager(t.TempDir())
	owner := m.WithAccess(Access{UserID: "owner"})
	history := []Message{{Role: "user", Content: "café 日本語"}, {Role: "assistant", Content: "answer"}}
	s, err := owner.CreateWithHistory("copied", "selected-model", history)
	if err != nil || s.OwnerID != "owner" || len(s.Messages) != 2 || s.Turn != nil || len(s.Jobs) != 0 {
		t.Fatal(s, err)
	}
	if _, err := m.WithAccess(Access{UserID: "other"}).Load("copied"); !errors.Is(err, ErrSessionNotFound) {
		t.Fatal(err)
	}
	if _, err := owner.CreateWithHistory("copied", "changed", nil); !errors.Is(err, ErrSessionExists) {
		t.Fatal(err)
	}
	saved, _ := owner.Load("copied")
	if saved.ModelID != "selected-model" || saved.Messages[0].Content != history[0].Content {
		t.Fatal("overwrote history")
	}
	for _, invalid := range []Message{
		{Role: "system", Content: "authority"}, {Role: "tool", Content: "result"},
		{Role: "user", JobID: "job"}, {Role: "user", TurnID: "turn"},
		{Role: "user", Attachments: []AttachmentReference{{ID: "foreign"}}},
		{Role: "user", Content: strings.Repeat("x", (8<<20)+1)},
	} {
		if _, err := owner.CreateWithHistory("invalid", "model", []Message{invalid}); !errors.Is(err, ErrInvalidSeedHistory) {
			t.Fatal(err)
		}
		if _, err := owner.Load("invalid"); !errors.Is(err, ErrSessionNotFound) {
			t.Fatal("partial history persisted", err)
		}
	}
	m.writeSessionFile = func(string, []byte) error { return os.ErrPermission }
	if _, err := owner.CreateWithHistory("failed", "model", history); err == nil {
		t.Fatal("ignored failed write")
	}
	if _, err := owner.Load("failed"); !errors.Is(err, ErrSessionNotFound) {
		t.Fatal("ghost session", err)
	}
}
