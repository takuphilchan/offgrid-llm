package agents

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestDocumentMutationPersistenceFailureDoesNotRepeatInput(t *testing.T) {
	for _, name := range []string{"computer_create_document", "computer_insert_text", "computer_replace_selection", "computer_replace_text"} {
		for _, phase := range []string{"intent", "result"} {
			t.Run(name+"/"+phase, func(t *testing.T) {
				dir := t.TempDir()
				m := NewManagerWithPersistence(nil, nil, nil, dir)
				tools := &workflowTestTools{}
				blocked := filepath.Join(t.TempDir(), "not-directory")
				if err := os.WriteFile(blocked, []byte("fixture"), 0600); err != nil {
					t.Fatal(err)
				}
				breakStorage := func() { m.mu.Lock(); m.dataDir = blocked; m.mu.Unlock() }
				tools.authorize = func(string, json.RawMessage, ToolExecution) error {
					if phase == "intent" {
						breakStorage()
					}
					return nil
				}
				tools.execute = func(tool string, _ json.RawMessage, _ ToolExecution) (string, error) {
					if tool != name {
						t.Fatal("unexpected dispatch", tool)
					}
					breakStorage()
					return `{"action_id":"one-host-intent","outcome":"dispatched"}`, nil
				}
				r := NewRunner(m, tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
					return taskCall(name, `{}`), nil
				})
				cfg := workflowConfig()
				cfg.NativeWorkflowVersion = DocumentWorkflowEvidenceVersion
				cfg.NativeExpectations = []OutcomeExpectation{{ID: "open", Title: "Editor", Kind: "application_open", ApplicationID: "app"}}
				task, err := r.Create("Write a new unsaved document", "model", "alice", cfg)
				if err != nil {
					t.Fatal(err)
				}
				if err = m.AddTaskStep(task.ID, Step{ID: 1, Type: "action", ToolName: "computer_observe", ToolResult: `{"observation":{"id":"view"}}`}); err != nil {
					t.Fatal(err)
				}
				if _, err = r.Continue(context.Background(), task.ID, "alice", "start", "", false); !errors.Is(err, ErrRunStorage) {
					t.Fatalf("persistence failure hidden: %v", err)
				}
				want := int32(0)
				if phase == "result" {
					want = 1
				}
				if tools.calls.Load() != want {
					t.Fatal("unexpected input count", tools.calls.Load())
				}
				restored := NewManagerWithPersistence(nil, nil, nil, dir)
				got, ok := restored.GetTask(task.ID)
				if !ok || restored.StorageError() != nil {
					t.Fatal("lost durable intent")
				}
				if phase == "result" && (got.Status != TaskUncertain || got.Checkpoint.ExecutingCall == "") {
					t.Fatal("lost uncertain mutation boundary", got.Status)
				}
				if phase == "intent" && got.Status == TaskUncertain {
					t.Fatal("fabricated mutation before intent")
				}
				if _, err := NewRunner(restored, tools, nil).Continue(context.Background(), task.ID, "alice", "resume", "", false); err == nil {
					t.Fatal("resumed without fresh authority/reconciliation")
				}
				if tools.calls.Load() != want {
					t.Fatal("restart repeated document input")
				}
			})
		}
	}
}

