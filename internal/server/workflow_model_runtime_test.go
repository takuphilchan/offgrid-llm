package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/computer"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// This provider cannot reach the computer hub: all observations are synthetic,
// and every mutation is refused even if the model proposes it.
type declarationModelFixture struct {
	*browserRunTools
	view    computer.NativeView
	manager *agents.Manager
}

func (f *declarationModelFixture) GetTools() []api.Tool {
	return append(nativeComputerTools(), desktopComputerTools()...)
}
func (f *declarationModelFixture) ToolsForTaskContext(context.Context, *agents.Task) []api.Tool {
	return f.GetTools()
}
func (f *declarationModelFixture) Capability(name string) (capabilities.Descriptor, bool) {
	return capabilities.Descriptor{Name: name, Namespace: "computer", Source: "synthetic-fixture", Kind: capabilities.Computer}, true
}
func (f *declarationModelFixture) Authorize(_ context.Context, name string, args json.RawMessage, execution agents.ToolExecution) error {
	task, ok := f.manager.GetTask(execution.RunID)
	if !ok {
		return agents.ErrTaskNotFound
	}
	if err := f.ValidateWorkflowEdit(task, name, args); err != nil {
		return safeComputerAuthorization(err)
	}
	if name == "computer_applications" || name == "computer_observe" || name == "computer_verify_application" || name == "computer_verify" || name == "computer_verify_checked" || nativeDiscoveryTool(name) {
		return nil
	}
	return fmt.Errorf("fixture refuses all input")
}
func (f *declarationModelFixture) ExecuteWithPolicy(_ context.Context, name string, args json.RawMessage, _ agents.ToolExecution) (string, error) {
	switch name {
	case "computer_applications":
		return `{"applications":[{"id":"fixture-editor","name":"Notepad"}]}`, nil
	case "computer_observe":
		b, _ := json.Marshal(f.view)
		return string(b), nil
	case "computer_verify_application":
		b, _ := json.Marshal(computer.ApplicationEvidence{Verified: true, Check: "application_open", Application: "fixture-editor", Target: &f.view.Observation.Target, View: &f.view})
		return string(b), nil
	case "computer_inspect_control":
		var q computer.ControlInspection
		if err := json.Unmarshal(args, &q); err != nil {
			return "", err
		}
		return computer.InspectNativeControl(f.view, q, time.Now().UTC())
	case "computer_find_controls":
		var q computer.ControlQuery
		if err := json.Unmarshal(args, &q); err != nil {
			return "", err
		}
		return computer.FindNativeControls(f.view, q, time.Now().UTC())
	case "computer_verify", "computer_verify_checked":
		// Independent synthetic read-back: no mutation has been permitted, so
		// verify the actual fixture value rather than the model's requested one.
		var a struct {
			Observation string  `json:"observation_id"`
			Element     string  `json:"element"`
			Text        *string `json:"text,omitempty"`
			Checked     *bool   `json:"checked,omitempty"`
		}
		if !decodeNativeModelResult(string(args), &a) {
			return "", computer.ErrInvalidControl
		}
		for _, element := range f.view.Elements {
			if a.Observation != f.view.Observation.ID || a.Element != element.ID {
				continue
			}
			kind, verified := "control_text_equals", false
			result := map[string]any{"target": f.view.Observation.Target, "element": element.ID, "observation_id": a.Observation}
			if name == "computer_verify" {
				verified = a.Text != nil && element.Text != nil && *a.Text == *element.Text
				result["text"] = element.Text
			} else {
				kind = "control_checked_equals"
				verified = a.Checked != nil && element.Checked != nil && *a.Checked == *element.Checked
				result["checked"] = element.Checked
			}
			result["verified"], result["check"] = verified, kind
			b, _ := json.Marshal(result)
			return string(b), nil
		}
		return "", computer.ErrInvalidControl
	default:
		return "", fmt.Errorf("fixture refuses dispatch")
	}
}

