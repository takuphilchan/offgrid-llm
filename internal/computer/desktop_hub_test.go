package computer

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestDesktopReadFailureKeepsOnlyValidAuthority(t *testing.T) {
	for _, want := range []error{ErrTargetNotSelected, ErrApplicationNotReady, ErrDocumentChanged, ErrControlScope} {
		t.Run(want.Error(), func(t *testing.T) {
			h := NewBrowserHub()
			consent, _ := desktopConsentFixture()
			code, _ := h.PairTaskCode("alice", "run", "input", "workspace", ApprovalFullTask)
			s, token, err := h.PairDesktop(code, consent.Binding.Target, ApprovalFullTask)
			if err != nil {
				t.Fatal(err)
			}
			if err := h.ActivateDesktop(token); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
			defer cancel()
			done := make(chan error, 1)
			go func() {
				_, err := h.Execute(ctx, "alice", s.ID, "run", "read", "computer_observe", json.RawMessage(`{}`))
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
				t.Fatal("read not queued")
			}
			if err := h.Reply(token, BrowserReply{ID: action.ID, Error: want.Error()}); err != nil {
				t.Fatal(err)
			}
			if err := <-done; !errors.Is(err, want) {
				t.Fatalf("wrong read failure: %v", err)
			}
			alive := h.Check("alice", s.ID, "run") == nil
			if alive == (want == ErrControlScope) {
				t.Fatal("wrong authority lifetime")
			}
			if alive && h.List("alice")[0].Remaining != DesktopActionBudget {
				t.Fatal("read spent mutation budget")
			}
			if h.Check("bob", s.ID, "run") == nil {
				t.Fatal("cross-actor access")
			}
		})
	}
}

func TestDesktopEnrollmentCannotDowngradeOrRunBeforeConsent(t *testing.T) {
	h := NewBrowserHub()
	consent, _ := desktopConsentFixture()
	target := consent.Binding.Target
	code, err := h.PairTaskCode("alice", "run", "input", "workspace", ApprovalFullTask)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = h.PairNative(code, "window", target, ControlProtocolVersion, ApprovalFullTask); err == nil {
		t.Fatal("v3 code downgraded")
	}
	s, token, err := h.PairDesktop(code, target, ApprovalFullTask)
	if err != nil {
		t.Fatal(err)
	}
	if s.State != "awaiting_consent" || s.Desktop.Binding.Task != "run" || s.InputID != "input" {
		t.Fatalf("lost binding: %+v", s)
	}
	if err = h.ReserveTask("alice", s.ID, "run"); err == nil {
		t.Fatal("enrollment granted authority")
	}
	if err = h.ActivateDesktop("wrong"); err == nil {
		t.Fatal("forged activation")
	}
	if err = h.ActivateDesktop(token); err != nil {
		t.Fatal(err)
	}
	for _, values := range [][4]string{{"bob", "run", "input", "workspace"}, {"alice", "another", "input", "workspace"}, {"alice", "run", "other", "workspace"}, {"alice", "run", "input", "other"}} {
		if _, err = h.DesktopScope(values[0], s.ID, values[1], values[2], values[3]); err == nil {
			t.Fatal("scope expanded", values)
		}
	}
	if granted, err := h.DesktopScope("alice", s.ID, "run", "input", "workspace"); err != nil || !granted {
		t.Fatal(granted, err)
	}
	if err = h.ReserveTask("alice", s.ID, "run"); err != nil {
		t.Fatal(err)
	}
	if err = h.ReserveTask("alice", s.ID, "another"); err == nil {
		t.Fatal("task rebound")
	}
	h.StopTask("alice", "other")
	if err = h.Check("alice", s.ID, "run"); err != nil {
		t.Fatal("other task revoked grant", err)
	}
	h.StopTask("alice", "run")
	if err = h.Check("alice", s.ID, "run"); err == nil {
		t.Fatal("terminal task retained authority")
	}
}

func TestDesktopConsentDeclineCanReenrollWithoutHeartbeatDelay(t *testing.T) {
	h := NewBrowserHub()
	consent, _ := desktopConsentFixture()
	code, _ := h.PairTaskCode("alice", "run", "input", "workspace", ApprovalFullTask)
	_, token, err := h.PairDesktop(code, consent.Binding.Target, ApprovalFullTask)
	if err != nil {
		t.Fatal(err)
	}
	if err = h.RevokeCompanion(token); err != nil {
		t.Fatal(err)
	}
	code, _ = h.PairTaskCode("alice", "run", "input", "workspace", ApprovalFullTask)
	if _, _, err = h.PairDesktop(code, consent.Binding.Target, ApprovalFullTask); err != nil {
		t.Fatal(err)
	}
}
