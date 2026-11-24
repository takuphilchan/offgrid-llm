package agents

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestBrowserStructuredReadRepairAndRestart(t *testing.T) {
	for _, name := range []string{"browser_search", "browser_frames", "browser_frame_select"} {
		t.Run(name, func(t *testing.T) {
			tools := &browserRecoveryTools{source: "local-companion"}
			tools.authorize = func(string, json.RawMessage, ToolExecution) error { return nil }
			tools.execute = func(string, json.RawMessage, ToolExecution) (string, error) {
				return "", &ToolFailure{Code: "computer_browser_observation_failed", BeforeEffects: true, Recovery: "Inspect the frame again"}
			}
			dir := t.TempDir()
			calls := 0
			r := NewRunner(NewManagerWithPersistence(nil, nil, nil, dir), tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
				calls++
				args := map[string]string{"browser_search": `{"role":"button","name":"","text":""}`, "browser_frames": `{}`, "browser_frame_select": `{"frame_id":"frame","document_id":"document"}`}[name]
				return runAnswer(api.ChatMessage{ToolCalls: []api.ToolCall{{Function: api.FunctionCall{Name: name, Arguments: args}}}}), nil
			})
			cfg := observationConfig()
			cfg.ComputerDriver = "browser"
			cfg.ComputerScope = ""
			task, err := r.Create("Inspect frames", "model", "alice", cfg)
			if err != nil {
				t.Fatal(err)
			}
			got, err := r.Continue(context.Background(), task.ID, "alice", "start", "", false)
			if err != nil || got.Status != TaskInterrupted || got.ErrorCode != proposalRepairLimitCode || calls != 4 {
				t.Fatal("unbounded structured read repair", calls, got, err)
			}
			if err = r.Shutdown(context.Background()); err != nil {
				t.Fatal(err)
			}
			retained, ok := NewManagerWithPersistence(nil, nil, nil, dir).GetTask(task.ID)
			if !ok || !retained.Checkpoint.PreEffectRepair || retained.Checkpoint.ExecutingCall != "" {
				t.Fatalf("read failure identity lost on restart: found=%t checkpoint=%+v", ok, retained.Checkpoint)
			}
			d, _ := tools.Capability(name)
			if !readOnlyBuiltin(d) || !trustedBrowserRepair(d, &ToolFailure{Code: "computer_browser_observation_failed", BeforeEffects: true}) {
				t.Fatal("read not pinned")
			}
			if failure := readFailure(errors.New("computer_browser_observation_failed")); trustedBrowserRepair(d, failure) {
				t.Fatal("untyped provider error became trusted repair evidence")
			}
		})
	}
}

func TestBrowserStructuredObservationCannotBeForgedOrSatisfiedByInventory(t *testing.T) {
	tools := &browserRecoveryTools{source: "local-companion"}
	for _, tc := range []struct {
		name, result, source string
		observed             bool
	}{
		{"browser_search", `{"observation_id":"fresh"}`, "local-companion", true},
		{"browser_frame_select", `{"observation_id":"fresh"}`, "local-companion", true},
		{"browser_frames", `{"observation_id":"fresh"}`, "local-companion", false},
		{"browser_search", `{"observation_id":""}`, "local-companion", false},
		{"browser_search", `{"text":"visible"}`, "local-companion", false},
		{"browser_search", `{"observation_id":"fresh"}`, "mcp:local-companion", false},
	} {
		tools.source = tc.source
		step := Step{Type: "action", ToolName: tc.name, ToolResult: tc.result}
		if structuredBrowserObservation(step, tools) != tc.observed {
			t.Fatal("observation proof", tc)
		}
		step.Type = "tool_error"
		if structuredBrowserObservation(step, tools) {
			t.Fatal("failure satisfied observation")
		}
	}
	tools.source = "mcp:local-companion"
	descriptor, _ := tools.Capability("browser_search")
	if structuredBrowserRead("browser_search", descriptor) {
		t.Fatal("provider tool gained browser prerequisite exemption")
	}
}

func TestBrowserStructuredReadIntentSurvivesRestart(t *testing.T) {
	for _, name := range []string{"browser_search", "browser_frames", "browser_frame_select"} {
		for _, pinned := range []bool{false, true} {
			dir := t.TempDir()
			m, task := receiptFixture(t, dir)
			_, err := m.updateTask(task.ID, func(task *Task) error {
				task.Config.ComputerDriver = "browser"
				cp := task.Checkpoint
				cp.Calls[0].Function = api.FunctionCall{Name: name, Arguments: `{}`}
				cp.ExecutingCapability = &capabilities.Descriptor{Name: name, Namespace: "computer", Source: "local-companion", Kind: capabilities.Computer, Risk: capabilities.RiskLow, Revision: capabilities.BrowserEffectsRevision}
				if pinned {
					cp.ReadOnlyCall = "call"
					cp.ReadOnlyCapability = cp.ExecutingCapability
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			got, _ := NewManagerWithPersistence(nil, nil, nil, dir).GetTask(task.ID)
			if !pinned && got.Status != TaskUncertain || pinned && (got.Status != TaskInterrupted || got.Checkpoint.ExecutingCall != "") {
				t.Fatal("durable read boundary", name, pinned, got.Status)
			}
		}
	}
}
