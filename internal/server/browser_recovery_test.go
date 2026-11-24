package server

import (
	"errors"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/computer"
)

func TestBrowserFailureRequiresTypedEvidenceAndMatchingOperation(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		safe bool
	}{
		{"browser_observe", computer.ErrBrowserObservation, true},
		{"browser_click", computer.ErrBrowserPrecondition, true},
		{"browser_click", computer.ErrBrowserObservation, false},
		{"computer_observe", computer.ErrBrowserObservation, false},
		{"browser_click", computer.ErrUncertain, false},
		{"browser_click", errors.New("computer_browser_precondition_failed"), false},
	} {
		failure := browserFailure(tc.name, tc.err)
		if (failure != nil) != tc.safe {
			t.Fatalf("%s %v: %+v", tc.name, tc.err, failure)
		}
		if failure != nil && (!failure.BeforeEffects || failure.Recovery == "") {
			t.Fatal("missing recovery proof")
		}
	}
	wrapped := safeComputerAuthorization(computer.ErrBrowserPrecondition)
	if browserFailure("browser_click", wrapped) == nil {
		t.Fatal("authorization wrapping discarded trusted cause")
	}
}

func TestBrowserWorkspaceToolsAreNegotiatedAndEffectPinned(t *testing.T) {
	tools := browserToolsForVision(false, true, true)
	if len(tools) != 14 {
		t.Fatalf("workspace tools %d", len(tools))
	}
	for _, tool := range tools {
		if !computer.BrowserWorkspaceTool(tool.Function.Name) {
			continue
		}
		d, ok := browserDescriptor(tool.Function.Name)
		if !ok || !capabilities.TrustedBrowserOperation(d) {
			t.Fatal("untrusted workspace effect", tool.Function.Name)
		}
		if (d.Risk == capabilities.RiskLow) != (tool.Function.Name == "browser_tabs") {
			t.Fatal("wrong effect classification")
		}
	}
	for _, tool := range browserToolsForVision(false, true) {
		if computer.BrowserWorkspaceTool(tool.Function.Name) {
			t.Fatal("legacy companion sees task tabs")
		}
	}
}

func TestBrowserDiscoveryDoesNotLeakIntoLegacyProbeOrCompanion(t *testing.T) {
	for _, tc := range []struct {
		vision, discovery bool
		count             int
	}{{false, false, 9}, {true, false, 10}, {false, true, 10}, {true, true, 11}} {
		tools := browserToolsForVision(tc.vision, tc.discovery)
		if len(tools) != tc.count {
			t.Fatalf("wrong negotiated tools: %+v", tc)
		}
		for _, tool := range tools {
			if tool.Function.Name == "browser_inspect" && !tc.discovery {
				t.Fatal("unnegotiated discovery")
			}
		}
	}
	if len(browserToolsForVision(false)) != 9 {
		t.Fatal("legacy model probe contract changed")
	}
}
