package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/computer"
)

func TestJobSnapshotReportsSteeringFromActiveStateNotChildHistory(t *testing.T) {
	task := &agents.Task{Status: agents.TaskInterrupted, Checkpoint: &agents.Checkpoint{}, ChildHistory: []agents.ChildLink{{ID: "finished-child"}}}
	if taskResponse(task)["can_steer"] != true {
		t.Fatal("completed children hid follow-up")
	}
	task.Delegation = &agents.Delegation{CallID: "active"}
	if taskResponse(task)["can_steer"] != false {
		t.Fatal("active delegation permits steering")
	}
	task.Delegation = nil
	task.Checkpoint.ExecutingCall = "uncertain"
	if taskResponse(task)["can_steer"] != false {
		t.Fatal("uncertain action permits steering")
	}
}

func TestJobCommandsShareOwnerScopedRunnerAndExport(t *testing.T) {
	s := taskFirstServer(t)
	cfg := agents.DefaultAgentConfig()
	cfg.TaskFirst = true
	cfg.ContextWindow = 8192
	task, err := s.agentRunner.Create("saved task", "model", "local-admin", cfg)
	if err != nil {
		t.Fatal(err)
	}
	command := func(id, action, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		s.handleJobRead(w, httptest.NewRequest("POST", "/api/v2/jobs/"+id+"/"+action, strings.NewReader(body)))
		return w
	}
	if w := command(task.ID, "pause", `{}`); w.Code != 202 {
		t.Fatal(w.Body.String())
	}
	for i := 0; i < 2; i++ {
		if w := command(task.ID, "steer", `{"request_id":"instruction-001","instruction":"Updated task"}`); w.Code != 202 {
			t.Fatal(w.Body.String())
		}
	}
	stored, _ := s.agentManager.GetTask(task.ID)
	if len(stored.Instructions) != 1 {
		t.Fatal("duplicate instruction")
	}
	if w := command(task.ID, "steer", `{"request_id":"instruction-001","instruction":"Different task"}`); w.Code != 409 {
		t.Fatal("conflicting instruction")
	}
	if w := command(task.ID, "steer", `{} {}`); w.Code != 400 {
		t.Fatal("trailing request accepted")
	}
	foreign, _ := s.agentRunner.Create("private task", "model", "bob", cfg)
	if w := command(foreign.ID, "pause", `{}`); w.Code != 404 {
		t.Fatal("cross-actor pause")
	}
	w := httptest.NewRecorder()
	s.handleJobRead(w, httptest.NewRequest("GET", "/api/v2/jobs/"+task.ID+"/export", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Updated task") || !strings.Contains(w.Header().Get("Content-Disposition"), task.ID) || strings.Contains(w.Body.String(), `"checkpoint"`) {
		t.Fatalf("invalid export %s", w.Body.String())
	}
	if w = command(task.ID, "reconnect", `{"mode":"app","target":"Editor"}`); w.Code != 202 {
		t.Fatal(w.Body.String())
	}
	stored, _ = s.agentManager.GetTask(task.ID)
	if stored.Status != agents.TaskInput || stored.PendingInput.Target != "Editor" {
		t.Fatal("new consent absent")
	}
	if w = command(task.ID, "cancel", `{}`); w.Code != 202 {
		t.Fatal(w.Body.String())
	}
	w = httptest.NewRecorder()
	s.handleJobRead(w, httptest.NewRequest("DELETE", "/api/v2/jobs/"+task.ID, nil))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
}

func TestComputerMetadataAndEarlierTargetsCannotManufactureEvidence(t *testing.T) {
	target := computer.TargetIdentity{ID: "window", OSSession: "os", ProcessGeneration: "generation", Surface: "surface", Driver: "windows-uia"}
	value := "Saved text"
	view := computer.NativeView{Observation: computer.ControlObservation{ID: "fresh", Target: target}, Elements: []computer.NativeElement{{ID: "editor", Text: &value}}}
	observed, _ := json.Marshal(view)
	args := `{"observation_id":"fresh","element":"editor","text":"Saved text"}`
	verified, _ := json.Marshal(map[string]any{"verified": true, "check": "control_text_equals", "observation_id": "fresh", "element": "editor", "text": value, "target": target})
	task := &agents.Task{Config: agents.AgentConfig{ComputerDriver: target.Driver, ComputerSession: "session"}, Steps: []agents.Step{{Type: "action", ToolName: "computer_observe", ToolResult: string(observed)}, {Type: "action", ToolName: "computer_verify", ToolArgs: args, ToolResult: string(verified)}, {Type: "task_metadata", ToolName: "task_plan", ToolResult: `{"saved":true}`}}}
	b := &browserRunTools{}
	if err := b.ValidateCompletion(task); err != nil {
		t.Fatal(err)
	}
	task.Config.ComputerStepOffset = 2
	if err := b.ValidateCompletion(task); err == nil {
		t.Fatal("old target verification accepted for new target")
	}
	task.Config.ComputerStepOffset = 0
	task.Steps[1].ToolResult = `{"verified":false}`
	if err := b.ValidateCompletion(task); err == nil {
		t.Fatal("model plan overrides failed verification")
	}
}

func TestApplicationHandoffRequiresVerifiedCurrentTarget(t *testing.T) {
	s := taskFirstServer(t)
	cfg := agents.DefaultAgentConfig()
	cfg.TaskFirst = true
	cfg.ComputerSession = "existing"
	cfg.ComputerDriver = "windows-uia"
	task, _ := s.agentRunner.Create("Editor then browser", "model", "local-admin", cfg)
	b := &browserRunTools{RunTools: agents.NewToolRegistry(), server: s}
	descriptor := computerAccessDescriptor()
	if err := b.Authorize(context.Background(), "shell", json.RawMessage(`{"command":"whoami"}`), agents.ToolExecution{Actor: task.Actor, RunID: task.ID, Approved: true}); err == nil {
		t.Fatal("shell fallback bypassed task-first boundary")
	}
	err := b.Authorize(context.Background(), computerAccessToolName, json.RawMessage(`{"mode":"app","target":"Chrome","url":""}`), agents.ToolExecution{Actor: task.Actor, RunID: task.ID, ExpectedCapability: &descriptor})
	if err == nil || !strings.Contains(err.Error(), "verify") {
		t.Fatalf("unverified handoff accepted: %v", err)
	}
	// The tool remains available for same-run handoffs, including browser-only tests without a registry.
	for _, driver := range []string{"windows-uia", "browser"} {
		task.Config.ComputerDriver = driver
		found := false
		for _, tool := range b.ToolsForTask(task) {
			if tool.Function.Name == computerAccessToolName {
				found = true
			}
		}
		if !found {
			t.Fatal("handoff unavailable")
		}
	}
}
