package agents

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// This double tests runner metadata/persistence boundaries, not native provider
// validation. Server tests own derivation from actual canonical observations.
type actionContractTools struct {
	runTestTools
	derive func(*Task, api.ToolCall) (*OutcomeExpectation, error)
	bind   func(*Task) error
}

func (*actionContractTools) GetTools() []api.Tool {
	return []api.Tool{{Type: "function", Function: api.FunctionDef{Name: "computer_open_application", Parameters: map[string]any{"type": "object", "properties": map[string]any{"application_id": map[string]any{"type": "string"}}, "required": []string{"application_id"}, "additionalProperties": false}}}}
}

func (*actionContractTools) Capability(name string) (capabilities.Descriptor, bool) {
	return capabilities.Descriptor{Name: name, Source: "local-companion", Namespace: "computer", Kind: capabilities.Computer, Risk: capabilities.RiskHigh, Revision: capabilities.NativeEffectsRevision}, true
}

func (tools *actionContractTools) DeriveNativeActionExpectation(task *Task, call api.ToolCall) (*OutcomeExpectation, error) {
	if tools.derive != nil {
		return tools.derive(task, call)
	}
	var args struct {
		Application string `json:"application_id"`
	}
	if json.Unmarshal([]byte(call.Function.Arguments), &args) != nil || args.Application == "" {
		return nil, errors.New("fixture arguments invalid")
	}
	return &OutcomeExpectation{Kind: "application_open", Title: "Application ready", ApplicationID: args.Application}, nil
}

func (tools *actionContractTools) BindWorkflowExpectations(task *Task) error {
	if tools.bind != nil {
		return tools.bind(task)
	}
	for i := range task.Workflow.Checks {
		if task.Workflow.Checks[i].Binding == nil {
			task.Workflow.Checks[i].Binding = &OutcomeBinding{Session: task.Config.ComputerSession, TargetSHA256: strings.Repeat("a", 64), Observation: "fixture-observation", Step: 1}
		}
	}
	return nil
}

func actionContractTask(t *testing.T) *Task {
	t.Helper()
	task := progressTask()
	task.ID, task.Prompt, task.Model = "run-fixture", "Open the requested application", "model"
	if err := initializeWorkflow(task); err != nil {
		t.Fatal(err)
	}
	return task
}

func actionContractCall(id, application string) api.ToolCall {
	return api.ToolCall{ID: id, Type: "function", Function: api.FunctionCall{Name: "computer_open_application", Arguments: progressJSON(map[string]string{"application_id": application})}}
}

func TestNativeActionCheckDerivationIsIdempotentAndBound(t *testing.T) {
	task := actionContractTask(t)
	tools := &actionContractTools{}
	runner := NewRunner(nil, tools, nil)
	call := actionContractCall("canonical-call", "editor")
	if err := runner.prepareNativeActionCheck(task, call); err != nil {
		t.Fatal(err)
	}
	before := progressJSON(task.Workflow)
	if len(task.Workflow.Checks) != 1 || task.Workflow.Checks[0].ActionDerivedCallID != call.ID || task.Workflow.Checks[0].Binding == nil || task.Workflow.Checks[0].State != "pending" {
		t.Fatal("action did not acquire a bound pending check", task.Workflow)
	}
	if err := runner.prepareNativeActionCheck(task, call); err != nil || progressJSON(task.Workflow) != before {
		t.Fatal("same invocation changed immutable evidence", err)
	}
	// Reusing a call identity for different arguments cannot reuse the old check.
	changed := actionContractCall(call.ID, "different-app")
	if err := runner.prepareNativeActionCheck(task, changed); err == nil || progressJSON(task.Workflow) != before {
		t.Fatal("changed invocation reused or modified its saved check", err)
	}
	tools.derive = func(*Task, api.ToolCall) (*OutcomeExpectation, error) { return nil, nil }
	if err := runner.prepareNativeActionCheck(task, call); err == nil || progressJSON(task.Workflow) != before {
		t.Fatal("existing derived invocation silently lost its mechanical predicate", err)
	}
}

