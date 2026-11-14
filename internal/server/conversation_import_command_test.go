package server

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
)

func TestPublicImportCommandRetainsSourceAndRetriesWithoutSourceRead(t *testing.T) {
	f := newConversationFixture(t)
	f.s.config.EnableConversationPreview = true
	f.s.bindCanonicalConversationGuard()
	path := filepath.Join(f.directory, "chat.json")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	command := agents.ConversationCommand{RequestID: "import-chat", Kind: "import_conversation", Payload: agents.ConversationCommandPayload{LegacyName: "chat"}}
	w := controlHTTP(f.s, "POST", "/api/v2/conversations", command)
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	var receipt agents.ConversationCommandReceipt
	if err = json.Unmarshal(w.Body.Bytes(), &receipt); err != nil || receipt.Import == nil || receipt.Import.Authority != "agent_control_store" {
		t.Fatal(receipt, err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(source, after) {
		t.Fatal("source changed", err)
	}
	// Removing only the fixture's reader proves accepted retries are independent
	// of the compatibility store and cannot start another migration/executor.
	f.s.sessionHandlers = nil
	f.s.config.EnableConversationPreview = false
	w = controlHTTP(f.s, "POST", "/api/v2/conversations", command)
	if w.Code != 200 || f.calls.Load() != 0 {
		t.Fatal(w.Code, w.Body.String(), f.calls.Load())
	}
	command.Payload.LegacyName = "different"
	if w = controlHTTP(f.s, "POST", "/api/v2/conversations", command); w.Code != 409 {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestPublicImportCommandReportsMissingAndBusySources(t *testing.T) {
	f := newConversationFixture(t)
	f.s.config.EnableConversationPreview = true
	command := agents.ConversationCommand{RequestID: "missing-import", Kind: "import_conversation", Payload: agents.ConversationCommandPayload{LegacyName: "missing"}}
	if w := controlHTTP(f.s, "POST", "/api/v2/conversations", command); w.Code != 404 {
		t.Fatal(w.Code, w.Body.String())
	}
	scoped := f.s.sessionHandlers.manager.WithAccess(sessions.Access{All: true})
	if _, _, err := scoped.BeginTurn("chat", &sessions.Turn{ID: "active", RequestHash: "active", Model: "model", Prompt: "Current work", Status: "running"}); err != nil {
		t.Fatal(err)
	}
	command.RequestID, command.Payload.LegacyName = "busy-import", "chat"
	if w := controlHTTP(f.s, "POST", "/api/v2/conversations", command); w.Code != 409 {
		t.Fatal(w.Code, w.Body.String())
	}
}
