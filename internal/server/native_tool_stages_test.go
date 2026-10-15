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
	task.Config.ComputerSession, task.Config.ComputerScope, task.Config.NativeWorkflowVersion = session.ID, "desktop", agents.DocumentWorkflowEvidenceVersion
	task.Steps = []agents.Step{{ID: 1, Type: "action", ToolName: "computer_applications", ToolResult: `{"applications":[{"id":"app","name":"Editor"}]}`}, {ID: 2, Type: "action", ToolName: "computer_verify_application", ToolResult: `{"verified":true,"target":{"id":"window"}}`}}
	check := func(catalog bool) {
		t.Helper()
		names := map[string]bool{}
		for _, tool := range adapter.ToolsForTaskContext(context.Background(), task) {
			names[tool.Function.Name] = true
		}
		if !names["computer_documents"] || names["computer_applications"] != catalog || !names["mcp__docs__docs"] {
			t.Fatal(names)
		}
	}
	check(false)
	task.Steps = append(task.Steps, agents.Step{ID: 3, Type: "tool_error", ToolName: "computer_documents", ToolResult: `{"error":"computer_document_unavailable"}`})
	check(true)
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
	for _, stepType := range []string{"action", "tool_error"} {
		for _, code := range []string{"computer_document_incomplete", "computer_read_interrupted", "computer_document_unavailable"} {
			task.Steps = []agents.Step{{Type: stepType, ToolName: "computer_documents", ToolResult: `{"error":"` + code + `"}`}}
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
}
