package main

import (
	"github.com/takuphilchan/offgrid-llm/internal/computer"
	"testing"
)

func TestDesktopPrivateProtocolNeverDowngradesToWindowSelection(t *testing.T) {
	authority := &computer.DesktopAuthority{}
	requests := []request{{Protocol: 3, ID: "request", Kind: "desktop_identity"}, {Protocol: 3, ID: "request", Kind: "applications"}, {Protocol: 3, ID: "request", Kind: "surfaces"}, {Protocol: 3, ID: "request", Kind: "grant_desktop", Desktop: authority, Prompt: "Open Chrome", ApprovalMode: computer.ApprovalFullTask}, {Protocol: 3, ID: "request", Kind: "prepare_lifecycle", Operation: &computer.Operation{Kind: "open_application", Application: "opaque-id"}}, {Protocol: 3, ID: "request", Kind: "verify_application", Application: "opaque-id"}}
	requests = append(requests, request{Protocol: 3, ID: "request", Kind: "enable_desktop", Permission: &computer.DesktopPermission{Service: "http://127.0.0.1:11611", Workspace: "workspace", Actor: "owner"}}, request{Protocol: 3, ID: "request", Kind: "release_desktop"})
	for _, req := range requests {
		if !req.valid() {
			t.Fatal("valid v3 request rejected", req.Kind)
		}
		req.Protocol = 2
		if req.valid() {
			t.Fatal("v2 accepted desktop request", req.Kind)
		}
	}
	for _, req := range []request{{Protocol: 3, ID: "request", Kind: "select", Target: "window", Binding: &computer.ControlBinding{}, ApprovalMode: computer.ApprovalFullTask}, {Protocol: 3, ID: "request", Kind: "targets"}, {Protocol: 3, ID: "request", Kind: "applications", Application: "path"}, {Protocol: 3, ID: "request", Kind: "grant_desktop", Desktop: authority, Prompt: "Open Chrome", ApprovalMode: "unrestricted"}} {
		if req.valid() {
			t.Fatal("invalid desktop request accepted", req.Kind)
		}
	}
}
