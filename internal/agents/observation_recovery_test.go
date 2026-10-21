package agents

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type observationTestTools struct {
	runTestTools
	descriptor capabilities.Descriptor
}

func (t *observationTestTools) Capability(name string) (capabilities.Descriptor, bool) {
	if name == "task_save_artifact" {
		return t.descriptor, true
	}
	return capabilities.Descriptor{Name: name, Kind: capabilities.Computer, Namespace: "computer", Source: "local-companion", Risk: capabilities.RiskLow}, true
}

func observationConfig() AgentConfig {
	cfg := DefaultAgentConfig()
	cfg.TaskFirst, cfg.ComputerSession, cfg.ComputerDriver = true, "fixture-session", "windows-uia"
	cfg.ComputerScope, cfg.ComputerApprovalMode = "desktop", "full_task"
	cfg.ContextWindow, cfg.MaxIterations = 32768, 16
	return cfg
}

func observationCall(name string) *api.ChatCompletionResponse {
	return runAnswer(api.ChatMessage{ToolCalls: []api.ToolCall{{Function: api.FunctionCall{Name: name, Arguments: `{"text":"private-rejected-payload"}`}}}})
}

func TestDesktopObservationDoesNotGateWorkspaceArtifact(t *testing.T) {
	for _, denied := range []bool{false, true} {
		t.Run(map[bool]string{false: "authorized", true: "denied"}[denied], func(t *testing.T) {
			tools := &observationTestTools{descriptor: capabilities.Descriptor{Name: "task_save_artifact", Kind: capabilities.Write, Namespace: "workspace", Source: "workspace", Risk: capabilities.RiskLow}}
			authorizations := 0
			tools.authorize = func(string, json.RawMessage, ToolExecution) error {
				authorizations++
				if denied {
					return capabilities.ErrDenied
				}
				return nil
			}
			tools.execute = func(string, json.RawMessage, ToolExecution) (string, error) { return `{"verified":true}`, nil }
			calls := 0
			r := NewRunner(NewManagerWithPersistence(nil, nil, nil, t.TempDir()), tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
				calls++
				if calls == 1 {
					return observationCall("task_save_artifact"), nil
				}
				return runAnswer(api.ChatMessage{Content: "Saved artifact."}), nil
			})
			task, err := r.Create("Save a workspace draft", "model", "alice", observationConfig())
			if err != nil {
				t.Fatal(err)
			}
			got, err := r.Continue(context.Background(), task.ID, "alice", "start", "", false)
			if err != nil || authorizations == 0 {
				t.Fatalf("artifact never reached policy: task=%+v err=%v", got, err)
			}
			if denied {
				if got.Status != TaskFailed || tools.calls.Load() != 0 {
					t.Fatal("denial bypassed", got)
				}
			} else if got.Status != TaskCompleted || tools.calls.Load() != 1 {
				t.Fatalf("artifact blocked by desktop: %+v", got)
			}
		})
	}
}

