package computer

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestSessionToolAllowlistIncludesOnlyBoundedTransferAndShortcutTools(t *testing.T) {
	for _, name := range []string{"browser_download", "browser_upload"} {
		if !sessionAllowsTool("browser", name) {
			t.Fatalf("browser rejected %s", name)
		}
	}
	if !sessionAllowsTool("windows-uia", "computer_shortcut") {
		t.Fatal("native shortcut rejected")
	}
	for _, name := range []string{"computer_set_checked", "computer_verify_checked"} {
		if !sessionAllowsTool("windows-uia", name) {
			t.Fatalf("native checked-state tool rejected: %s", name)
		}
	}
	for _, name := range []string{"shell", "computer_key", "browser_script", "computer_shortcut"} {
		if sessionAllowsTool("browser", name) {
			t.Fatalf("browser accepted %s", name)
		}
	}
}

func TestNativeSessionCannotInheritBrowserAuthority(t *testing.T) {
	h := NewBrowserHub()
	code, _ := h.PairCode("alice")
	step, _, _, _ := controlFixture()
	target := step.Binding.Target
	if _, _, err := h.PairNative(code, "Editor", target, 1); err == nil {
		t.Fatal("v1 accepted native target")
	}
	if _, _, err := h.Pair(code, "https://example.com", 2); err == nil {
		t.Fatal("v2 accepted through browser adapter")
	}
	s, _, err := h.Pair(code, "https://example.com", 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.Execute(context.Background(), "alice", s.ID, "task", "call", "computer_activate", json.RawMessage(`{}`)); !errors.Is(err, ErrControlScope) {
		t.Fatalf("browser acquired native authority: %v", err)
	}
	if _, err := h.Driver("bob", s.ID); err == nil {
		t.Fatal("cross-user session disclosed")
	}
	if h.List("alice")[0].Remaining != 100 {
		t.Fatal("rejected action consumed budget")
	}
}

func TestNativeSessionDispatchCarriesImmutableHostTargetAndActor(t *testing.T) {
	for _, driver := range []string{"windows-uia", "macos-accessibility", "linux-atspi"} {
		t.Run(driver, func(t *testing.T) {
			h := NewBrowserHub()
			code, _ := h.PairCode("alice")
			step, _, _, _ := controlFixture()
			target := step.Binding.Target
			target.Driver = driver
			s, token, err := h.PairNative(code, "Editor — document", target, 2)
			if err != nil {
				t.Fatal(err)
			}
			s.Target.ID = "tampered-return"
			listed := h.List("alice")
			listed[0].Target.ID = "tampered-list"
			if _, err := h.Execute(context.Background(), "alice", s.ID, "task", "call", "browser_fill", json.RawMessage(`{}`)); !errors.Is(err, ErrControlScope) {
				t.Fatal("native accepted browser command")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			done := make(chan error, 1)
			go func() {
				_, err := h.Execute(ctx, "alice", s.ID, "task", "call", "computer_observe", json.RawMessage(`{}`))
				done <- err
			}()
			var action *BrowserAction
			for action == nil && ctx.Err() == nil {
				action, err = h.Poll(token)
				if err != nil {
					t.Fatal(err)
				}
				if action == nil {
					time.Sleep(time.Millisecond)
				}
			}
			if action == nil {
				t.Fatal("dispatch not delivered")
			}
			want := ControlBinding{Actor: "alice", Task: "task", Companion: s.ID, Session: s.ID, Target: target}
			if action.Binding == nil || *action.Binding != want {
				t.Fatalf("incorrect execution binding: %+v", action.Binding)
			}
			action.Binding.Actor = "bob"
			if h.sessions[s.ID].pending.Binding.Actor != "alice" {
				t.Fatal("poll exposed mutable binding")
			}
			if err := h.Reply(token, BrowserReply{ID: action.ID, Result: `{"elements":[]}`}); err != nil {
				t.Fatal(err)
			}
			if err := <-done; err != nil {
				t.Fatal(err)
			}
			h.Stop("alice")
			if _, err := h.Poll(token); err == nil {
				t.Fatal("revoked native session still live")
			}
		})
	}
}
