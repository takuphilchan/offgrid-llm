package computer

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"sync"
	"testing"
)

func browserTaskPairFixture(t *testing.T) (*BrowserHub, *BrowserSession, BrowserFeatures, []string) {
	t.Helper()
	h := NewBrowserHub()
	code, err := h.PairCode("actor")
	if err != nil {
		t.Fatal(err)
	}
	features := BrowserFeatures{Discovery: 1, Workspace: 1, Structured: 1}
	origins := []string{"https://one.example", "https://two.example"}
	old, _, err := h.PairBrowserFeatures(code, origins[0], 1, ApprovalScopedChanges, features, origins)
	if err != nil {
		t.Fatal(err)
	}
	if err = h.Reserve("actor", old.ID, "run-owned"); err != nil {
		t.Fatal(err)
	}
	return h, old, features, origins
}

func TestBrowserTaskPairingPinsAuthorityBeforePublication(t *testing.T) {
	h, old, features, origins := browserTaskPairFixture(t)
	if _, err := h.PairBrowserTaskCode("actor", "run-owned", "input-owned", "workspace", old.ID, ApprovalScopedChanges); err == nil {
		t.Fatal("live prior grant renewed")
	}
	h.StopSession("actor", old.ID)
	code, err := h.PairBrowserTaskCode("actor", "run-owned", "input-owned", "workspace", old.ID, ApprovalScopedChanges)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	stop := make(chan struct{})
	observed := make(chan BrowserSession, 1)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			for _, s := range h.List("actor") {
				if s.ID != old.ID && (s.RunID != "run-owned" || s.InputID != "input-owned" || s.BrowserFeatures != features || !slices.Equal(s.BrowserOrigins, origins)) {
					select {
					case observed <- s:
					default:
					}
					return
				}
			}
		}
	}()
	next, token, err := h.PairBrowserFeatures(code, origins[0], 1, ApprovalScopedChanges, features, origins)
	close(stop)
	wg.Wait()
	if err != nil || token == "" {
		t.Fatal(next, err)
	}
	select {
	case invalid := <-observed:
		t.Fatal("partly initialized session was published", invalid)
	default:
	}
	if next.RunID != "run-owned" || next.InputID != "input-owned" || !slices.Equal(next.BrowserOrigins, origins) {
		t.Fatal(next)
	}
	if err = h.ReserveTask("actor", next.ID, "other-run"); err == nil {
		t.Fatal("another task claimed renewed browser")
	}
	if err = h.Reserve("actor", next.ID, "other-run"); err == nil {
		t.Fatal("fresh submission claimed renewed browser")
	}
	if err = h.ReserveTask("actor", next.ID, "run-owned"); err != nil {
		t.Fatal(err)
	}
	for _, binding := range [][3]string{{"other-run", "input-owned", "workspace"}, {"run-owned", "other-input", "workspace"}, {"run-owned", "input-owned", "other-workspace"}} {
		if _, err = h.DesktopScope("actor", next.ID, binding[0], binding[1], binding[2]); err == nil {
			t.Fatal("changed binding accepted", binding)
		}
	}
	if desktop, err := h.DesktopScope("actor", next.ID, "run-owned", "input-owned", "workspace"); err != nil || desktop {
		t.Fatal(desktop, err)
	}
	if _, err = h.ExecuteAuthorized(context.Background(), "actor", next.ID, "other-run", "call", "browser_observe", json.RawMessage("{}"), "auto"); !errors.Is(err, ErrSession) {
		t.Fatal("foreign task dispatched", err)
	}
	next.BrowserOrigins[0] = "https://changed.example"
	if h.List("actor")[0].BrowserOrigins[0] != origins[0] {
		t.Fatal("returned scope aliases authority")
	}
}

func TestBrowserTaskPairingRejectsProtocolScopePolicyAndActorChanges(t *testing.T) {
	h, old, features, origins := browserTaskPairFixture(t)
	h.StopSession("actor", old.ID)
	for _, args := range [][5]string{{"other-actor", "run-owned", "input", "workspace", old.ID}, {"actor", "other-run", "input", "workspace", old.ID}, {"actor", "run-owned", "input", "workspace", "missing"}} {
		if _, err := h.PairBrowserTaskCode(args[0], args[1], args[2], args[3], args[4], ApprovalScopedChanges); err == nil {
			t.Fatal(args)
		}
	}
	if _, err := h.PairBrowserTaskCode("actor", "run-owned", "input", "workspace", old.ID, ApprovalFullTask); err == nil {
		t.Fatal("policy expanded")
	}
	code, err := h.PairBrowserTaskCode("actor", "run-owned", "input", "workspace", old.ID, ApprovalScopedChanges)
	if err != nil {
		t.Fatal(err)
	}
	for _, attempt := range []func() error{
		func() error {
			_, _, err := h.PairBrowserFeatures(code, origins[0], 1, ApprovalFullTask, features, origins)
			return err
		},
		func() error {
			_, _, err := h.PairBrowserFeatures(code, origins[0], 1, ApprovalScopedChanges, features, []string{origins[0], "https://other.example"})
			return err
		},
		func() error {
			_, _, err := h.PairBrowserFeatures(code, origins[0], 1, ApprovalScopedChanges, BrowserFeatures{Discovery: 1, Workspace: 1}, origins)
			return err
		},
		func() error {
			_, _, err := h.PairNative(code, "Native", TargetIdentity{ID: "window", OSSession: "os", ProcessGeneration: "process", Surface: "window", Driver: "windows-uia"}, 2, ApprovalScopedChanges)
			return err
		},
	} {
		if err = attempt(); err == nil {
			t.Fatal("changed enrollment accepted")
		}
	}
	if _, _, err = h.PairBrowserFeatures(code, origins[0], 1, ApprovalScopedChanges, features, origins); err != nil {
		t.Fatal("rejected mismatch consumed code", err)
	}
}