func TestArtifactCannotCompleteRequestedComputerWorkWithoutObservation(t *testing.T) {
	tools := &observationTestTools{descriptor: capabilities.Descriptor{Name: "task_save_artifact", Kind: capabilities.Write, Namespace: "workspace", Source: "workspace", Risk: capabilities.RiskLow}}
	tools.authorize = func(string, json.RawMessage, ToolExecution) error { return nil }
	tools.execute = func(string, json.RawMessage, ToolExecution) (string, error) {
		return `{"verified":true,"check":"stored_bytes_sha256_and_format","sha256":"` + strings.Repeat("a", 64) + `"}`, nil
	}
	manager := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	calls := 0
	r := NewRunner(manager, tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		if calls == 1 {
			return observationCall("task_save_artifact"), nil
		}
		return runAnswer(api.ChatMessage{Content: "Finished researching the requested sources."}), nil
	})
	cfg := observationConfig()
	cfg.NativeWorkflowVersion = DocumentWorkflowEvidenceVersion
	task, err := r.Create("Inspect the requested application and research public sources", "model", "alice", cfg)
	if err != nil {
		t.Fatal(err)
	}
	// Reproduce a resolved model-requested access call, not a preattached grant.
	_, err = manager.updateTask(task.ID, func(task *Task) error {
		task.LastAccess = &InputRequest{Kind: "computer", CallID: "access-call", Mode: "desktop", Target: "requested application"}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.Continue(context.Background(), task.ID, "alice", "start", "", false)
	if err != nil || got.Status != TaskInterrupted || got.ErrorCode != "workflow_completion_repair_limit" || tools.calls.Load() != 1 || calls != 5 {
		t.Fatalf("artifact substituted for application work: status=%s code=%s calls=%d tools=%d err=%v", got.Status, got.ErrorCode, calls, tools.calls.Load(), err)
	}
	if got.Workflow == nil || !strings.Contains(got.Steps[len(got.Steps)-1].Content, "access") {
		t.Fatal("lost outstanding access outcome")
	}
}

func TestDesktopObservationRepairIsBoundedRedactedAndDurable(t *testing.T) {
	for _, next := range []string{"repeat", "false-completion"} {
		t.Run(next, func(t *testing.T) {
			dir := t.TempDir()
			tools := &observationTestTools{}
			calls := 0
			r := NewRunner(NewManagerWithPersistence(nil, nil, nil, dir), tools, func(_ context.Context, task *Task, messages []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
				calls++
				if calls > 1 && (len(task.Checkpoint.Calls) != 0 || task.Checkpoint.ExecutingCall != "") {
					t.Fatal("rejected proposal queued")
				}
				if calls > 1 && next == "false-completion" {
					return runAnswer(api.ChatMessage{Content: "Finished the edit."}), nil
				}
				return observationCall("computer_replace_text"), nil
			})
			task, err := r.Create("Edit the selected field", "model", "alice", observationConfig())
			if err != nil {
				t.Fatal(err)
			}
			got, err := r.Continue(context.Background(), task.ID, "alice", "start", "", false)
			if err != nil || got.Status != TaskInterrupted || got.ErrorCode != proposalRepairLimitCode || calls != 4 || tools.calls.Load() != 0 {
				t.Fatalf("unsafe/unbounded recovery: %+v calls=%d err=%v", got, calls, err)
			}
			if got.Checkpoint.Iteration != 4 || len(got.Checkpoint.Calls) != 0 || got.PendingApproval != nil {
				t.Fatal("bad checkpoint", got.Checkpoint)
			}
			first := got.Steps[0]
			if first.Type != "rejected" || first.ToolName != "computer_replace_text" || first.ToolArgs != "" || !strings.Contains(first.ToolResult, "computer_observation_required") {
				t.Fatal("missing sanitized rejection", first)
			}
			encoded, _ := json.Marshal(got.Checkpoint)
			if strings.Contains(string(encoded), "private-rejected-payload") {
				t.Fatal("rejected private payload retained")
			}
			if err := r.Shutdown(context.Background()); err != nil {
				t.Fatal(err)
			}
			saved, ok := NewManagerWithPersistence(nil, nil, nil, dir).GetTask(task.ID)
			if !ok || saved.Status != TaskInterrupted || saved.Checkpoint.ProposalRepairs != 4 || len(saved.Checkpoint.Calls) != 0 || saved.Steps[0].ToolName != first.ToolName {
				t.Fatal("rejection lost across restart", saved)
			}
		})
	}
}

func TestDesktopObservationRepairRequiresRealProgress(t *testing.T) {
	tools := &observationTestTools{}
	tools.authorize = func(string, json.RawMessage, ToolExecution) error { return nil }
	var executed []string
	tools.execute = func(name string, _ json.RawMessage, _ ToolExecution) (string, error) {
		executed = append(executed, name)
		return `{"observation":{"id":"fresh"}}`, nil
	}
	calls := 0
	r := NewRunner(NewManagerWithPersistence(nil, nil, nil, t.TempDir()), tools, func(_ context.Context, task *Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		switch calls {
		case 1:
			return observationCall("computer_replace_text"), nil
		case 2:
			return observationCall("computer_observe"), nil
		case 3:
			if !task.Checkpoint.ProposalUnresolved || task.Checkpoint.ProposalRepairs != 1 {
				t.Fatal("observation erased unfinished edit")
			}
			return observationCall("computer_replace_text"), nil
		default:
			return runAnswer(api.ChatMessage{Content: "Finished."}), nil
		}
	})
	task, err := r.Create("Edit the field", "model", "alice", observationConfig())
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.Continue(context.Background(), task.ID, "alice", "start", "", false)
	if err != nil || got.Status != TaskCompleted || strings.Join(executed, ",") != "computer_observe,computer_replace_text" || got.Checkpoint.ProposalUnresolved {
		t.Fatalf("repair failed: %+v executed=%v err=%v", got, executed, err)
	}
}

func TestDesktopObservationRepairDoesNotRetryUncertainEffects(t *testing.T) {
	tools := &observationTestTools{}
	tools.authorize = func(string, json.RawMessage, ToolExecution) error { return nil }
	tools.execute = func(name string, _ json.RawMessage, _ ToolExecution) (string, error) {
		if name == "computer_observe" {
			return `{"observation":{"id":"fresh"}}`, nil
		}
		return "", errors.New("input outcome unknown")
	}
	calls := 0
	r := NewRunner(NewManagerWithPersistence(nil, nil, nil, t.TempDir()), tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		if calls == 2 {
			return observationCall("computer_observe"), nil
		}
		return observationCall("computer_replace_text"), nil
	})
	task, err := r.Create("Edit the field", "model", "alice", observationConfig())
	if err != nil {
		t.Fatal(err)
	}
	got, _ := r.Continue(context.Background(), task.ID, "alice", "start", "", false)
	if got.Status != TaskUncertain || calls != 3 || tools.calls.Load() != 2 {
		t.Fatalf("uncertain input retried: %+v model=%d tools=%d", got, calls, tools.calls.Load())
	}
}

