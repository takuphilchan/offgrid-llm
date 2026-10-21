package server

import (
	"context"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/computer"
)

func TestNewlySelectedDocumentHostInspectsBeforeCatalogRediscovery(t *testing.T) {
	s, adapter, _, _, _ := compositionFixture(t)
	task := &agents.Task{Actor: "alice"}
	code, _ := s.browserHub.PairCode("alice")
	target := computer.TargetIdentity{ID: "window", OSSession: "os", ProcessGeneration: "process", Surface: "surface", Driver: "windows-uia"}
	session, _, err := s.browserHub.PairNativeFeatures(code, "Owned fixture", target, 2, computer.ApprovalFullTask, computer.NativeFeatures{Documents: computer.NativeDocumentsVersion})
	if err != nil {
		t.Fatal(err)
	}
	task.Config = compositionConfig(target.Driver, true)
	task.Prompt = "Inspect the current document surface without changing it."
	task.Config.ComputerSession, task.Config.ComputerScope, task.Config.NativeWorkflowVersion = session.ID, "desktop", agents.DocumentWorkflowEvidenceVersion
	view := computer.NativeView{Observation: computer.ControlObservation{ID: "ready", Target: target}}
	task.Steps = []agents.Step{{ID: 1, Type: "action", ToolName: "computer_applications", ToolResult: `{"applications":[{"id":"app","name":"Editor"}]}`}, {ID: 2, Type: "action", ToolName: "computer_verify_application", ToolResult: documentJSON(computer.ApplicationEvidence{Verified: true, Target: &target, View: &view})}}
	check := func(catalog bool) {
		t.Helper()
		names := map[string]bool{}
		for _, tool := range adapter.ToolsForTaskContext(context.Background(), task) {
			names[tool.Function.Name] = true
		}
		if !names["computer_documents"] || !names["computer_observe"] || names["computer_applications"] != catalog || !names["mcp__docs__docs"] {
			t.Fatal(names)
		}
	}
	check(false)
	task.Steps = append(task.Steps, agents.Step{ID: 3, Type: "action", ToolName: "computer_documents", ToolResult: unavailableDocumentCapabilityResult})
	check(true)
	if availableNames(adapter.ToolsForTask(task))["computer_activate"] {
		t.Fatal("unavailable documents authorized use of an earlier view")
	}
	view.Observation.ID = "new-view"
	task.Steps = append(task.Steps, agents.Step{ID: 4, Type: "action", ToolName: "computer_observe", ToolResult: documentJSON(view)})
	if !availableNames(adapter.ToolsForTask(task))["computer_activate"] {
		t.Fatal("fresh observation did not expose ordinary controls")
	}
}

func TestDesktopDiscoveryFollowsExecutedEvidenceNotRejectedBatches(t *testing.T) {
	task := &agents.Task{Config: agents.AgentConfig{ComputerScope: "desktop", ComputerSession: "session"}}
	check := func(open, verify, documents bool) {
		t.Helper()
		if desktopPlanningTool(task, "computer_open_application") != open || desktopPlanningTool(task, "computer_verify_application") != verify || desktopPlanningTool(task, "computer_documents") != documents {
			t.Fatal("incorrect desktop stage", task.Steps)
		}
	}
	check(false, false, false)
	task.Steps = append(task.Steps, agents.Step{ID: 1, Type: "action", ToolName: "computer_applications", ToolResult: `{"applications":[{"id":"editor","name":"Editor"}]}`})
	check(false, false, false)
	appendApplicationLookup(t, task, "Editor", 1)
	check(true, false, false)
	task.Steps = append(task.Steps, agents.Step{Type: "rejected", ToolName: "computer_open_application", ToolArgs: `{"application_id":"editor"}`})
	check(true, false, false)
	failure := nativeReadFailureForTask(task, nativeReadFailure(computer.ErrTargetNotSelected))
	if !strings.Contains(failure.Recovery, "first launch") {
		t.Fatal("rejected proposal fabricated an earlier launch", failure)
	}
	task.Steps = append(task.Steps, agents.Step{Type: "action", ToolName: "computer_open_application", ToolArgs: `{"application_id":"editor"}`})
	check(false, true, false)
	failure = nativeReadFailureForTask(task, nativeReadFailure(computer.ErrTargetNotSelected))
	if !strings.Contains(failure.Recovery, "Do not relaunch") {
		t.Fatal("allowed replay of dispatched launch", failure)
	}
	task.Steps = append(task.Steps, agents.Step{Type: "action", ToolName: "computer_verify_application", ToolArgs: `{"application_id":"editor"}`, ToolResult: `{"application_id":"editor","check":"application_open","verified":true,"target":{"id":"window"}}`})
	check(true, false, true)
}

