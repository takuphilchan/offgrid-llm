package computer

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestBrowserWorkspaceScopeNegotiation(t *testing.T) {
	for _, raw := range []string{`{"workspace":1}`, `{"discovery":1,"workspace":2}`, `{"discovery":1,"workspace":null}`, `{"discovery":1,"workspace":1,"origins":["https://one.example"]}`} {
		var f BrowserFeatures
		if json.Unmarshal([]byte(raw), &f) == nil {
			t.Fatalf("accepted malformed features %s", raw)
		}
	}
	for _, scope := range [][]string{{"https://two.example"}, {"https://one.example", "http://two.example"}, {"https://one.example", "https://127.0.0.1"}, {"https://one.example", "https://two.example/path"}, {"https://one.example", "https://user@two.example"}, {"https://one.example", "https://two.local"}, {"https://one.example", "https://one.example"}} {
		h := NewBrowserHub()
		code, _ := h.PairCode("alice")
		if _, _, err := h.PairBrowserFeatures(code, "https://one.example", 1, ApprovalFullTask, BrowserFeatures{Discovery: 1, Workspace: 1}, scope); err == nil {
			t.Fatalf("accepted scope %v", scope)
		}
	}
	h := NewBrowserHub()
	code, _ := h.PairCode("alice")
	scope := []string{"https://one.example", "https://two.example"}
	s, _, err := h.PairBrowserFeatures(code, scope[0], 1, ApprovalFullTask, BrowserFeatures{Discovery: 1, Workspace: 1}, scope)
	if err != nil {
		t.Fatal(err)
	}
	scope[1] = "https://evil.example"
	s.BrowserOrigins[1] = "https://evil.example"
	list := h.List("alice")
	list[0].BrowserOrigins[1] = "https://evil.example"
	if got := h.List("alice")[0].BrowserOrigins[1]; got != "https://two.example" {
		t.Fatal("mutable consent scope", got)
	}
	if len(h.List("bob")) != 0 {
		t.Fatal("foreign actor saw scope")
	}
	if _, err := h.Execute(context.Background(), "alice", s.ID, "run", "call", "browser_tab_open", json.RawMessage(`{"url":"https://evil.example"}`)); !errors.Is(err, ErrBrowserPrecondition) {
		t.Fatal("scope widened", err)
	}
}

func TestBrowserWorkspaceLegacyAndPrepareCannotBypassScope(t *testing.T) {
	h := NewBrowserHub()
	code, _ := h.PairCode("alice")
	s, _, err := h.PairBrowserFeatures(code, "https://one.example", 1, ApprovalFullTask, BrowserFeatures{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = h.Execute(context.Background(), "alice", s.ID, "run", "call", "browser_tabs", json.RawMessage(`{}`)); !errors.Is(err, ErrControlScope) {
		t.Fatal(err)
	}
	for _, args := range []string{`{"tool":"browser_tab_open","arguments":{"url":"https://one.example"}}`, `{"tool":"browser_navigate","arguments":{"url":"https://two.example"}}`} {
		if _, err = h.Execute(context.Background(), "alice", s.ID, "run", "call", "computer_prepare", json.RawMessage(args)); !errors.Is(err, ErrBrowserPrecondition) {
			t.Fatal("prepare bypass", err)
		}
	}
	session := &BrowserSession{Origin: "https://one.example", BrowserFeatures: BrowserFeatures{Discovery: 1, Workspace: 1}, BrowserOrigins: []string{"https://one.example", "https://two.example"}}
	for _, kind := range []string{"browser_navigate", "browser_tab_open"} {
		if !browserScopeAllows(session, kind, json.RawMessage(`{"url":"https://two.example/page?q=1"}`)) {
			t.Fatal("approved origin denied")
		}
		for _, raw := range []string{`{"url":"http://two.example"}`, `{"url":"https://user@two.example"}`, `{"url":"https://two.example:444"}`, `{"url":"file:///etc/passwd"}`} {
			if browserScopeAllows(session, kind, json.RawMessage(raw)) {
				t.Fatal("unapproved scope", raw)
			}
		}
	}
}
