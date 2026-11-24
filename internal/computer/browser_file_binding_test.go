package computer

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

const fileRun = "run-11111111111111111111111111111111"

func fileFeatures() BrowserFeatures {
	return BrowserFeatures{Discovery: 1, Workspace: 1, Structured: 1, Transfers: 1}
}

func TestBrowserFileFeaturesRequireExactNegotiationAndTaskCode(t *testing.T) {
	var features BrowserFeatures
	if err := json.Unmarshal([]byte(`{"discovery":1,"workspace":1,"structured":1,"transfers":1}`), &features); err != nil || features != fileFeatures() {
		t.Fatal(features, err)
	}
	for _, raw := range []string{`{"transfers":1}`, `{"discovery":1,"workspace":1,"transfers":1}`, `{"transfers":0}`, `{"transfers":null}`, `{"transfers":true}`, `{"transfers":1.0}`, `{"transfers":2}`, `{"transfers":1,"transfers":1}`, `{"unknown":1}`} {
		if json.Unmarshal([]byte(raw), &features) == nil {
			t.Fatal("invalid feature accepted", raw)
		}
	}
	h := NewBrowserHub()
	code, _ := h.PairCode("local-admin")
	if _, _, err := h.PairBrowserFeatures(code, "https://one.example", 1, ApprovalScopedChanges, fileFeatures()); err == nil {
		t.Fatal("generic pairing claimed task-bound file support")
	}
	legacy, _, err := h.PairBrowserFeatures(code, "https://one.example", 1, ApprovalScopedChanges, BrowserFeatures{})
	if err != nil {
		t.Fatal("rejected upgrade consumed legacy code", err)
	}
	if binding, err := legacy.FileBinding(); binding != nil || err != nil {
		t.Fatal(binding, err)
	}
}

func TestBrowserFileInitialPairingPinsSavedOriginPolicyAndCanonicalBinding(t *testing.T) {
	h := NewBrowserHub()
	code, err := h.PairBrowserTaskCode("local-admin", fileRun, "input-initial", "workspace", "", ApprovalScopedChanges, "https://ONE.example:443/path?private=value")
	if err != nil {
		t.Fatal(err)
	}
	for _, attempt := range []struct {
		origin   string
		mode     ApprovalMode
		features BrowserFeatures
	}{
		{"https://other.example", ApprovalScopedChanges, fileFeatures()},
		{"https://one.example", ApprovalFullTask, fileFeatures()},
		{"https://one.example", ApprovalScopedChanges, BrowserFeatures{Discovery: 1, Workspace: 1, Structured: 1}},
	} {
		if _, _, err := h.PairBrowserFeatures(code, attempt.origin, 1, attempt.mode, attempt.features); err == nil {
			t.Fatal("changed initial authority accepted", attempt)
		}
	}
	origins := []string{"https://one.example", "https://two.example"}
	session, _, err := h.PairBrowserFeatures(code, origins[0], 1, ApprovalScopedChanges, fileFeatures(), origins)
	if err != nil {
		t.Fatal(err)
	}
	binding, err := session.FileBinding()
	want := BrowserFileBinding{Version: 1, ActorID: "local-admin", WorkspaceID: "workspace", RunID: fileRun, InputID: "input-initial", SessionID: session.ID}
	if err != nil || binding == nil || *binding != want {
		t.Fatal(binding, err)
	}
	encoded, _ := json.Marshal(session)
	if strings.Contains(string(encoded), "actor_id") || strings.Contains(string(encoded), `"actor"`) || strings.Contains(string(encoded), "file_binding") {
		t.Fatal("public session exposed private actor binding", string(encoded))
	}
	origins[0] = "https://changed.example"
	if h.List("local-admin")[0].BrowserOrigins[0] != "https://one.example" {
		t.Fatal("scope alias")
	}
	if _, _, err := h.PairBrowserFeatures(code, "https://one.example", 1, ApprovalScopedChanges, fileFeatures()); err == nil {
		t.Fatal("pairing code replayed")
	}
	if err := h.ReserveTask("local-admin", session.ID, "run-22222222222222222222222222222222"); err == nil {
		t.Fatal("other task reserved initial pairing")
	}
}