func TestDesktopObservationArtifactExemptionIsIdentityBound(t *testing.T) {
	valid := capabilities.Descriptor{Name: "task_save_artifact", Kind: capabilities.Write, Namespace: "workspace", Source: "workspace", Risk: capabilities.RiskLow}
	for _, field := range []string{"name", "kind", "namespace", "source", "legacy"} {
		t.Run(field, func(t *testing.T) {
			d := valid
			cfg := observationConfig()
			switch field {
			case "name":
				d.Name = "other"
			case "kind":
				d.Kind = capabilities.Computer
			case "namespace":
				d.Namespace = "computer"
			case "source":
				d.Source = "local-companion"
			case "legacy":
				cfg.TaskFirst = false
			}
			tools := &observationTestTools{descriptor: d}
			r := NewRunner(NewManagerWithPersistence(nil, nil, nil, t.TempDir()), tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
				return observationCall("task_save_artifact"), nil
			})
			task, err := r.Create("fixture", "model", "alice", cfg)
			if err != nil {
				t.Fatal(err)
			}
			got, err := r.Continue(context.Background(), task.ID, "alice", "start", "", false)
			if err != nil || got.Status != TaskInterrupted || tools.calls.Load() != 0 {
				t.Fatalf("identity/legacy boundary bypassed: %s %v", got.Status, err)
			}
		})
	}
}

func TestDesktopObservationRepairPersistenceFailureStopsInference(t *testing.T) {
	manager := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	blocked := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	tools := &observationTestTools{}
	calls := 0
	r := NewRunner(manager, tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		manager.mu.Lock()
		manager.dataDir = blocked
		manager.mu.Unlock()
		return observationCall("computer_replace_text"), nil
	})
	task, err := r.Create("fixture", "model", "alice", observationConfig())
	if err != nil {
		t.Fatal(err)
	}
	_, err = r.Continue(context.Background(), task.ID, "alice", "start", "", false)
	if !errors.Is(err, ErrRunStorage) || calls != 1 || tools.calls.Load() != 0 {
		t.Fatalf("failed checkpoint retried: %v model=%d tools=%d", err, calls, tools.calls.Load())
	}
}

func TestDesktopObservationArtifactCannotClearNativeObligations(t *testing.T) {
	for _, obligation := range []string{"unverified-artifact", "native-check", "unfinished-plan", "prior-native-action"} {
		t.Run(obligation, func(t *testing.T) {
			tools := &observationTestTools{descriptor: capabilities.Descriptor{Name: "task_save_artifact", Kind: capabilities.Write, Namespace: "workspace", Source: "workspace"}}
			r := NewRunner(NewManagerWithPersistence(nil, nil, nil, t.TempDir()), tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
				return runAnswer(api.ChatMessage{Content: "All done."}), nil
			})
			cfg := observationConfig()
			cfg.NativeWorkflowVersion = DocumentWorkflowEvidenceVersion
			if obligation == "native-check" {
				cfg.NativeExpectations = []OutcomeExpectation{{ID: "edit", Title: "Edit document", Kind: "application_open", ApplicationID: "fixture"}}
			}
			task, err := r.Create("fixture", "model", "alice", cfg)
			if err != nil {
				t.Fatal(err)
			}
			_, err = r.manager.updateTask(task.ID, func(saved *Task) error {
				result := `{"verified":true,"check":"stored_bytes_sha256_and_format","sha256":"` + strings.Repeat("a", 64) + `"}`
				if obligation == "unverified-artifact" {
					result = `{"verified":false}`
				}
				saved.Steps = []Step{{ID: 1, Type: "action", ToolName: "task_save_artifact", ToolResult: result}}
				if obligation == "unfinished-plan" {
					saved.Plan = []PlanItem{{ID: "write", Title: "Write in application", State: "pending"}}
				}
				if obligation == "prior-native-action" {
					saved.Steps = append(saved.Steps, Step{ID: 2, Type: "action", ToolName: "computer_observe", ToolResult: `{}`})
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			got, err := r.Continue(context.Background(), task.ID, "alice", "start", "", false)
			if err != nil || got.Status != TaskInterrupted || got.ErrorCode != "workflow_completion_repair_limit" || tools.calls.Load() != 0 {
				t.Fatalf("artifact manufactured completion: %s %s %v", got.Status, got.ErrorCode, err)
			}
		})
	}
}

func TestDesktopObservationAndInvalidDecisionShareRepairBudget(t *testing.T) {
	tools := &observationTestTools{}
	calls := 0
	r := NewRunner(NewManagerWithPersistence(nil, nil, nil, t.TempDir()), tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		if calls%2 == 1 {
			return nil, rejectedDecision()
		}
		return observationCall("computer_replace_text"), nil
	})
	task, err := r.Create("fixture", "model", "alice", observationConfig())
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.Continue(context.Background(), task.ID, "alice", "start", "", false)
	if err != nil || got.Status != TaskInterrupted || calls != 3 || tools.calls.Load() != 0 || got.Checkpoint.DecisionRepairs != 2 || got.Checkpoint.ProposalRepairs != 1 {
		t.Fatalf("alternating failures reset repair budget: status=%s calls=%d err=%v", got.Status, calls, err)
	}
}
