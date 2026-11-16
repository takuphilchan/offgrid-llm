package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestCanonicalImportRetainsSourceAndRejectsDualWriters(t *testing.T) {
	f := newConversationFixture(t)
	scope := f.s.sessionHandlers.manager.WithAccess(sessions.Access{All: true})
	if _, err := scope.AppendExchange("chat", "model", "Keep this instruction", "Historical answer", nil); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(f.directory, "chat.json")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	f.s.bindCanonicalConversationGuard()
	request := httptest.NewRequest("POST", "/api/v2/conversations/import", nil)
	var wg sync.WaitGroup
	ids := make(chan string, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			manifest, e := f.s.migrateConversation(request, "chat")
			if e != nil {
				t.Error(e)
				return
			}
			ids <- manifest.ConversationID
		}()
	}
	wg.Wait()
	close(ids)
	id := ""
	for got := range ids {
		if id != "" && id != got {
			t.Fatal("duplicate import")
		}
		id = got
	}
	if id == "" {
		t.Fatal("missing import")
	}
	controlScope := f.s.controlScope(request)
	imported, err := f.s.agentManager.Conversation(controlScope, id)
	if err != nil || imported.ConfigurationKnown || imported.Preferences.ProfileID != "" || imported.Preferences.Model != "model" {
		t.Fatal("invented browser configuration", imported, err)
	}
	retained, err := f.s.agentManager.ImportedConversationSource(controlScope, id)
	if err != nil || !bytes.Equal(original, retained) {
		t.Fatal("source not retained exactly", err)
	}
	if _, err = scope.AppendExchange("chat", "model", "stale client", "not canonical", nil); !errors.Is(err, sessions.ErrCanonicalConversation) {
		t.Fatal("second authority accepted edit", err)
	}
	if err = scope.Delete("chat"); !errors.Is(err, sessions.ErrCanonicalConversation) {
		t.Fatal("legacy delete erased migration source", err)
	}
	if _, err = scope.CreateAttachment("chat", "draft.txt", "text/plain", []byte("new content")); err != nil {
		t.Fatal("immutable source upload rejected after canonical migration", err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(original, after) {
		t.Fatal("original modified", err)
	}
	if foreign, readErr := f.s.agentManager.ImportedConversationSource(agents.ControlScope{Workspace: controlScope.Workspace, Actor: "other"}, id); readErr != nil || len(foreign) != 0 {
		t.Fatal("cross-actor import source", readErr)
	}
	// Reload observes canonical ownership without reimporting or replaying work.
	f.s.agentManager = agents.NewManagerWithPersistence(nil, nil, nil, f.agentDirectory)
	if err = f.s.agentManager.StorageError(); err != nil {
		t.Fatal(err)
	}
	if _, err = scope.AppendExchange("chat", "model", "after restart", "still forbidden", nil); !errors.Is(err, sessions.ErrCanonicalConversation) {
		t.Fatal("restart reopened legacy writer", err)
	}
}

func TestCanonicalImportLeavesActiveLegacyRunsUntouched(t *testing.T) {
	f := newConversationFixture(t)
	if err := f.s.agentRunner.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	current, err := f.s.sessionHandlers.manager.Load("chat")
	if err != nil {
		t.Fatal(err)
	}
	current.Turn = &sessions.Turn{ID: "active", Status: "running"}
	if err = f.s.sessionHandlers.manager.Save(current); err != nil {
		t.Fatal(err)
	}
	f.s.bindCanonicalConversationGuard()
	request := httptest.NewRequest("POST", "/api/v2/conversations/import", nil)
	if _, err = f.s.migrateConversation(request, "chat"); !errors.Is(err, sessions.ErrTurnConflict) {
		t.Fatal("active work migrated", err)
	}
	manifest, err := f.s.agentManager.ImportedConversation(f.s.controlScope(request), "chat")
	if err != nil || manifest != nil {
		t.Fatal("failed migration switched authority", manifest, err)
	}
	loaded, err := f.s.sessionHandlers.manager.Load("chat")
	if err != nil || loaded.Turn.Status != "running" {
		t.Fatal("legacy state changed", loaded, err)
	}
}

func TestCanonicalProjectionFailureCannotRepeatAcceptedWork(t *testing.T) {
	f := newConversationFixture(t)
	f.s.config.EnableConversationPreview = true
	scoped := f.s.sessionHandlers.manager.WithAccess(sessions.Access{All: true})
	if _, err := scoped.AppendExchange("chat", "model", "Keep the original constraint", "Original answer", nil); err != nil {
		t.Fatal(err)
	}
	f.s.bindCanonicalConversationGuard()
	request := httptest.NewRequest("POST", "/api/v2/conversations/import", nil)
	manifest, err := f.s.migrateConversation(request, "chat")
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	runner := agents.NewRunner(f.s.agentManager, f.s.toolRegistry, func(_ context.Context, _ *agents.Task, messages []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		calls.Add(1)
		found := false
		for _, m := range messages {
			if m.StringContent() == "Keep the original constraint" {
				found = true
			}
		}
		if !found {
			t.Error("imported context missing")
		}
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "stop", Message: api.ChatMessage{Role: "assistant", Content: "Canonical answer"}}}}, nil
	})
	f.s.agentRunner = runner
	t.Cleanup(func() { _ = runner.Shutdown(context.Background()) })
	command := agents.ConversationCommand{RequestID: "canonical-after-import", TargetID: manifest.ConversationID, ExpectedRevision: 1, Kind: "create_work", Payload: agents.ConversationCommandPayload{Input: &agents.ConversationInput{Content: "Continue from the original"}}}
	path := "/api/v2/conversations/" + manifest.ConversationID + "/commands"
	w := controlHTTP(f.s, "POST", path, command)
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	var receipt agents.ConversationCommandReceipt
	if err = json.Unmarshal(w.Body.Bytes(), &receipt); err != nil {
		t.Fatal(err)
	}
	if _, err = runner.Continue(context.Background(), receipt.Work.RunID, "local-admin", "start", "", false); err != nil {
		t.Fatal(err)
	}
	f.s.sessionHandlers.admissionFault = func(stage string) error {
		if stage == "canonical_projection" {
			return errors.New("injected projection failure")
		}
		return nil
	}
	read := httptest.NewRecorder()
	f.s.sessionHandlers.HandleSessionGet(read, httptest.NewRequest("GET", "/v1/sessions/chat", nil), "chat")
	if read.Code < 500 {
		t.Fatal("projection injection did not fail", read.Code)
	}
	w = controlHTTP(f.s, "POST", path, command)
	if w.Code != 200 || calls.Load() != 1 {
		t.Fatal("projection failure repeated execution", w.Code, calls.Load())
	}
	f.s.sessionHandlers.admissionFault = nil
	read = httptest.NewRecorder()
	f.s.sessionHandlers.HandleSessionGet(read, httptest.NewRequest("GET", "/v1/sessions/chat", nil), "chat")
	if read.Code != 200 || !strings.Contains(read.Body.String(), "Canonical answer") || !strings.Contains(read.Body.String(), manifest.ConversationID) {
		t.Fatal("canonical compatibility projection missing", read.Code, read.Body.String())
	}
	var oldCalls atomic.Int32
	f.s.sessionHandlers.SetCompleter(func(context.Context, string, []api.ChatMessage, bool) (string, error) {
		oldCalls.Add(1)
		return "must not run", nil
	})
	legacy := httptest.NewRecorder()
	f.s.sessionHandlers.HandleSessionGenerate(legacy, httptest.NewRequest("POST", "/v1/sessions/chat/generate", strings.NewReader(`{"content":"legacy edit","model_id":"model"}`)), "chat")
	if legacy.Code != 409 || oldCalls.Load() != 0 {
		t.Fatal("legacy executor dispatched after ownership switch", legacy.Code, oldCalls.Load(), legacy.Body.String())
	}
	if calls.Load() != 1 {
		t.Fatal("observations dispatched work")
	}
}
