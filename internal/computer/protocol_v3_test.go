package computer

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func desktopConsentFixture() (LocalConsent, BoundedStep) {
	step, _, consent, _ := controlFixture()
	consent.IssuedAt = time.Now().Add(-time.Second)
	consent.ExpiresAt = consent.IssuedAt.Add(DesktopConsentDuration)
	consent.Binding.Target.ID = "desktop-grant"
	consent.Binding.Target.ProcessGeneration = "desktop-grant"
	consent.Binding.Target.Surface = "desktop"
	consent.Binding.Target.Driver = "windows-uia"
	step.Binding.Target.Driver = "windows-uia"
	consent.Desktop = &DesktopAuthority{Workspace: "workspace", Binding: consent.Binding}
	return consent, step
}

func TestDesktopConsentSeparatesAuthorityFromExecutionTarget(t *testing.T) {
	consent, step := desktopConsentFixture()
	if !consent.permits(step.Binding) {
		t.Fatal("ordinary target rejected")
	}
	other := step.Binding
	other.Target.ID, other.Target.ProcessGeneration, other.Target.Surface = "another-window", "new-process", "dialog"
	if !consent.permits(other) {
		t.Fatal("same desktop target requires new grant")
	}
	for _, change := range []func(*ControlBinding){
		func(b *ControlBinding) { b.Actor = "other" }, func(b *ControlBinding) { b.Task = "other" },
		func(b *ControlBinding) { b.Session = "other" }, func(b *ControlBinding) { b.Companion = "other" },
		func(b *ControlBinding) { b.Target.OSSession = "other" }, func(b *ControlBinding) { b.Target.Driver = "browser" },
	} {
		changed := step.Binding
		change(&changed)
		if consent.permits(changed) {
			t.Fatalf("expanded authority: %+v", changed)
		}
	}
	legacy := consent
	legacy.Desktop = nil
	legacy.Binding = step.Binding
	if legacy.permits(other) {
		t.Fatal("v2 silently expanded")
	}
	consent.ExpiresAt = consent.IssuedAt.Add(DesktopConsentDuration + time.Second)
	if consent.permits(step.Binding) {
		t.Fatal("overlong desktop consent")
	}
}

func TestDesktopSupervisorBudgetAndTargetBoundGrants(t *testing.T) {
	consent, step := desktopConsentFixture()
	journal, err := OpenNativeJournal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer journal.Close()
	effects := 0
	s := NewNativeSupervisor(journal, consent, func(_ context.Context, a PreparedControlAction) (ControlResult, error) {
		effects++
		return ControlResult{ActionID: a.ID, Outcome: "dispatched"}, nil
	})
	defer s.Stop()
	grant, err := s.Approve(step, func(BoundedStep) bool { return true })
	if err != nil {
		t.Fatal(err)
	}
	changed := step
	changed.Binding.Target.ID = "different"
	if _, err := s.Execute(changed, grant.ID); err == nil || effects != 0 {
		t.Fatal("approval moved to different target")
	}
	for i := 0; i < DesktopActionBudget; i++ {
		step.ID = fmt.Sprintf("step-%d", i)
		step.Actions[0].ID = fmt.Sprintf("action-%d", i)
		g, e := s.Approve(step, func(BoundedStep) bool { return true })
		if e != nil {
			t.Fatal(i, e)
		}
		if _, e = s.Execute(step, g.ID); e != nil {
			t.Fatal(i, e)
		}
	}
	if effects != DesktopActionBudget || s.Check() == nil {
		t.Fatal("budget not enforced", effects)
	}
	step.Actions[0].ID = "one-more"
	g, _ := s.Approve(step, func(BoundedStep) bool { return true })
	if _, err = s.Execute(step, g.ID); err == nil || effects != DesktopActionBudget {
		t.Fatal("budget overflow")
	}
}