func TestDocumentRepairSchemasRemainTaskLocal(t *testing.T) {
	for _, name := range []string{"computer_create_document", "computer_insert_text", "computer_replace_selection", "computer_replace_text"} {
		t.Run(name, func(t *testing.T) {
			cfg := workflowConfig()
			cfg.NativeWorkflowVersion = DocumentWorkflowEvidenceVersion
			task := &Task{Config: cfg, Checkpoint: &Checkpoint{WorkflowEditPending: true}, Steps: []Step{{Type: "rejected", ToolName: name, ToolArgs: `{"document":"doc"}`, ToolResult: `{"error":"workflow_control_expectation_required"}`}}}
			found := false
			for _, tool := range runtimeTools(task) {
				if tool.Function.Name != "task_expectations" {
					continue
				}
				found = true
				item := tool.Function.Parameters["properties"].(map[string]interface{})["expectations"].(map[string]any)["items"].(map[string]any)
				p := item["properties"].(map[string]any)
				required := item["required"].([]string)
				want := 8
				if name == "computer_create_document" {
					want = 5
				}
				if len(p) != want || len(required) != want {
					t.Fatalf("unexpected repair fields: %v / %v", p, required)
				}
				for _, field := range required {
					if p[field] == nil {
						t.Fatal("missing", field)
					}
				}
				if p["binding"] != nil || p["element"] != nil || p["expected_sha256"] != nil {
					t.Fatal("model can invent canonical identity")
				}
			}
			if !found {
				t.Fatal("no declaration repair tool")
			}
			fresh := workflowTool().Function.Parameters["properties"].(map[string]interface{})["expectations"].(map[string]any)["items"].(map[string]any)
			if len(fresh["required"].([]string)) != 3 || fresh["properties"].(map[string]any)["document"] != nil {
				t.Fatal("mutated legacy schema")
			}
		})
	}
}

func TestDocumentEditRepairIsBoundedAcrossReadsAndRestart(t *testing.T) {
	for _, name := range []string{"computer_create_document", "computer_insert_text", "computer_replace_selection", "computer_replace_text"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			m := NewManagerWithPersistence(nil, nil, nil, dir)
			tools := &workflowTestTools{}
			tools.authorize = func(tool string, _ json.RawMessage, _ ToolExecution) error {
				if tool == name {
					return &ToolAuthorizationError{Code: "workflow_control_expectation_required", Message: "Declare the exact document check."}
				}
				return nil
			}
			tools.execute = func(tool string, _ json.RawMessage, _ ToolExecution) (string, error) {
				if tool != "computer_documents" {
					t.Fatal("unexpected input", tool)
				}
				return `{}`, nil
			}
			calls := 0
			r := NewRunner(m, tools, func(_ context.Context, task *Task, messages []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
				calls++
				if calls > 7 {
					t.Fatal("unbounded repair")
				}
				if calls > 1 && (!task.Checkpoint.WorkflowEditPending || task.Checkpoint.ProposalRepairs == 0) {
					t.Fatal("discovery reset the repair budget")
				}
				if calls%2 == 0 {
					if !strings.Contains(messages[len(messages)-1].StringContent(), "document") {
						t.Fatal("missing document repair guidance")
					}
					return taskCall("computer_documents", `{}`), nil
				}
				return taskCall(name, `{}`), nil
			})
			cfg := workflowConfig()
			cfg.NativeWorkflowVersion = DocumentWorkflowEvidenceVersion
			cfg.NativeExpectations = []OutcomeExpectation{{ID: "open", Title: "Open editor", Kind: "application_open", ApplicationID: "app"}}
			task, err := r.Create("Edit only a new document", "model", "alice", cfg)
			if err != nil {
				t.Fatal(err)
			}
			if err := m.AddTaskStep(task.ID, Step{ID: 1, Type: "action", ToolName: "computer_observe", ToolResult: `{"observation":{"id":"view"}}`}); err != nil {
				t.Fatal(err)
			}
			got, err := r.Continue(context.Background(), task.ID, "alice", "start", "", false)
			if err != nil || got.Status != TaskInterrupted || got.ErrorCode != proposalRepairLimitCode || calls != 7 || tools.calls.Load() != 3 {
				t.Fatalf("calls=%d executions=%d status=%s code=%s err=%v message=%s", calls, tools.calls.Load(), got.Status, got.ErrorCode, err, got.Error)
			}
			if err := r.Shutdown(context.Background()); err != nil {
				t.Fatal(err)
			}
			saved, ok := NewManagerWithPersistence(nil, nil, nil, dir).GetTask(task.ID)
			if !ok || saved.Workflow.Version != DocumentWorkflowEvidenceVersion || !saved.Checkpoint.WorkflowEditPending || saved.Checkpoint.ProposalRepairs != 4 || !saved.ComputerSessionExpired {
				t.Fatal("lost repair boundary at restart")
			}
		})
	}
}