func TestBrowserFileInitialBindingRejectsUnsupportedIdentity(t *testing.T) {
	h := NewBrowserHub()
	for _, values := range [][4]string{{"https://actor.example", "workspace", fileRun, "input-initial"}, {"local-admin", "C:/private", fileRun, "input-initial"}, {"local-admin", "workspace", "run-short", "input-initial"}, {"local-admin", "workspace", fileRun, "wrong-input"}} {
		if _, err := h.PairBrowserTaskCode(values[0], values[2], values[3], values[1], "", ApprovalScopedChanges); err == nil {
			t.Fatal(values)
		}
	}
	for _, requested := range []string{"https://user:secret@one.example/path", "http://one.example", "https://127.0.0.1/path", "https://host.local/path"} {
		if _, err := h.PairBrowserTaskCode("local-admin", fileRun, "input-initial", "workspace", "", ApprovalScopedChanges, requested); err == nil {
			t.Fatal("invalid saved request accepted", requested)
		}
	}
}

func TestBrowserFileRenewalKeepsExactScopeAndFreshPrivateInput(t *testing.T) {
	h := NewBrowserHub()
	code, _ := h.PairBrowserTaskCode("local-admin", fileRun, "input-initial", "workspace", "", ApprovalScopedChanges)
	origins := []string{"https://one.example", "https://two.example"}
	old, oldToken, err := h.PairBrowserFeatures(code, origins[0], 1, ApprovalScopedChanges, fileFeatures(), origins)
	if err != nil {
		t.Fatal(err)
	}
	h.StopSession("local-admin", old.ID)
	code, err = h.PairBrowserTaskCode("local-admin", fileRun, "input-renewed", "workspace", old.ID, ApprovalScopedChanges)
	if err != nil {
		t.Fatal(err)
	}
	for _, features := range []BrowserFeatures{{Discovery: 1, Workspace: 1, Structured: 1}, fileFeatures()} {
		scope := origins
		if features.Transfers == 1 {
			scope = []string{origins[0], "https://foreign.example"}
		}
		if _, _, err := h.PairBrowserFeatures(code, origins[0], 1, ApprovalScopedChanges, features, scope); err == nil {
			t.Fatal("renewal changed scope")
		}
	}
	next, _, err := h.PairBrowserFeatures(code, origins[0], 1, ApprovalScopedChanges, fileFeatures(), origins)
	if err != nil {
		t.Fatal(err)
	}
	binding, err := next.FileBinding()
	if err != nil || binding.InputID != "input-renewed" || binding.SessionID == old.ID || binding.RunID != fileRun {
		t.Fatal(binding, err)
	}
	if _, err := h.Poll(oldToken); err == nil {
		t.Fatal("old token remained live")
	}
}

func TestBrowserFilePollBindingIsDetachedAndLostReplyCannotRedeliver(t *testing.T) {
	h := NewBrowserHub()
	code, _ := h.PairBrowserTaskCode("local-admin", fileRun, "input-initial", "workspace", "", ApprovalScopedChanges)
	session, token, err := h.PairBrowserFeatures(code, "https://one.example", 1, ApprovalScopedChanges, fileFeatures())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := h.ExecuteAuthorized(ctx, "local-admin", session.ID, fileRun, "read", "browser_observe", json.RawMessage(`{}`), "auto")
		done <- err
	}()
	var action *BrowserAction
	for action == nil && ctx.Err() == nil {
		action, err = h.Poll(token)
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Millisecond)
	}
	if action == nil || action.FileBinding == nil || action.Binding != nil {
		t.Fatal(action)
	}
	if err := action.FileBinding.Validate(); err != nil {
		t.Fatal(err)
	}
	action.FileBinding.ActorID = "changed-copy"
	if again, err := h.Poll(token); err != nil || again != nil {
		t.Fatal("lost poll redelivered", again, err)
	}
	writeErr := errors.New("fixture durable receipt failure")
	h.RecordReply = func(actor, id, run string, a BrowserAction, reply BrowserReply) error {
		if actor != "local-admin" || id != session.ID || run != fileRun || a.FileBinding.ActorID != "local-admin" {
			t.Fatal("binding alias or mismatch", a.FileBinding)
		}
		a.FileBinding.ActorID = "changed-receipt-copy"
		return writeErr
	}
	reply := BrowserReply{ID: action.ID, Result: `{"observed":true}`}
	if err := h.Reply(token, reply); !errors.Is(err, writeErr) {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		t.Fatal("acknowledged before durability", err)
	default:
	}
	if again, err := h.Poll(token); err != nil || again != nil {
		t.Fatal("receipt failure redelivered", again, err)
	}
	writeErr = nil
	if err := h.Reply(token, reply); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