func TestNativeActionCheckRetainsExplicitChecksAndRejectsConflicts(t *testing.T) {
	for _, conflict := range []bool{false, true} {
		t.Run(map[bool]string{false: "equal", true: "conflict"}[conflict], func(t *testing.T) {
			task := actionContractTask(t)
			text := "requested text"
			expectation := OutcomeExpectation{ID: "explicit", Title: "Exact user text", Kind: "control_text_equals", TargetID: "window", Element: "control", Text: &text}
			if err := declareExpectations(task, []OutcomeExpectation{expectation}, "caller_explicit"); err != nil {
				t.Fatal(err)
			}
			before := progressJSON(task.Workflow)
			tools := &actionContractTools{derive: func(*Task, api.ToolCall) (*OutcomeExpectation, error) {
				result := expectation
				if conflict {
					other := "different text"
					result.Text = &other
				}
				return &result, nil
			}}
			err := NewRunner(nil, tools, nil).prepareNativeActionCheck(task, actionContractCall("action", "editor"))
			if (err != nil) != conflict || progressJSON(task.Workflow) != before || len(task.Workflow.Checks) != 1 || task.Workflow.Checks[0].ActionDerivedCallID != "" {
				t.Fatal("explicit check changed or conflict ignored", err)
			}
		})
	}
}

func TestNativeActionCheckFailureDoesNotMutateExistingEvidence(t *testing.T) {
	for _, failure := range []string{"derive", "bind"} {
		t.Run(failure, func(t *testing.T) {
			task := actionContractTask(t)
			text := "original"
			if err := declareExpectations(task, []OutcomeExpectation{{ID: "existing", Title: "Existing content", Kind: "control_text_equals", TargetID: "window", Element: "old-control", Text: &text}}, "caller_explicit"); err != nil {
				t.Fatal(err)
			}
			task.Workflow.Checks[0].Binding = &OutcomeBinding{Session: "session", Observation: "old", Step: 1}
			before := progressJSON(task.Workflow)
			mutateThenFail := func(candidate *Task) error {
				*candidate.Workflow.Checks[0].Expectation.Text = "must-not-leak"
				candidate.Workflow.Checks[0].Binding.Step = 999
				return errors.New("injected preparation failure")
			}
			tools := &actionContractTools{}
			if failure == "derive" {
				tools.derive = func(candidate *Task, _ api.ToolCall) (*OutcomeExpectation, error) {
					return nil, mutateThenFail(candidate)
				}
			} else {
				tools.bind = mutateThenFail
			}
			if err := NewRunner(nil, tools, nil).prepareNativeActionCheck(task, actionContractCall("action", "editor")); err == nil {
				t.Fatal("preparation failure ignored")
			}
			if progressJSON(task.Workflow) != before {
				t.Fatal("failed preparation mutated existing canonical evidence")
			}
		})
	}
}

func TestNativeActionCheckOwnsDerivedTextAndCheckedValues(t *testing.T) {
	for _, kind := range []string{"control_text_equals", "control_checked_equals"} {
		t.Run(kind, func(t *testing.T) {
			task := actionContractTask(t)
			text, checked := "requested", true
			expectation := &OutcomeExpectation{Kind: kind, TargetID: "window", Element: "control", Title: "Requested value"}
			if kind == "control_text_equals" {
				expectation.Text = &text
			} else {
				expectation.Checked = &checked
			}
			tools := &actionContractTools{derive: func(*Task, api.ToolCall) (*OutcomeExpectation, error) { return expectation, nil }}
			if err := NewRunner(nil, tools, nil).prepareNativeActionCheck(task, actionContractCall("action", "editor")); err != nil {
				t.Fatal(err)
			}
			before := progressJSON(task.Workflow)
			text, checked = "changed-after-return", false
			if progressJSON(task.Workflow) != before {
				t.Fatal("provider-owned pointer can mutate the derived check")
			}
		})
	}
}

func TestNativeActionCheckLegacyAndUnknownVersionsDoNotDerive(t *testing.T) {
	for _, version := range []int{0, 2} {
		task := actionContractTask(t)
		task.Config.NativeActionContractVersion = version
		before := progressJSON(task.Workflow)
		tools := &actionContractTools{derive: func(*Task, api.ToolCall) (*OutcomeExpectation, error) {
			t.Fatal("unselected or unknown contract invoked derivation")
			return nil, nil
		}}
		if err := NewRunner(nil, tools, nil).prepareNativeActionCheck(task, actionContractCall("action", "editor")); err != nil || progressJSON(task.Workflow) != before {
			t.Fatal("legacy task changed", err)
		}
	}
}

