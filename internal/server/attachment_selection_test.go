package server

import (
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/sessions"
)

func TestDocumentQuestionUsesBoundedSourcesWithoutChangingHistory(t *testing.T) {
	manager := sessions.NewSessionManager(t.TempDir())
	scope := manager.WithAccess(sessions.Access{UserID: "alice"})
	saved, _ := scope.Create("paper", "model")
	text := strings.Repeat("Local renewable energy field observations.\n", 6000) + "\nThe calibration constant is 718.25.\n"
	a, err := scope.CreateAttachment("paper", "paper.txt", "text/plain", []byte(text))
	if err != nil {
		t.Fatal(err)
	}
	a, err = scope.PrepareAttachment("paper", a.ID, text, "test-parser", "")
	if err != nil {
		t.Fatal(err)
	}
	ref := sessions.AttachmentReference{ID: a.ID, Revision: a.Revision}
	saved.Messages = []sessions.Message{{Role: "user", Content: "Explain this source", Attachments: []sessions.AttachmentReference{ref}}, {Role: "assistant", Content: "Earlier answer remains unchanged."}}
	turn := &sessions.Turn{Prompt: "What is the calibration constant?", MaxTokens: 1024, Attachments: []sessions.AttachmentReference{ref}}
	messages, err := attachmentMessages(scope, "paper", saved, turn, 8192)
	if err != nil {
		t.Fatal(err)
	}
	last := messages[len(messages)-1].StringContent()
	if !strings.Contains(last, "718.25") || !strings.Contains(last, `"selection":"excerpts"`) || len(last) > 8192-1024 {
		t.Fatal("source retrieval failed", len(last), last)
	}
	if strings.Count(last, `"id":"`+a.ID+`"`) != 1 {
		t.Fatal("history duplicated source")
	}
	if messages[0].StringContent() != saved.Messages[0].Content || messages[1].StringContent() != saved.Messages[1].Content {
		t.Fatal("history rewritten")
	}
	_, original, err := scope.ReadAttachment("paper", a.ID)
	if err != nil || original != text {
		t.Fatal("source changed")
	}
	if _, err = attachmentMessages(scope, "paper", saved, turn, 1536); err == nil {
		t.Fatal("impossible budget accepted")
	}
}