func TestDocumentControlFallbackRequiresExplicitUnsupportedCurrentTarget(t *testing.T) {
	task := &agents.Task{}
	target := computer.TargetIdentity{ID: "window", OSSession: "os", ProcessGeneration: "process", Surface: "surface", Driver: "windows-uia"}
	observed := agents.Step{Type: "action", ToolName: "computer_observe", ToolResult: documentJSON(computer.NativeView{Observation: computer.ControlObservation{ID: "before-discovery", Target: target}})}
	for _, stepType := range []string{"action", "tool_error"} {
		for _, code := range []string{"computer_document_incomplete", "computer_read_interrupted", "computer_document_unavailable"} {
			result := `{"status":"failed","code":"` + code + `","effects":"none"}`
			if stepType == "action" {
				result = strings.Replace(unavailableDocumentCapabilityResult, "computer_document_unavailable", code, 1)
			}
			task.Steps = []agents.Step{observed, {Type: stepType, ToolName: "computer_documents", ToolResult: result}}
			if nativeDocumentControlFallback(task) != (code == "computer_document_unavailable") {
				t.Fatal("unsafe fallback", stepType, code)
			}
		}
	}
	task.Steps = append(task.Steps, agents.Step{Type: "action", ToolName: "computer_focus_window"})
	if nativeDocumentControlFallback(task) {
		t.Fatal("previous target limitation reused")
	}
	task.Steps = append(task.Steps, agents.Step{Type: "rejected", ToolName: "computer_documents", ToolResult: `{"error":"computer_document_unavailable"}`})
	if nativeDocumentControlFallback(task) {
		t.Fatal("rejected proposal treated as observed support")
	}
	for _, later := range []agents.Step{
		{Type: "action", ToolName: "computer_windows"},
		{Type: "action", ToolName: "computer_open_application"},
		{Type: "action", ToolName: "computer_verify_application"},
		{Type: "action", ToolName: "computer_documents", ToolResult: `{"complete":true}`},
		{Type: "tool_error", ToolName: "computer_documents", ToolResult: `{"status":"failed","code":"computer_document_incomplete","effects":"none"}`},
	} {
		task.Steps = []agents.Step{observed, {Type: "action", ToolName: "computer_documents", ToolResult: unavailableDocumentCapabilityResult}, later}
		if nativeDocumentControlFallback(task) {
			t.Fatal("old capability result crossed selection/discovery boundary", later)
		}
	}
	for _, result := range []string{
		`{"error":"computer_document_unavailable"}`,
		`{"status":"failed","code":"computer_document_unavailable","effects":"unknown"}`,
		`{"status":"unavailable","capability":"native_documents","supported":true,"code":"computer_document_unavailable","effects":"none"}`,
	} {
		task.Steps = []agents.Step{observed, {Type: "action", ToolName: "computer_documents", ToolResult: result}}
		if nativeDocumentControlFallback(task) {
			t.Fatal("untyped/uncertain capability result enabled fallback", result)
		}
	}
}

func TestDocumentControlFallbackCannotCrossObservedTargetChanges(t *testing.T) {
	target := computer.TargetIdentity{ID: "window", OSSession: "os", ProcessGeneration: "process", Surface: "surface", Driver: "windows-uia"}
	observe := func(target computer.TargetIdentity) agents.Step {
		return agents.Step{Type: "action", ToolName: "computer_observe", ToolResult: documentJSON(computer.NativeView{Observation: computer.ControlObservation{ID: "fresh", Target: target}})}
	}
	unsupported := agents.Step{Type: "action", ToolName: "computer_documents", ToolResult: unavailableDocumentCapabilityResult}
	for _, change := range []string{"same-target", "window", "os-session", "process", "surface", "driver", "invalid", "missing-before", "new-session-boundary"} {
		t.Run(change, func(t *testing.T) {
			next := target
			task := &agents.Task{Steps: []agents.Step{observe(target), unsupported}}
			switch change {
			case "window":
				next.ID = "replacement"
			case "os-session":
				next.OSSession = "replacement"
			case "process":
				next.ProcessGeneration = "replacement"
			case "surface":
				next.Surface = "replacement"
			case "driver":
				next.Driver = "macos-accessibility"
			case "invalid":
				next = computer.TargetIdentity{}
			case "missing-before":
				task.Steps = task.Steps[1:]
			case "new-session-boundary":
				task.Config.ComputerStepOffset = len(task.Steps)
			}
			task.Steps = append(task.Steps, observe(next))
			if nativeDocumentControlFallback(task) != (change == "same-target") {
				t.Fatal("negative capability crossed target/session boundary", change)
			}
			if change != "same-target" {
				// Returning to the old surface cannot restore an invalidated result.
				task.Steps = append(task.Steps, observe(target))
				if nativeDocumentControlFallback(task) {
					t.Fatal("old negative capability resurrected")
				}
			}
		})
	}
}
