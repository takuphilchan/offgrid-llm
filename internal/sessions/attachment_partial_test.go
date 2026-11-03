package sessions

import (
	"errors"
	"strings"
	"testing"
)

func TestAttachmentPartialRequiresExplicitAcceptance(t *testing.T) {
	dir := t.TempDir()
	manager := NewSessionManager(dir)
	scope := manager.WithAccess(Access{UserID: "alice"})
	if _, err := scope.Create("paper", "model"); err != nil {
		t.Fatal(err)
	}
	a, err := scope.CreateAttachment("paper", "paper.pdf", "application/pdf", []byte("original PDF bytes"))
	if err != nil {
		t.Fatal(err)
	}
	failed, err := scope.PrepareAttachment("paper", a.ID, "Native café 日本語", "parser", "document_ocr_required")
	if err != nil || failed.Status != "failed" {
		t.Fatal(failed, err)
	}
	if _, _, err = scope.ReadAttachment("paper", a.ID); !errors.Is(err, ErrAttachmentUnavailable) {
		t.Fatal("partial accepted without consent", err)
	}
	ready, err := scope.PrepareAttachmentTextOnly("paper", a.ID, "Native café 日本語", "parser")
	if err != nil || ready.Status != "ready" || ready.Coverage != "native_text_only" {
		t.Fatal(ready, err)
	}
	scope = NewSessionManager(dir).WithAccess(Access{UserID: "alice"})
	texts, err := scope.AttachmentTexts("paper", []AttachmentReference{{ID: a.ID, Revision: ready.Revision}})
	if err != nil || len(texts) != 1 || texts[0].Text != "Native café 日本語" {
		t.Fatal(texts, err)
	}
	prompt := AttachmentPrompt("Summarize", texts)
	if !strings.Contains(prompt, "native_text_only") || !strings.Contains(prompt, "unread image/scanned content") {
		t.Fatal("coverage lost from inference")
	}
	if _, err = scope.PrepareAttachment("paper", a.ID, "replacement", "parser", ""); !errors.Is(err, ErrTurnConflict) {
		t.Fatal("ready revision overwritten", err)
	}
}
