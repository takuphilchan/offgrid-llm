package computer

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func paired(t *testing.T) (*BrowserHub, *BrowserSession, string) {
	t.Helper()
	h := NewBrowserHub()
	code, err := h.PairCode("alice")
	if err != nil {
		t.Fatal(err)
	}
	s, token, err := h.Pair(code, "https://example.com", 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = h.Pair(code, "https://example.com", 1); err == nil {
		t.Fatal("pairing code reused")
	}
	return h, s, token
}

func TestDemoPairingDoesNotEnableArbitraryLocalOrigins(t *testing.T) {
	h := NewBrowserHub()
	code, err := h.PairCode("alice")
	if err != nil {
		t.Fatal(err)
	}
	for _, origin := range []string{"http://127.0.0.1:11611", "offgrid-demo://other", "offgrid-demo://research/path", "offgrid-demo://research?url=http://localhost"} {
		if _, _, err := h.Pair(code, origin, ProtocolVersion); err == nil {
			t.Fatalf("accepted %s", origin)
		}
	}
	session, _, err := h.Pair(code, "offgrid-demo://research", ProtocolVersion)
	if err != nil || session.Origin != "offgrid-demo://research" {
		t.Fatalf("demo: %v %v", session, err)
	}
}
func TestBrowserPairingScopeAndExpiry(t *testing.T) {
	h, s, token := paired(t)
	if len(h.List("bob")) != 0 || h.Check("bob", s.ID, "") == nil {
		t.Fatal("cross-user session")
	}
	if _, err := h.Poll("forged"); err == nil {
		t.Fatal("invalid token accepted")
	}
	if _, err := h.Poll(token); err != nil {
		t.Fatal(err)
	}
	h.mu.Lock()
	h.sessions[s.ID].ExpiresAt = time.Now().Add(-time.Second)
	h.mu.Unlock()
	if _, err := h.Poll(token); err == nil {
		t.Fatal("expired session accepted")
	}
}
func TestBrowserDispatchCannotReplayOrChangeRun(t *testing.T) {
	h, s, token := paired(t)
	done := make(chan error, 1)
	go func() {
		_, err := h.Execute(context.Background(), "alice", s.ID, "run", "call", "browser_observe", json.RawMessage(`{}`))
		done <- err
	}()
	var a *BrowserAction
	deadline := time.Now().Add(time.Second)
	for a == nil && time.Now().Before(deadline) {
		a, _ = h.Poll(token)
		time.Sleep(time.Millisecond)
	}
	if a == nil {
		t.Fatal("no action")
	}
	if duplicate, _ := h.Poll(token); duplicate != nil {
		t.Fatal("action redelivered")
	}
	if h.Check("alice", s.ID, "other-run") == nil {
		t.Fatal("session reused across active runs")
	}
	if h.Reply(token, BrowserReply{ID: "forged", Result: `{}`}) == nil {
		t.Fatal("wrong call result accepted")
	}
	if err := h.Reply(token, BrowserReply{ID: a.ID, Result: `{"observed":true}`}); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	h.Stop("alice")
	if h.Heartbeat(token) == nil {
		t.Fatal("stopped token still valid")
	}
}
func TestBrowserCancellationLeavesUncertainOutcome(t *testing.T) {
	h, s, _ := paired(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := h.Execute(ctx, "alice", s.ID, "run", "call", "browser_click", json.RawMessage(`{}`))
	if !errors.Is(err, ErrUncertain) {
		t.Fatalf("outcome: %v", err)
	}
	if len(h.List("alice")) != 0 {
		t.Fatal("cancelled session remains live")
	}
}

func TestBrowserSessionReservationAndPresentation(t *testing.T) {
	h, s, _ := paired(t)
	if got := h.List("alice"); len(got) != 1 || got[0].State != "ready" {
		t.Fatal("fresh session not ready")
	}
	if err := h.Reserve("bob", s.ID, "wrong"); err == nil {
		t.Fatal("cross actor reservation")
	}
	if err := h.Reserve("alice", s.ID, "run-a"); err != nil {
		t.Fatal(err)
	}
	if err := h.Reserve("alice", s.ID, "run-b"); err == nil {
		t.Fatal("session reserved twice")
	}
	got := h.List("alice")
	if got[0].State != "in_use" || got[0].RunID != "run-a" {
		t.Fatal("used session advertised as ready")
	}
	if h.Check("alice", s.ID, "") == nil {
		t.Fatal("new submission allowed on reserved session")
	}
	if h.Check("alice", s.ID, "run-a") != nil {
		t.Fatal("owning task lost session")
	}
}