func TestNativeActionCheckRequiresSafePendingIdentity(t *testing.T) {
	for _, invalid := range []string{"missing-id", "executing", "missing-checkpoint"} {
		t.Run(invalid, func(t *testing.T) {
			task := actionContractTask(t)
			call := actionContractCall("action", "editor")
			switch invalid {
			case "missing-id":
				call.ID = ""
			case "executing":
				task.Checkpoint.ExecutingCall = "in-flight"
			case "missing-checkpoint":
				task.Checkpoint = nil
			}
			tools := &actionContractTools{derive: func(*Task, api.ToolCall) (*OutcomeExpectation, error) {
				t.Fatal("unsafe identity reached derivation")
				return nil, nil
			}}
			before := progressJSON(task.Workflow)
			if err := NewRunner(nil, tools, nil).prepareNativeActionCheck(task, call); err == nil || progressJSON(task.Workflow) != before {
				t.Fatal("unsafe identity changed metadata", err)
			}
			if invalid == "missing-checkpoint" {
				discardRejectedActionCheck(task, call.ID)
				if progressJSON(task.Workflow) != before {
					t.Fatal("malformed checkpoint allowed evidence removal")
				}
			}
		})
	}
}

func TestNativeActionUnknownContractCannotAdmitExecution(t *testing.T) {
	tools := &actionContractTools{}
	runner := NewRunner(NewManagerWithPersistence(nil, nil, nil, t.TempDir()), tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		t.Fatal("unknown action contract reached inference")
		return nil, nil
	})
	cfg := actionContractTask(t).Config
	cfg.NativeActionContractVersion = NativeActionContractVersion + 1
	if _, err := runner.Create("Open Editor", "model", "alice", cfg); err == nil || tools.calls.Load() != 0 {
		t.Fatal("unknown contract admitted execution", err)
	}
}

func TestDiscardRejectedActionCheckPreservesExplicitDispatchedAndUncertain(t *testing.T) {
	for _, protected := range []string{"explicit", "executing", "verified", "evidence", "dispatched-binding", "dispatched-history", "none"} {
		t.Run(protected, func(t *testing.T) {
			task := actionContractTask(t)
			call := actionContractCall("canonical-call", "editor")
			if err := NewRunner(nil, &actionContractTools{}, nil).prepareNativeActionCheck(task, call); err != nil {
				t.Fatal(err)
			}
			check := &task.Workflow.Checks[0]
			switch protected {
			case "explicit":
				check.ActionDerivedCallID = ""
			case "executing":
				task.Checkpoint.ExecutingCall = call.ID
			case "verified":
				check.State = "verified"
			case "evidence":
				check.EvidenceSteps = []int{1}
			case "dispatched-binding":
				check.Binding.Document = &OutcomeDocumentBinding{ActionID: "dispatched-action"}
			case "dispatched-history":
				task.Steps = []Step{{ID: 1, CallID: call.ID, Type: "action", ToolName: call.Function.Name, ToolArgs: call.Function.Arguments, ToolResult: `{"dispatched":true}`}}
			}
			before := progressJSON(task.Workflow)
			discardRejectedActionCheck(task, call.ID)
			if protected == "none" {
				if len(task.Workflow.Checks) != 0 {
					t.Fatal("proven pre-effect rejected check not removed")
				}
			} else if progressJSON(task.Workflow) != before {
				t.Fatal("explicit/dispatched/uncertain check discarded")
			}
		})
	}
}

func TestNativeActionCheckPersistsBeforeAuthorizationAndDispatch(t *testing.T) {
	manager := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	tools := &actionContractTools{}
	authorized, executed := 0, 0
	assertSaved := func(execution ToolExecution, inFlight bool) {
		t.Helper()
		saved, ok := manager.GetTask(execution.RunID)
		if !ok || saved.Workflow == nil || len(saved.Workflow.Checks) != 1 || saved.Workflow.Checks[0].ActionDerivedCallID != execution.CallID || saved.Workflow.Checks[0].Binding == nil || len(saved.Checkpoint.Calls) != 1 || saved.Checkpoint.Calls[0].ID != execution.CallID {
			t.Fatal("mechanical check and queued call not durably available before external boundary")
		}
		if (saved.Checkpoint.ExecutingCall == execution.CallID) != inFlight {
			t.Fatal("incorrect intent persistence boundary", saved.Checkpoint.ExecutingCall)
		}
	}
	tools.authorize = func(_ string, _ json.RawMessage, execution ToolExecution) error {
		authorized++
		assertSaved(execution, false)
		return nil
	}
	tools.execute = func(_ string, _ json.RawMessage, execution ToolExecution) (string, error) {
		executed++
		assertSaved(execution, true)
		return `{"dispatched":true}`, nil
	}
	calls := 0
	runner := NewRunner(manager, tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		if calls > 1 {
			return nil, context.Canceled // No fabricated verification/completion.
		}
		return taskCall("computer_open_application", `{"application_id":"editor"}`), nil
	})
	cfg := actionContractTask(t).Config
	task, err := runner.Create("Open Editor", "model", "alice", cfg)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := runner.Continue(context.Background(), task.ID, "alice", "start", "", false)
	if authorized != 1 || executed != 1 || got == nil || got.Status == TaskCompleted {
		t.Fatal("fixture did not reach the guarded boundaries exactly once", authorized, executed)
	}
}

