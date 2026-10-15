package agents

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestConversationWithDesktopAuthorityNeedsNoOutcomeDeclaration(t *testing.T) {
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	tools := &runTestTools{}
	modelCalls := 0
	r := NewRunner(m, tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		modelCalls++
		return runAnswer(api.ChatMessage{Role: "assistant", Content: "Hello!"}), nil
	})
	cfg := workflowConfig()
	task, err := r.Create("hello", "model", "alice", cfg)
	if err != nil {
		t.Fatal(err)
	}
	done, err := r.Continue(context.Background(), task.ID, task.Actor, "start", "", false)
	if err != nil || done.Status != TaskCompleted || done.Result != "Hello!" || modelCalls != 1 || tools.calls.Load() != 0 {
		t.Fatalf("conversation required action: status=%s model_calls=%d tools=%d err=%v", done.Status, modelCalls, tools.calls.Load(), err)
	}
	if done.PendingInput != nil || done.PendingApproval != nil || (done.Workflow != nil && len(done.Workflow.Checks) != 0) {
		t.Fatal("greeting fabricated access requests or outcome checks")
	}
}

// Sanitized OBS regression: a verified launch must permit an ordinary final
// sentence without a calculator call used as a makeshift answer channel.
func TestVerifiedOBSCanFinishWithoutUnrelatedTool(t *testing.T) {
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	tools := &assessedTestTools{states: map[string]string{"open-obs": "verified"}}
	calls := 0
	r := NewRunner(m, tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		return runAnswer(api.ChatMessage{Role: "assistant", Content: "OBS is open."}), nil
	})
	cfg := workflowConfig()
	cfg.NativeExpectations = []OutcomeExpectation{{ID: "open-obs", Title: "Open OBS", Kind: "application_open", ApplicationID: "host-obs"}}
	task, err := r.Create("Open OBS", "model", "alice", cfg)
	if err != nil {
		t.Fatal(err)
	}
	done, err := r.Continue(context.Background(), task.ID, "alice", "start", "", false)
	if err != nil || done.Status != TaskCompleted || done.Result != "OBS is open." || done.Workflow.State != "checks_passed" || calls != 1 || tools.calls.Load() != 0 {
		t.Fatal("verified outcome required an unrelated tool", done, err)
	}
}

func TestRejectedWriteHasTrustedNoEffects(t *testing.T) {
	_, err := executeWriteFile("", "not written")
	var failure *ToolFailure
	if !errors.As(err, &failure) {
		t.Fatalf("pre-write rejection lacks effects classification: %v", err)
	}
}

func TestMalformedWriteCannotTruncateExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "existing.txt")
	if err := os.WriteFile(path, []byte("Preserve this"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, content := range []any{nil, 42, []string{"text"}} {
		args, _ := json.Marshal(map[string]any{"path": path, "content": content})
		_, err := ExecuteTool(context.Background(), "write_file", args)
		var failure *ToolFailure
		if !errors.As(err, &failure) || !failure.BeforeEffects {
			t.Fatalf("missing no-effects rejection: %v", err)
		}
		data, err := os.ReadFile(path)
		if err != nil || string(data) != "Preserve this" {
			t.Fatal("malformed request changed file", err)
		}
	}
}

func TestRestrictedServiceWriteIsRejectedBeforeFilesystemEntry(t *testing.T) {
	for _, root := range getRestrictedWritePaths() {
		_, err := executeWriteFileWith(filepath.Join(root, "offgrid-test-never-created"), "not written", func(string, os.FileMode) error { t.Fatal("entered filesystem on a protected path"); return nil }, func(string, []byte, os.FileMode) error { t.Fatal("entered filesystem on a protected path"); return nil })
		var failure *ToolFailure
		if !errors.As(err, &failure) || !failure.BeforeEffects {
			t.Fatalf("restricted path was not pre-effect rejection: %v", err)
		}
	}
}

type partialWriteTools struct {
	*preEffectTestTools
	directory      string
	writes         int
	afterDirectory bool
}

func (p *partialWriteTools) ExecuteWithPolicy(ctx context.Context, name string, args json.RawMessage, execution ToolExecution) (string, error) {
	if name != "write_file" {
		return p.preEffectTestTools.ExecuteWithPolicy(ctx, name, args, execution)
	}
	p.writes++
	return executeWriteFileWith(filepath.Join(p.directory, "new", "result.txt"), "complete content", func(path string, mode os.FileMode) error {
		if err := os.MkdirAll(path, mode); err != nil {
			return err
		}
		if p.afterDirectory {
			return errors.New("injected failure after directory creation")
		}
		return nil
	}, func(path string, data []byte, mode os.FileMode) error {
		if err := os.WriteFile(path, data[:3], mode); err != nil {
			return err
		}
		return errors.New("injected failure after partial file write")
	})
}

func TestPartialFilesystemEffectsAreNotRetried(t *testing.T) {
	if runtime.GOOS == "darwin" {
		// macOS's default /var/folders temp location is intentionally protected.
		// Exercise post-effect failures in an allowed, disposable location instead.
		t.Setenv("TMPDIR", "/tmp")
	}
	for _, afterDirectory := range []bool{true, false} {
		m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
		tools := &partialWriteTools{preEffectTestTools: &preEffectTestTools{NewToolRegistry()}, directory: t.TempDir(), afterDirectory: afterDirectory}
		r := NewRunner(m, tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
			return taskCall("write_file", `{"path":"isolated-fixture","content":"complete content"}`), nil
		})
		task, err := r.Create("write test file", "model", "alice", DefaultAgentConfig())
		if err != nil {
			t.Fatal(err)
		}
		task, err = r.Continue(context.Background(), task.ID, "alice", "start", "", false)
		if err == nil || task.Status != TaskUncertain || tools.writes != 1 {
			t.Fatal("partial effects not retained", task.Status, tools.writes, err)
		}
		if _, err = r.Continue(context.Background(), task.ID, "alice", "resume", "", false); !errors.Is(err, ErrRunConflict) || tools.writes != 1 {
			t.Fatal("partial write replayed", err)
		}
		original := task.Checkpoint.Calls[0]
		task, err = r.Reconcile(task.ID, "alice", original.ID, "done")
		if err != nil || task.Status != TaskInterrupted || tools.writes != 1 {
			t.Fatal("human report replayed write", err)
		}
		step := task.Steps[len(task.Steps)-1]
		if step.Provenance != "human_reported" || step.ToolArgs != original.Function.Arguments || step.ToolResult != "done" {
			t.Fatal("human report lost provenance", step)
		}
		var report struct {
			Verified   bool   `json:"verified"`
			Provenance string `json:"provenance"`
		}
		if json.Unmarshal([]byte(task.Checkpoint.Messages[len(task.Checkpoint.Messages)-1].StringContent()), &report) != nil || report.Verified || report.Provenance != "human_reported" {
			t.Fatal("human report promoted to evidence")
		}
	}
}

func TestWriteFailureAfterFilesystemEntryRemainsUnknown(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Setenv("TMPDIR", "/tmp")
	}
	// A directory at the destination deterministically fails WriteFile after
	// the operation has entered the filesystem phase, on every supported OS.
	destination := filepath.Join(t.TempDir(), "directory")
	if err := os.Mkdir(destination, 0700); err != nil {
		t.Fatal(err)
	}
	_, err := executeWriteFile(destination, "text")
	var failure *ToolFailure
	if err == nil || errors.As(err, &failure) {
		t.Fatalf("filesystem-phase failure incorrectly classified: %v", err)
	}
}

func TestConversationCannotClearDeclaredPendingOutcome(t *testing.T) {
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	r := NewRunner(m, &runTestTools{}, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		return runAnswer(api.ChatMessage{Role: "assistant", Content: "Hello!"}), nil
	})
	cfg := workflowConfig()
	cfg.NativeExpectations = []OutcomeExpectation{{ID: "open", Title: "Open editor", Kind: "application_open", ApplicationID: "editor"}}
	task, err := r.Create("open editor", "model", "alice", cfg)
	if err != nil {
		t.Fatal(err)
	}
	done, err := r.Continue(context.Background(), task.ID, task.Actor, "start", "", false)
	if err != nil || done.Status != TaskInterrupted || done.ErrorCode != "workflow_completion_repair_limit" {
		t.Fatalf("pending outcome bypassed: %+v %v", done, err)
	}
}

func TestOnDemandDesktopPreferenceDoesNotRequestAccessForConversation(t *testing.T) {
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	tools := &runTestTools{}
	r := NewRunner(m, tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		return runAnswer(api.ChatMessage{Role: "assistant", Content: "Hello!"}), nil
	})
	cfg := DefaultAgentConfig()
	cfg.TaskFirst, cfg.ComputerAccessOnDemand = true, true
	cfg.RequestedComputerScope, cfg.RequestedComputerApprovalMode = "desktop", "full_task"
	task, err := r.Create("hello", "model", "alice", cfg)
	if err != nil || task.PendingInput != nil || task.Status != TaskPending {
		t.Fatalf("preference requested access: %+v %v", task, err)
	}
	task, err = r.Continue(context.Background(), task.ID, task.Actor, "start", "", false)
	if err != nil || task.Status != TaskCompleted || task.PendingInput != nil || tools.calls.Load() != 0 {
		t.Fatalf("conversation dispatched work: %+v %v", task, err)
	}
}

