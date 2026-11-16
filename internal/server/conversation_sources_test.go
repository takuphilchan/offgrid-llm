package server

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCanonicalSourceAdmissionRetainsEvidenceAndLegacyGuard(t *testing.T) {
	f := newConversationFixture(t)
	f.s.config.EnableConversationPreview = true
	f.s.bindCanonicalConversationGuard()
	r := httptest.NewRequest("POST", "/api/v2/conversations/import", nil)
	manifest, err := f.s.migrateConversation(r, "chat")
	if err != nil {
		t.Fatal(err)
	}
	// Uploading an immutable asset is legal after migration; executing a legacy
	// transcript remains prohibited.
	w := uploadChatFile(t, f.s.sessionHandlers, "policy.txt", "Policy exception: shipment 12345 is exempt.")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var uploaded struct {
		Attachment sessions.Attachment `json:"attachment"`
	}
	if err = json.Unmarshal(w.Body.Bytes(), &uploaded); err != nil {
		t.Fatal(err)
	}
	c, err := f.s.agentManager.Conversation(f.s.controlScope(r), manifest.ConversationID)
	if err != nil {
		t.Fatal(err)
	}
	input := api.ConversationInboxInput{Content: "Which shipment is exempt?", Model: "model", Origin: "typed", SourceSession: "chat", Attachments: []api.ConversationAttachment{{ID: uploaded.Attachment.ID, Revision: uploaded.Attachment.Revision}}}
	cfg := agents.AgentConfig{ContextWindow: 8192, MaxTokens: 1024}
	if err = f.s.attachConversationInputSources(r, c, input, &cfg); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cfg.InputEvidence, "12345") || len(cfg.AttachmentSelections) == 0 || cfg.InputAttachments[0].Revision != uploaded.Attachment.Revision {
		t.Fatal("lost source or receipt", cfg.InputEvidence)
	}
	command := api.ConversationInputCommand{RequestID: "typed-source-request", TargetID: c.ID, ExpectedRevision: c.Revision, Kind: "submit_input", Payload: input}
	admitted := controlHTTP(f.s, "POST", "/api/v2/conversations/"+c.ID+"/inputs", command)
	if admitted.Code != 202 {
		t.Fatal(admitted.Code, admitted.Body.String())
	}
	retry := controlHTTP(f.s, "POST", "/api/v2/conversations/"+c.ID+"/inputs", command)
	if retry.Code != 200 {
		t.Fatal("retry lost receipt", retry.Code, retry.Body.String())
	}
	var receipt api.ConversationCommandReceipt
	if err = json.Unmarshal(admitted.Body.Bytes(), &receipt); err != nil {
		t.Fatal(err)
	}
	prepared, err := f.s.prepareConversationTurn(context.Background(), f.s.controlScope(r), c.ID, receipt.Input.ID)
	if err != nil {
		t.Fatal("source preparation", err)
	}
	last := prepared.Checkpoint.Messages[len(prepared.Checkpoint.Messages)-1]
	if !strings.Contains(last.StringContent(), "12345") || prepared.Prompt != input.Content {
		t.Fatal("evidence missing from measured model context")
	}
	restarted := agents.NewManagerWithPersistence(nil, nil, nil, f.agentDirectory)
	retained, found := restarted.GetTask(prepared.ID)
	if !found || retained.Config.InputAttachments[0].Revision != uploaded.Attachment.Revision || string(retained.Config.AttachmentSelections) != string(prepared.Config.AttachmentSelections) {
		t.Fatal("restart lost source snapshot")
	}
	bad := input
	bad.SourceSession = "another-session"
	if err = f.s.validateConversationSources(r, c, bad); !errors.Is(err, agents.ErrControlNotFound) {
		t.Fatal("cross-session reference accepted", err)
	}
	bad = input
	bad.Attachments = append([]api.ConversationAttachment(nil), input.Attachments...)
	bad.Attachments[0].Revision = strings.Repeat("0", 64)
	if err = f.s.validateConversationSources(r, c, bad); err == nil {
		t.Fatal("stale revision accepted")
	}
	bad = input
	bad.Attachments = append([]api.ConversationAttachment(nil), input.Attachments...)
	bad.Attachments[0].Page = 99
	if err = f.s.validateConversationSources(r, c, bad); err == nil {
		t.Fatal("nonexistent page accepted")
	}
	if _, err = f.s.sessionHandlers.manager.WithAccess(sessions.Access{All: true}).AppendExchange("chat", "model", "stale writer", "answer", nil); !errors.Is(err, sessions.ErrCanonicalConversation) {
		t.Fatal("legacy executor guard lost", err)
	}
	task := &agents.Task{Actor: f.s.agentActor(r), Origin: &agents.ConversationOrigin{Conversation: c.ID}, Config: cfg}
	if !f.s.taskAttachmentsAllowed(task.Actor, task) {
		t.Fatal("authorized source unavailable")
	}
	if f.s.taskAttachmentsAllowed("another-actor", task) {
		t.Fatal("source leaked across actors")
	}
}

