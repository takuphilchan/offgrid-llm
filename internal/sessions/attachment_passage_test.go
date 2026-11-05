package sessions

import (
	"strings"
	"testing"
)

func TestAttachmentPassagePinsOwnerRevisionAndUTF8(t *testing.T) {
	manager := NewSessionManager(t.TempDir())
	alice := manager.WithAccess(Access{UserID: "alice"})
	_, _ = alice.Create("chat", "model")
	content := "Before café 日本語 after"
	a, err := alice.CreateAttachment("chat", "notes.txt", "text/plain", []byte(content))
	if err != nil {
		t.Fatal(err)
	}
	a, err = alice.PrepareAttachment("chat", a.ID, content, "text-v1", "")
	if err != nil {
		t.Fatal(err)
	}
	ref := AttachmentReference{ID: a.ID, Revision: a.Revision}
	start, end := len("Before "), len("Before café 日本語")
	p, err := alice.ReadAttachmentPassage("chat", ref, start, end)
	if err != nil || p.Text != "café 日本語" || p.SHA256 != a.SHA256 || p.Revision != a.Revision {
		t.Fatalf("passage: %+v %v", p, err)
	}
	for _, bounds := range [][2]int{{-1, end}, {start, start}, {0, len(content) + 1}, {0, MaxAttachmentContextBytes + 1}, {start, end - 1}, {10, 11}} {
		if _, err := alice.ReadAttachmentPassage("chat", ref, bounds[0], bounds[1]); err == nil {
			t.Fatalf("accepted invalid range %v", bounds)
		}
	}
	if _, err := manager.WithAccess(Access{UserID: "bob"}).ReadAttachmentPassage("chat", ref, start, end); err == nil {
		t.Fatal("cross-owner source exposed")
	}
	if _, err := alice.ReadAttachmentPassage("chat", AttachmentReference{ID: a.ID, Revision: strings.Repeat("0", 64)}, start, end); err == nil {
		t.Fatal("stale revision exposed")
	}
	if err := alice.DeleteAttachment("chat", a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := alice.ReadAttachmentPassage("chat", ref, start, end); err == nil {
		t.Fatal("deleted source exposed")
	}
}

func TestAttachmentPromptLabelsWithoutChangingSelection(t *testing.T) {
	sources := []AttachmentText{{Text: "whole"}, {Selection: "excerpts", Excerpts: []AttachmentExcerpt{{StartByte: 4, EndByte: 8, Text: "part"}}}}
	prompt := AttachmentPrompt("Question", sources)
	for _, want := range []string{`"citation":"S1"`, `"citation":"S2.1"`, "Do not print byte offsets", "untrusted content"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("missing %s", want)
		}
	}
	if sources[0].Citation != "" || sources[1].Excerpts[0].Citation != "" {
		t.Fatal("modified retained source selection")
	}
}