type preEffectTestTools struct{ *ToolRegistry }

func (p *preEffectTestTools) Authorize(context.Context, string, json.RawMessage, ToolExecution) error {
	return nil
}
func (p *preEffectTestTools) ExecuteWithPolicy(ctx context.Context, name string, args json.RawMessage, _ ToolExecution) (string, error) {
	return ExecuteTool(ctx, name, args)
}

func TestPreEffectRepairBudgetSurvivesUnrelatedReads(t *testing.T) {
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	tools := &preEffectTestTools{NewToolRegistry()}
	modelCalls := 0
	r := NewRunner(m, tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		modelCalls++
		if modelCalls%2 == 0 {
			return taskCall("calculator", `{"expression":"1+1"}`), nil
		}
		return taskCall("write_file", `{"path":"","content":"text"}`), nil
	})
	cfg := DefaultAgentConfig()
	cfg.MaxIterations = 20
	task, err := r.Create("write a service file", "model", "alice", cfg)
	if err != nil {
		t.Fatal(err)
	}
	task, err = r.Continue(context.Background(), task.ID, task.Actor, "start", "", false)
	if err != nil || task.Status != TaskInterrupted || task.ErrorCode != proposalRepairLimitCode || modelCalls != 7 || task.Checkpoint.ExecutingCall != "" {
		t.Fatalf("repair loop escaped budget: status=%s code=%s calls=%d error=%v", task.Status, task.ErrorCode, modelCalls, err)
	}
}

func TestPreEffectRepairRequiresRelevantTrustedProgress(t *testing.T) {
	native := capabilities.Descriptor{Kind: capabilities.Computer, Namespace: "computer", Source: "local-companion"}
	for _, test := range []struct {
		tool       string
		descriptor capabilities.Descriptor
		repaired   bool
	}{
		{"computer_observe", native, false},
		{"computer_create_document", native, false}, // Blank document is not the rejected text edit.
		{"computer_insert_text", native, true},
		{"computer_insert_text", capabilities.Descriptor{Source: "mcp:untrusted"}, false},
		{"write_file", capabilities.Descriptor{Source: "builtin", Namespace: "tools"}, true},
	} {
		cp := &Checkpoint{PreEffectRepair: true, PreEffectRepairTool: "write_file", ProposalTool: "write_file", ProposalUnresolved: true, ProposalRepairs: 2}
		resolvePreEffectRepair(cp, test.tool, test.descriptor)
		if cp.PreEffectRepair == test.repaired || cp.ProposalUnresolved == test.repaired {
			t.Fatalf("%s: %+v", test.tool, cp)
		}
	}
	cp := &Checkpoint{PreEffectRepair: true, PreEffectRepairTool: "write_file", ProposalTool: "computer_insert_text", WorkflowEditPending: true, ProposalUnresolved: true, ProposalRepairs: 2}
	resolvePreEffectRepair(cp, "computer_insert_text", native)
	if cp.PreEffectRepair || !cp.WorkflowEditPending || !cp.ProposalUnresolved {
		t.Fatalf("stronger repair lost: %+v", cp)
	}
}

func TestPreEffectRepairBudgetSurvivesOtherRepairReset(t *testing.T) {
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	r := NewRunner(m, &runTestTools{}, nil)
	task, err := r.Create("edit host document", "model", "alice", DefaultAgentConfig())
	if err != nil {
		t.Fatal(err)
	}
	if err := m.StartTask(task.ID); err != nil {
		t.Fatal(err)
	}
	task.Status = TaskRunning
	task.Checkpoint.PreEffectRepair = true
	task.Checkpoint.PreEffectRepairTool = "write_file"
	for i := 0; i < 4; i++ {
		// A different native obligation may reset the general proposal counter.
		// It cannot give an unresolved before-effects write unlimited retries.
		task.Checkpoint.ProposalRepairs = 0
		task.Checkpoint.Calls = []api.ToolCall{{ID: "rejected", Function: api.FunctionCall{Name: "write_file", Arguments: `{"path":"","content":"text"}`}}}
		if err := r.rejectProposal(task, &ToolAuthorizationError{Code: "tool_write_rejected", Message: "Choose a permitted service path."}, "Correct the path."); err != nil {
			t.Fatal(err)
		}
		if i < 3 && task.Status == TaskInterrupted {
			t.Fatal("repair budget exhausted early")
		}
	}
	if task.Status != TaskInterrupted || task.Checkpoint.PreEffectRepairs != 4 || task.ErrorCode != proposalRepairLimitCode {
		t.Fatalf("independent repair budget lost: %+v", task)
	}
	if !strings.Contains(task.Error, "Choose a permitted service path.") {
		t.Fatal("actionable rejection lost", task.Error)
	}
}
