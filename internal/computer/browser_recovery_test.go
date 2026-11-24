package computer

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestBrowserSafeFailuresKeepSessionAndUnknownEffectsStop(t *testing.T) {
	for _, tc := range []struct {
		kind, code string
		want       error
		live       bool
	}{
		{"browser_observe", "computer_browser_observation_failed", ErrBrowserObservation, true},
		{"browser_click", "computer_browser_precondition_failed", ErrBrowserPrecondition, true},
		{"computer_prepare", "computer_browser_precondition_failed", ErrBrowserPrecondition, true},
		{"browser_click", "computer_browser_observation_failed", ErrUncertain, false},
		{"browser_click", "computer_uncertain_outcome", ErrUncertain, false},
	} {
		t.Run(tc.kind+tc.code, func(t *testing.T) {
			h, session, token := paired(t)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			done := make(chan error, 1)
			go func() {
				_, err := h.Execute(ctx, "alice", session.ID, "run", "call", tc.kind, json.RawMessage(`{}`))
				done <- err
			}()
			var action *BrowserAction
			for action == nil && ctx.Err() == nil {
				action, _ = h.Poll(token)
				time.Sleep(time.Millisecond)
			}
			if action == nil {
				t.Fatal("no action")
			}
			if err := h.Reply(token, BrowserReply{ID: action.ID, Error: tc.code}); err != nil {
				t.Fatal(err)
			}
			if err := <-done; !errors.Is(err, tc.want) {
				t.Fatalf("error=%v want=%v", err, tc.want)
			}
			if live := h.Heartbeat(token) == nil; live != tc.live {
				t.Fatalf("live=%v want=%v", live, tc.live)
			}
		})
	}
}

func TestBrowserDiscoveryRequiresNegotiation(t *testing.T) {
	for _, version := range []int{0, 1} {
		h := NewBrowserHub()
		code, err := h.PairCode("alice")
		if err != nil {
			t.Fatal(err)
		}
		s, _, err := h.PairBrowserFeatures(code, "https://example.com", 1, ApprovalFullTask, BrowserFeatures{Discovery: version})
		if err != nil {
			t.Fatal(err)
		}
		features, err := h.BrowserFeatures("alice", s.ID)
		if err != nil || features.Discovery != version {
			t.Fatal("lost negotiation")
		}
		if _, err := h.BrowserFeatures("bob", s.ID); err == nil {
			t.Fatal("cross actor features")
		}
		if version == 0 {
			_, err = h.Execute(context.Background(), "alice", s.ID, "run", "call", "browser_inspect", json.RawMessage(`{}`))
			if !errors.Is(err, ErrControlScope) {
				t.Fatal("legacy companion received extended tool")
			}
		}
	}
	for _, raw := range []string{`{"discovery":0}`, `{"discovery":2}`, `{"discovery":"1"}`, `{"discovery":null}`, `{"extra":1}`} {
		var features BrowserFeatures
		if json.Unmarshal([]byte(raw), &features) == nil {
			t.Fatal("accepted invalid negotiation", raw)
		}
	}
}