func TestCanonicalLibraryBlockScopeHTTPAndRestart(t *testing.T) {
	f := newConversationFixture(t)
	f.s.config.EnableConversationPreview = true
	f.s.bindCanonicalConversationGuard()
	e := rag.NewEngine(nil, t.TempDir())
	defer e.Close()
	f.s.ragEngine = e
	store, err := e.ImportStore()
	if err != nil {
		t.Fatal(err)
	}
	text := "Only the selected policy clause.\nAn unrelated paragraph must not be added."
	doc := &rag.Document{ID: "policy", Name: "Policy", RawContent: text, SourceRanges: []rag.SourceRange{{Kind: "section", Index: 1, Start: 0, End: len(text), SHA256: strings.Repeat("a", 64), Revision: "fixture"}}, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err = store.AddDocumentWithChunks(doc, nil, nil); err != nil {
		t.Fatal(err)
	}
	page, err := e.ForActor("local").Structure(context.Background(), doc.ID, "", "", 0, 100, 0)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", "/api/v2/conversations/import", nil)
	manifest, err := f.s.migrateConversation(r, "chat")
	if err != nil {
		t.Fatal(err)
	}
	c, err := f.s.agentManager.Conversation(f.s.controlScope(r), manifest.ConversationID)
	if err != nil {
		t.Fatal(err)
	}
	selected := api.ConversationKnowledgeSelection{DocumentID: doc.ID, Revision: page.Revision, BlockIDs: []string{page.Blocks[0].ID}}
	// An explicit scope remains usable when semantic search has not been enabled.
	// Profile/conversation inventory must still be authorized, without embedding.
	ids := []string{doc.ID}
	config := agents.AgentConfig{KnowledgeSelection: &ids}
	wInventory := httptest.NewRecorder()
	if !f.s.attachJobKnowledge(wInventory, r, &jobSubmitRequest{DeferKnowledge: true, SelectedKnowledge: true}, &config) || config.KnowledgeContext != "" || len(config.KnowledgeDocuments) != 1 {
		t.Fatal("exact evidence invoked semantic retrieval", wInventory.Body.String(), config.KnowledgeContext)
	}
	input := api.ConversationInboxInput{Content: "Explain this clause", Model: "model", KnowledgeSelections: []api.ConversationKnowledgeSelection{selected}}
	command := api.ConversationInputCommand{RequestID: "library-scope-once", TargetID: c.ID, ExpectedRevision: c.Revision, Kind: "submit_input", Payload: input}
	w := controlHTTP(f.s, "POST", "/api/v2/conversations/"+c.ID+"/inputs", command)
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	var receipt api.ConversationCommandReceipt
	if err = json.Unmarshal(w.Body.Bytes(), &receipt); err != nil {
		t.Fatal(err)
	}
	task, err := f.s.prepareConversationTurn(context.Background(), f.s.controlScope(r), c.ID, receipt.Input.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(task.Config.KnowledgeSources) == 0 || task.Config.KnowledgeSources[0].Content != page.Blocks[0].Text || task.Config.KnowledgeSources[0].EvidenceReceipt.Revision != page.Revision {
		t.Fatal("lost immutable scope", task.Config.KnowledgeSources)
	}
	restarted := agents.NewManagerWithPersistence(nil, nil, nil, f.agentDirectory)
	retained, ok := restarted.GetTask(task.ID)
	if !ok || retained.Config.KnowledgeSources[0].Content != page.Blocks[0].Text {
		t.Fatal("restart lost evidence")
	}
	input.KnowledgeSelections[0].Revision = "missing"
	if err = f.s.validateConversationSources(r, c, input); err == nil {
		t.Fatal("stale revision broadened")
	}
	input.KnowledgeSelections[0] = selected
	input.UseKnowledge = true
	if err = f.s.validateConversationSources(r, c, input); err == nil {
		t.Fatal("mixed broad scope accepted")
	}
}