// Opt-in installed-model test, not an installed OS workflow qualification. No
// downloads, desktop input, user task writes or application launch are possible.
func TestWorkflowDeclarationInstalledModel(t *testing.T) {
	endpoint, model := os.Getenv("OFFGRID_TEST_COMPLETION_URL"), os.Getenv("OFFGRID_TEST_CONTEXT_MODEL")
	if endpoint == "" || model == "" {
		t.Skip("requires explicit installed-model endpoint; synthetic observations only")
	}
	for _, stage := range []string{"before_launch", "before_text_edit"} {
		t.Run(stage, func(t *testing.T) {
			empty := ""
			fixture := &declarationModelFixture{browserRunTools: &browserRunTools{}, view: computer.NativeView{
				Observation: computer.ControlObservation{ID: "fixture-observation", Target: computer.TargetIdentity{ID: "fixture-window", OSSession: "fixture-os", ProcessGeneration: "fixture-process", Surface: "fixture-surface", Driver: "windows-uia"}, Sequence: 1, CapturedAt: time.Now().UTC(), StateDigest: "fixture"},
				Elements:    []computer.NativeElement{{ID: "fixture-editor-control", Identity: "fixture-identity", Name: "Text Editor", Role: "edit", Writable: true, Text: &empty}},
			}}
			cfg := agents.DefaultAgentConfig()
			cfg.TaskFirst, cfg.NativeWorkflowVersion = true, agents.WorkflowEvidenceVersion
			cfg.ComputerScope, cfg.ComputerDriver, cfg.ComputerSession = "desktop", "windows-uia", "synthetic-session"
			cfg.ContextWindow, cfg.MaxIterations = 8192, 16
			configureDesktopTask(&cfg)
			if stage == "before_text_edit" {
				cfg.NativeExpectations = []agents.OutcomeExpectation{{ID: "open", Title: "Application ready", Kind: "application_open", ApplicationID: "fixture-editor"}}
			}
			modelCalls, accepted := 0, false
			start := time.Now()
			fixture.manager = agents.NewManagerWithPersistence(nil, nil, nil, t.TempDir())
			runner := agents.NewRunner(fixture.manager, fixture, func(ctx context.Context, task *agents.Task, messages []api.ChatMessage, tools []api.Tool) (*api.ChatCompletionResponse, error) {
				for _, c := range task.Workflow.Checks {
					e := c.Expectation
					if (stage == "before_launch" && e.Kind == "application_open" && e.ApplicationID == "fixture-editor") ||
						(stage == "before_text_edit" && e.Kind == "control_text_equals" && e.Text != nil && *e.Text == "OffGrid test — café 日本語 12345" && c.Binding != nil && c.Binding.Identity == "fixture-identity") {
						accepted = true
						return nil, context.Canceled // stop before any actual application input
					}
				}
				if len(task.Steps) == 0 {
					name := "computer_applications"
					args := `{}`
					if stage == "before_text_edit" {
						name, args = "computer_verify_application", `{"application_id":"fixture-editor"}`
					}
					return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "tool_calls", Message: api.ChatMessage{ToolCalls: []api.ToolCall{{Function: api.FunctionCall{Name: name, Arguments: args}}}}}}}, nil
				}
				modelCalls++
				zero, max := float32(0), 512
				request := &api.ChatCompletionRequest{Model: model, Messages: messages, Tools: tools, ToolChoice: "auto", Temperature: &zero, MaxTokens: &max}
				configureComputerRequest(request, task)
				body, _ := json.Marshal(request)
				req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
				if err != nil {
					return nil, err
				}
				req.Header.Set("Content-Type", "application/json")
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					return nil, err
				}
				defer resp.Body.Close()
				if resp.StatusCode != 200 {
					return nil, fmt.Errorf("model HTTP %d", resp.StatusCode)
				}
				var result api.ChatCompletionResponse
				err = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result)
				if err == nil {
					t.Logf("fixture token usage: prompt=%d completion=%d", result.Usage.PromptTokens, result.Usage.CompletionTokens)
				}
				if err == nil && result.Usage.PromptTokens+max+512 > cfg.ContextWindow {
					return nil, fmt.Errorf("prompt exceeds 8192-token budget")
				}
				if len(result.Choices) > 0 {
					for _, call := range result.Choices[0].Message.ToolCalls {
						t.Logf("fixture model call: %s %s", call.Function.Name, call.Function.Arguments)
					}
				}
				return &result, err
			})
			defer runner.Shutdown(context.Background())
			prompt := "Open Notepad. In a new blank document, type exactly: OffGrid test — café 日本語 12345. Read it back and confirm it matches. Do not save or change existing documents."
			if stage == "before_text_edit" {
				prompt = "Notepad is already open at a new blank document. Type exactly: OffGrid test — café 日本語 12345. Read it back and confirm it matches. Do not save or change existing documents."
			}
			task, err := runner.Create(prompt, model, "fixture-actor", cfg)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
			defer cancel()
			got, err := runner.Continue(ctx, task.ID, "fixture-actor", "start", "", false)
			if !accepted || !errors.Is(err, context.Canceled) {
				t.Fatalf("declaration not accepted: calls=%d err=%v status=%s code=%s", modelCalls, err, got.Status, got.ErrorCode)
			}
			t.Logf("stage=%s accepted; model_calls=%d elapsed=%s; no desktop dispatch", stage, modelCalls, time.Since(start))
		})
	}
}
