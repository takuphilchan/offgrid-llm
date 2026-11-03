package sessions

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestAttachmentAdmissionAndRemovalAreSerialized(t *testing.T) {
	for i := 0; i < 20; i++ {
		manager := NewSessionManager(t.TempDir())
		scope := manager.WithAccess(Access{UserID: "alice"})
		_, _ = scope.Create("chat", "model")
		a, _ := scope.CreateAttachment("chat", "note.txt", "text/plain", []byte("source"))
		a, _ = scope.PrepareAttachment("chat", a.ID, "source", "text-v1", "")
		turn := &Turn{ID: "test", Prompt: "read", Model: "model", Status: "pending", Attachments: []AttachmentReference{{ID: a.ID, Revision: a.Revision}}}
		var admitted, removed error
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); _, _, admitted = scope.BeginTurn("chat", turn) }()
		go func() { defer wg.Done(); removed = scope.DeleteAttachment("chat", a.ID) }()
		wg.Wait()
		if admitted == nil && removed == nil {
			t.Fatal("admitted a source concurrently deleted")
		}
		if admitted != nil && !errors.Is(admitted, ErrAttachmentUnavailable) {
			t.Fatalf("unexpected admission error: %v", admitted)
		}
		if removed != nil && !errors.Is(removed, ErrAttachmentInUse) {
			t.Fatalf("unexpected removal error: %v", removed)
		}
	}
}

func TestAttachmentContextRejectsDuplicatesAndOversizeWithoutTruncation(t *testing.T) {
	scope := NewSessionManager(t.TempDir()).WithAccess(Access{UserID: "alice"})
	_, _ = scope.Create("chat", "model")
	a, _ := scope.CreateAttachment("chat", "note.txt", "text/plain", []byte("source"))
	a, _ = scope.PrepareAttachment("chat", a.ID, strings.Repeat("a", MaxAttachmentContextBytes+1), "text-v1", "")
	refs := []AttachmentReference{{ID: a.ID, Revision: a.Revision}}
	if _, err := scope.AttachmentTexts("chat", refs); !errors.Is(err, ErrAttachmentContextLimit) {
		t.Fatalf("silently truncated: %v", err)
	}
	b, _ := scope.CreateAttachment("chat", "small.txt", "text/plain", []byte("small"))
	b, _ = scope.PrepareAttachment("chat", b.ID, "small", "text-v1", "")
	ref := AttachmentReference{ID: b.ID, Revision: b.Revision}
	if _, err := scope.AttachmentTexts("chat", []AttachmentReference{ref, ref}); !errors.Is(err, ErrAttachmentUnavailable) {
		t.Fatal("duplicate accepted")
	}
}

func TestAttachmentInventoryDoesNotHideUnsafeNamespace(t *testing.T) {
	directory := t.TempDir()
	manager := NewSessionManager(directory)
	scope := manager.WithAccess(Access{All: true})
	_, _ = scope.Create("chat", "model")
	if _, err := scope.ListAttachments("chat"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(manager.sessionsDir, ".attachments"), []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := scope.ListAttachments("chat"); !errors.Is(err, ErrAttachmentUnavailable) {
		t.Fatalf("unsafe source storage hidden as empty: %v", err)
	}
}

func TestAttachmentAdmissionDeduplicatesHistoryForRetrieval(t *testing.T) {
	manager := NewSessionManager(t.TempDir())
	scope := manager.WithAccess(Access{UserID: "alice"})
	_, _ = scope.Create("chat", "model")
	a, _ := scope.CreateAttachment("chat", "note.txt", "text/plain", []byte("source"))
	a, _ = scope.PrepareAttachment("chat", a.ID, strings.Repeat("a", MaxAttachmentContextBytes/2+1), "text-v1", "")
	ref := AttachmentReference{ID: a.ID, Revision: a.Revision}
	saved, _ := scope.Load("chat")
	saved.Messages = []Message{{Role: "user", Content: "first", Attachments: []AttachmentReference{ref}}, {Role: "assistant", Content: "answer"}}
	if err := manager.Save(saved); err != nil {
		t.Fatal(err)
	}
	_, _, err := scope.BeginTurn("chat", &Turn{ID: "next", Status: "pending", Attachments: []AttachmentReference{ref}})
	if err != nil {
		t.Fatalf("same source should be admitted once for bounded retrieval: %v", err)
	}
	current, _ := scope.Load("chat")
	if current.Turn == nil || !current.TurnRequests["next"] {
		t.Fatal("valid request not durably admitted")
	}
}

func TestAttachmentQuotaAggregatesOwnerConversations(t *testing.T) {
	manager := NewSessionManager(t.TempDir())
	alice := manager.WithAccess(Access{UserID: "alice"})
	bob := manager.WithAccess(Access{UserID: "bob"})
	_, _ = alice.Create("one", "model")
	_, _ = alice.Create("two", "model")
	_, _ = bob.Create("other", "model")
	_, _ = alice.CreateAttachment("one", "note.txt", "text/plain", []byte("1234"))
	_, _ = bob.CreateAttachment("other", "note.txt", "text/plain", []byte("12345678"))
	if err := alice.checkAttachmentActorQuota("two", 4, 8); err != nil {
		t.Fatalf("another owner counted: %v", err)
	}
	if err := alice.checkAttachmentActorQuota("two", 5, 8); !errors.Is(err, ErrAttachmentLimit) {
		t.Fatalf("owner quota bypassed with a new conversation: %v", err)
	}
}
