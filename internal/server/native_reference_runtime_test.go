package server

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/computer"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Real model with synthetic observations, no host, grant or native dispatch.
func TestSelectedWindowInstalledRuntimeReferencesAndNextObservation(t *testing.T) {
	base := os.Getenv("OFFGRID_TEST_SINGLE_STEP_RUNTIME")
	if base == "" {
		t.Skip("requires explicit idle local runtime; no host actions")
	}
	s := taskFirstServer(t)
	registry := agents.NewToolRegistry()
	for _, tool := range registry.GetTools() {
		if err := registry.DisableTool(tool.Function.Name); err != nil {
			t.Fatal(err)
		}
	}
	adapter := &browserRunTools{RunTools: registry, server: s}
	view := desktopContextFixture(computer.TargetIdentity{ID: "surface", OSSession: "os", ProcessGeneration: "process", Surface: "window", Driver: "windows-uia"}, "Microsoft Edge")
	view.Observation.ID = "0c6aa70ffed49954ee7cb0fb74b825d0"
	view.Elements[7].ID = "6e22646134272a01cc658659348b78fb"
	// This contract check isolates reference copying and post-input sequencing.
	// Large-observation pressure is covered separately, not by this small fixture.
	view.Elements = []computer.NativeElement{view.Elements[0], view.Elements[7], view.Elements[40]}
	data, _ := json.Marshal(view)
	cfg := agents.DefaultAgentConfig()
	cfg.TaskFirst, cfg.ComputerSession, cfg.ComputerDriver, cfg.SystemPrompt = true, "fixture", "windows-uia", taskFirstPrompt
	configureComputerTask(&cfg)
	task := &agents.Task{Config: cfg, Prompt: "In the selected Microsoft Edge window, enter tourism in Zimbabwe in the address/search bar. Do not submit yet.", Steps: []agents.Step{{ID: 1, Type: "action", ToolName: "computer_observe", ToolResult: string(data)}}}
	messages := []api.ChatMessage{{Role: "system", Content: cfg.SystemPrompt}, {Role: "user", Content: task.Prompt}, {Role: "assistant", ToolCalls: []api.ToolCall{{ID: "observe", Type: "function", Function: api.FunctionCall{Name: "computer_observe", Arguments: `{}`}}}}, {Role: "tool", ToolCallID: "observe", Name: "computer_observe", Content: string(data)}}
	for _, stage := range []string{"input", "observe_after_input"} {
		t.Run(stage, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			max, zero, parallel := 512, float32(0), false
			response, err := inference.NewLlamaHTTPEngine(base).ChatCompletion(ctx, &api.ChatCompletionRequest{Model: os.Getenv("OFFGRID_TEST_RUNTIME_MODEL"), Messages: adapter.ProjectModelContext(task, messages), Tools: adapter.ToolsForTask(task), ParallelToolCalls: &parallel, MaxTokens: &max, Temperature: &zero})
			if err != nil {
				t.Fatal(err)
			}
			if len(response.Choices) != 1 || len(response.Choices[0].Message.ToolCalls) != 1 {
				t.Fatal("expected exactly one tool decision")
			}
			call := response.Choices[0].Message.ToolCalls[0]
			if stage == "input" {
				var args struct {
					Observation string `json:"observation_id"`
					Element     string `json:"element"`
					Text        string `json:"text"`
				}
				if json.Unmarshal([]byte(call.Function.Arguments), &args) != nil || call.Function.Name != "computer_replace_text" || args.Observation != view.Observation.ID || args.Element != view.Elements[1].ID || args.Text != "tourism in Zimbabwe" {
					t.Fatalf("wrong proposed edit: %s %s", call.Function.Name, call.Function.Arguments)
				}
				call.ID = "synthetic-input"
				result := `{"actions":[{"outcome":"dispatched","uncertain":false}]}`
				task.Steps = append(task.Steps, agents.Step{ID: 2, Type: "action", ToolName: call.Function.Name, ToolArgs: call.Function.Arguments, ToolResult: result})
				messages = append(messages, api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{call}}, api.ChatMessage{Role: "tool", ToolCallID: call.ID, Name: call.Function.Name, Content: result})
			} else if call.Function.Name != "computer_observe" {
				t.Fatalf("input receipt led to %s, not fresh observation", call.Function.Name)
			}
			t.Logf("model proposed %s; no tool dispatched", call.Function.Name)
		})
		if t.Failed() {
			break
		}
	}
}