func TestNativeActionExistingExplicitCheckBoundBeforeAuthorization(t *testing.T) {
	manager := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	tools := &actionContractTools{}
	authorized := false
	tools.authorize = func(_ string, _ json.RawMessage, execution ToolExecution) error {
		authorized = true
		saved, ok := manager.GetTask(execution.RunID)
		if !ok || len(saved.Workflow.Checks) != 1 || saved.Workflow.Checks[0].Expectation.ID != "explicit" || saved.Workflow.Checks[0].ActionDerivedCallID != "" || saved.Workflow.Checks[0].Binding == nil || saved.Checkpoint.ExecutingCall != "" {
			t.Fatal("reused explicit check was not durably bound before authorization")
		}
		return capabilities.ErrDenied // Metadata must not grant execution.
	}
	runner := NewRunner(manager, tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		return taskCall("computer_open_application", `{"application_id":"editor"}`), nil
	})
	cfg := actionContractTask(t).Config
	cfg.NativeExpectations = []OutcomeExpectation{{ID: "explicit", Title: "Editor ready", Kind: "application_open", ApplicationID: "editor"}}
	task, err := runner.Create("Open Editor", "model", "alice", cfg)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = runner.Continue(context.Background(), task.ID, "alice", "start", "", false)
	if !authorized || tools.calls.Load() != 0 {
		t.Fatal("authorization was skipped or metadata granted execution")
	}
}

func TestNativeActionCheckPersistenceFailureAndUncertainEffectNeverReplay(t *testing.T) {
	for _, phase := range []string{"check", "intent", "result"} {
		t.Run(phase, func(t *testing.T) {
			dir := t.TempDir()
			manager := NewManagerWithPersistence(nil, nil, nil, dir)
			blocked := filepath.Join(t.TempDir(), "not-a-directory")
			if err := os.WriteFile(blocked, []byte("fixture"), 0600); err != nil {
				t.Fatal(err)
			}
			breakStorage := func() { manager.mu.Lock(); manager.dataDir = blocked; manager.mu.Unlock() }
			tools := &actionContractTools{}
			if phase == "check" {
				tools.derive = func(_ *Task, _ api.ToolCall) (*OutcomeExpectation, error) {
					breakStorage()
					return &OutcomeExpectation{Kind: "application_open", Title: "Ready", ApplicationID: "editor"}, nil
				}
			}
			authorizations := 0
			tools.authorize = func(string, json.RawMessage, ToolExecution) error {
				authorizations++
				if phase == "intent" {
					breakStorage()
				}
				return nil
			}
			tools.execute = func(string, json.RawMessage, ToolExecution) (string, error) {
				breakStorage()
				return `{"dispatched":true}`, nil
			}
			runner := NewRunner(manager, tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
				return taskCall("computer_open_application", `{"application_id":"editor"}`), nil
			})
			task, err := runner.Create("Open Editor", "model", "alice", actionContractTask(t).Config)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = runner.Continue(context.Background(), task.ID, "alice", "start", "", false); !errors.Is(err, ErrRunStorage) {
				t.Fatal("persistence failure hidden", err)
			}
			wantCalls := int32(0)
			if phase == "result" {
				wantCalls = 1
			}
			if tools.calls.Load() != wantCalls || phase == "check" && authorizations != 0 {
				t.Fatal("external boundary crossed before required persistence", tools.calls.Load(), authorizations)
			}
			restored := NewManagerWithPersistence(nil, nil, nil, dir)
			saved, ok := restored.GetTask(task.ID)
			if !ok || restored.StorageError() != nil {
				t.Fatal("lost durable state", restored.StorageError())
			}
			if phase == "result" && (saved.Status != TaskUncertain || saved.Checkpoint.ExecutingCall == "" || len(saved.Workflow.Checks) != 1) {
				t.Fatal("dispatched action/check lost uncertainty", saved.Status)
			}
			if phase != "result" && saved.Status == TaskUncertain {
				t.Fatal("fabricated effects before dispatch intent")
			}
			before := progressJSON(saved.Workflow)
			if _, err = NewRunner(restored, tools, nil).Continue(context.Background(), task.ID, "alice", "resume", "", false); err == nil {
				t.Fatal("replayed without fresh authority/reconciliation")
			}
			after, _ := restored.GetTask(task.ID)
			if tools.calls.Load() != wantCalls || !reflect.DeepEqual(progressJSON(after.Workflow), before) {
				t.Fatal("recovery repeated input or rewrote existing check")
			}
		})
	}
}
