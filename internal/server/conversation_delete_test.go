package server

import (
	"errors"
	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestCanonicalConversationDeletionThroughSessionHTTP(t *testing.T) {
	f := newConversationFixture(t)
	f.s.bindCanonicalConversationGuard()
	r := httptest.NewRequest("POST", "/api/v2/conversations", nil)
	scope := f.s.controlScope(r)
	manifest, err := f.s.migrateConversation(r, "chat")
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.s.sessionHandlers.manager.WithAccess(sessions.Access{All: true}).CreateAttachment("chat", "evidence.txt", "text/plain", []byte("private retained source"))
	if err != nil {
		t.Fatal(err)
	}
	remove := func(query string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		f.s.sessionHandlers.HandleSessionDelete(w, httptest.NewRequest("DELETE", "/v1/sessions/chat"+query, nil), "chat")
		return w
	}
	if w := remove(""); w.Code != 409 {
		t.Fatal("legacy fence removed", w.Code, w.Body.String())
	}
	if w := remove("?canonical_id=other&revision=1"); w.Code != 404 {
		t.Fatal("identity mismatch", w.Code)
	}
	if w := remove("?canonical_id=" + manifest.ConversationID + "&revision=99"); w.Code != 409 {
		t.Fatal("stale revision", w.Code)
	}
	current, err := f.s.agentManager.Conversation(scope, manifest.ConversationID)
	if err != nil {
		t.Fatal(err)
	}
	query := "?canonical_id=" + manifest.ConversationID + "&revision=" + strconv.FormatInt(current.Revision, 10)
	// Simulate a cleanup failure after the canonical transaction has committed.
	dirs, err := filepath.Glob(filepath.Join(f.directory, ".attachments", "*"))
	if err != nil || len(dirs) != 1 {
		t.Fatal(dirs, err)
	}
	obstacle := filepath.Join(dirs[0], "unexpected")
	if err = os.Mkdir(obstacle, 0700); err != nil {
		t.Fatal(err)
	}
	if w := remove(query); w.Code != 503 {
		t.Fatal("cleanup failure was hidden", w.Code, w.Body.String())
	}
	if _, err = f.s.sessionHandlers.manager.WithAccess(sessions.Access{All: true}).Load("chat"); !errors.Is(err, sessions.ErrSessionNotFound) {
		t.Fatal("deleted source readable during cleanup failure", err)
	}
	if err = os.Remove(obstacle); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if w := remove(query); w.Code != 200 {
			t.Fatal("explicit delete/retry", w.Code, w.Body.String())
		}
	}
	if _, err = os.Stat(filepath.Join(f.directory, "chat.json")); !os.IsNotExist(err) {
		t.Fatal("legacy source not removed", err)
	}
	if _, err = f.s.agentManager.Conversation(scope, manifest.ConversationID); !errors.Is(err, agents.ErrControlNotFound) {
		t.Fatal("canonical conversation remains", err)
	}
	if source, err := f.s.agentManager.ImportedConversationSource(scope, manifest.ConversationID); err != nil || len(source) != 0 {
		t.Fatal("source remains readable", err)
	}
	page, err := f.s.agentManager.ControlRecords(scope, "conversation", "", 100)
	if err != nil || len(page.Items) != 0 {
		t.Fatal("deleted chat listed", page, err)
	}
	w := httptest.NewRecorder()
	f.s.sessionHandlers.HandleSessionsList(w, httptest.NewRequest("GET", "/v1/sessions", nil))
	if w.Code != 200 {
		t.Fatal("history unavailable after deletion", w.Code, w.Body.String())
	}
	if _, err = f.s.agentManager.ApplyConversationCommand(scope, agents.ConversationCommand{RequestID: "after-delete", TargetID: manifest.ConversationID, ExpectedRevision: current.Revision, Kind: "rename_conversation", Payload: agents.ConversationCommandPayload{Title: "resurrection"}}); !errors.Is(err, agents.ErrControlNotFound) {
		t.Fatal("deleted chat accepted write", err)
	}
	// A restart retains the deletion fence, not the removed body.
	f.s.agentManager = agents.NewManagerWithPersistence(nil, nil, nil, f.agentDirectory)
	if deleted, err := f.s.agentManager.ConversationDeleted(scope, manifest.ConversationID); err != nil || !deleted {
		t.Fatal("lost tombstone", err)
	}
}
