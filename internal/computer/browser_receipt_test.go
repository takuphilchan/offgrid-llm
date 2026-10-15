package computer

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestNativeReplyPersistsBeforeAcknowledgment(t *testing.T) {
	h := NewBrowserHub()
	code, _ := h.PairCode("alice")
	target := TargetIdentity{ID: "window", OSSession: "desktop", ProcessGeneration: "process", Surface: "surface", Driver: "windows-uia"}
	session, token, err := h.PairNative(code, "Editor", target, ControlProtocolVersion)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := h.ExecuteAuthorized(ctx, "alice", session.ID, "run", "call", "computer_replace_text", json.RawMessage(`{"observation_id":"view","element":"edit","text":"hello"}`), "exact")
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
	writeErr := errors.New("simulated journal failure")
	count := 0
	h.RecordReply = func(actor, sessionID, run string, a BrowserAction, r BrowserReply) error {
		count++
		if actor != "alice" || sessionID != session.ID || run != "run" || a.ID != "run:call" || r.ID != a.ID {
			t.Fatal("receipt lost authenticated binding")
		}
		return writeErr
	}
	reply := BrowserReply{ID: action.ID, Result: `{"actions":[{"outcome":"dispatched"}]}`}
	if h.Reply("forged", reply) == nil || count != 0 {
		t.Fatal("unauthenticated result reached journal")
	}
	if err := h.Reply(token, reply); !errors.Is(err, writeErr) {
		t.Fatal("acknowledged before durability", err)
	}
	select {
	case <-done:
		t.Fatal("released runner after failed persistence")
	default:
	}
	writeErr = nil
	if err := h.Reply(token, reply); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatal("unexpected recorder calls", count)
	}
}
