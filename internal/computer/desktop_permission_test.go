package computer

import "testing"

func TestDesktopPermissionDoesNotBroadenIdentityOrPolicy(t *testing.T) {
	consent, _ := desktopConsentFixture()
	authority := *consent.Desktop
	p := DesktopPermission{Service: "http://127.0.0.1:11611", Workspace: authority.Workspace, Actor: authority.Binding.Actor}
	if !p.Permits(p.Service, authority, ApprovalFullTask) {
		t.Fatal("matching session permission rejected")
	}
	for _, change := range []func(*DesktopPermission){func(p *DesktopPermission) { p.Actor = "other" }, func(p *DesktopPermission) { p.Workspace = "other" }, func(p *DesktopPermission) { p.Service = "http://127.0.0.1:9999" }} {
		other := p
		change(&other)
		if other.Permits(p.Service, authority, ApprovalFullTask) {
			t.Fatal("permission broadened")
		}
	}
	for _, mode := range []ApprovalMode{ApprovalScopedChanges, ApprovalAskEveryTime, ""} {
		if p.Permits(p.Service, authority, mode) {
			t.Fatal("different policy reused consent")
		}
	}
	for _, service := range []string{"https://example.com", "http://127.0.0.1/path", "http://user@127.0.0.1", "http://127.0.0.1?q=1", "file:///tmp"} {
		other := p
		other.Service = service
		if other.Validate() == nil {
			t.Fatal(service)
		}
	}
	// A new task still needs its own binding; permission is not a task grant.
	authority.Binding.Task = "next-task"
	if !p.Permits(p.Service, authority, ApprovalFullTask) {
		t.Fatal("next explicit task should be eligible")
	}
	if (DesktopPermission{}).Permits(p.Service, authority, ApprovalFullTask) {
		t.Fatal("empty/restarted host authorized")
	}
}
