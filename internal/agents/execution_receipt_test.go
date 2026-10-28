package agents

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func executionReceiptFixture(t *testing.T, dir string) (*Manager, *Task, execution.Request) {
	t.Helper()
	m := NewManagerWithPersistence(nil, nil, nil, dir)
	r := NewRunner(m, &runTestTools{}, nil)
	config := DefaultAgentConfig()
	ref := execution.Reference{Host: "host", Session: "session", Grant: "grant", Revision: 1}
	config.Execution = &ref
	config.ExecutionWorkspace = &execution.Resource{Reference: ref, Backend: "local", Root: "/fixture", Directory: ".", Permissions: execution.Permissions{Read: true, Write: true}}
	task, err := r.Create("write exact text", "model", "alice", config)
	if err != nil {
		t.Fatal(err)
	}
	modelArgs := `{"path":"exact.txt","content":"café 日本語"}`
	d, _ := execution.Descriptor("workspace_write")
	task, err = m.updateTask(task.ID, func(t *Task) error {
		t.Status = TaskRunning
		t.Checkpoint = &Checkpoint{ExecutingCall: "call", ExecutingCapability: &d, Calls: []api.ToolCall{{ID: "call", Type: "function", Function: api.FunctionCall{Name: "workspace_write", Arguments: modelArgs}}}}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	args, _ := json.Marshal(map[string]any{"path": "exact.txt", "data": []byte("café 日本語")})
	request, err := execution.NewIntent(execution.Binding{Actor: task.Actor, Workspace: "workspace", Host: ref.Host, Session: ref.Session, Grant: ref.Grant, Revision: ref.Revision, Job: task.ID}, "call", "file_write", args, false)
	if err != nil {
		t.Fatal(err)
	}
	if err = m.RecordExecutionIntent(task.ID, task.Actor, "workspace_write", []byte(modelArgs), request); err != nil {
		t.Fatal(err)
	}
	return m, task, request
}

func TestExecutionReceiptSurvivesRestartWithoutReplayOrGrantRestoration(t *testing.T) {
	dir := t.TempDir()
	m, task, request := executionReceiptFixture(t, dir)
	response := execution.Response{Version: 1, ID: request.ID, Receipt: &execution.Receipt{State: "complete", Result: json.RawMessage(`{"verified":true,"revision":"saved-digest"}`)}}
	for i := 0; i < 2; i++ {
		if err := m.RecordExecutionReceipt(task.ID, task.Actor, request, response); err != nil {
			t.Fatal(err)
		}
	}
	restored := NewManagerWithPersistence(nil, nil, nil, dir)
	got, ok := restored.GetTask(task.ID)
	if !ok || got.Status != TaskInterrupted || got.ErrorCode != "execution_result_recovered" || got.Checkpoint.ExecutingCall != "" || got.Checkpoint.ExecutionIntent != nil || len(got.Steps) != 1 || got.Steps[0].Provenance != "execution_receipt" || got.Result != "" {
		t.Fatal(got)
	}
	if recoverExecutionReceipt(got) || len(got.Steps) != 1 {
		t.Fatal("evidence applied twice")
	}
	if !strings.Contains(got.Steps[0].ToolResult, `"historical":true`) || !strings.Contains(got.Steps[0].ToolResult, "Re-read outstanding outcomes") {
		t.Fatal("historical receipt presented as current observation")
	}
	if got.Config.Execution == nil {
		t.Fatal("lost inspection metadata")
	}
}

func TestHumanReconciliationClearsPendingExecutionIntentWithoutVerification(t *testing.T) {
	m, task, _ := executionReceiptFixture(t, t.TempDir())
	_, err := m.updateTask(task.ID, func(t *Task) error { t.Status = TaskUncertain; return nil })
	if err != nil {
		t.Fatal(err)
	}
	r := NewRunner(m, &runTestTools{}, nil)
	got, err := r.Reconcile(task.ID, task.Actor, "call", "human inspected the outcome")
	if err != nil || got.Status != TaskInterrupted || got.Checkpoint.ExecutionIntent != nil || got.Checkpoint.ExecutionReceipt != nil || got.Steps[0].Provenance != "human_reported" {
		t.Fatal(got, err)
	}
}

func TestExecutionReceiptSurvivesStaleWorkerSnapshotAndMissingEvidenceStaysUnknown(t *testing.T) {
	for _, hasReceipt := range []bool{false, true} {
		dir := t.TempDir()
		m, stale, request := executionReceiptFixture(t, dir)
		if hasReceipt {
			if err := m.RecordExecutionReceipt(stale.ID, stale.Actor, request, execution.Response{Version: 1, ID: request.ID, Receipt: &execution.Receipt{State: "complete", Result: json.RawMessage(`{"verified":true}`)}}); err != nil {
				t.Fatal(err)
			}
		}
		// The worker's older in-memory checkpoint must not overwrite a receipt
		// delivered concurrently by the authenticated native reply handler.
		r := NewRunner(m, &runTestTools{}, nil)
		if err := r.persist(stale); err != nil {
			t.Fatal(err)
		}
		restored := NewManagerWithPersistence(nil, nil, nil, dir)
		got, _ := restored.GetTask(stale.ID)
		if hasReceipt {
			if got.Status != TaskInterrupted || len(got.Steps) != 1 {
				t.Fatal(got)
			}
		} else if got.Status != TaskUncertain || !CanRecoverExecution(got) || len(got.Steps) != 0 {
			t.Fatal("missing evidence inferred success", got)
		}
	}
}

func TestExecutionReceiptsRejectIdentityAndPayloadMismatch(t *testing.T) {
	for _, change := range []string{"actor", "job", "grant", "revision", "operation", "arguments", "approval", "token", "missing-intent", "descriptor"} {
		t.Run(change, func(t *testing.T) {
			m, task, request := executionReceiptFixture(t, t.TempDir())
			switch change {
			case "actor":
				request.Binding.Actor = "bob"
			case "job":
				request.Binding.Job = "other-job"
			case "grant":
				request.Binding.Grant = "other-grant"
			case "revision":
				request.Binding.Revision++
			case "operation":
				request.Operation = "process_start"
			case "arguments":
				request.Arguments = json.RawMessage(`{"path":"other.txt","data":"eA=="}`)
			case "approval":
				request.Approved = true
			case "token":
				request.Token = "never persist credentials"
			case "missing-intent":
				_, _ = m.updateTask(task.ID, func(t *Task) error { t.Checkpoint.ExecutionIntent = nil; return nil })
			case "descriptor":
				_, _ = m.updateTask(task.ID, func(t *Task) error { t.Checkpoint.ExecutingCapability.Revision = "unreviewed"; return nil })
			}
			response := execution.Response{Version: 1, ID: "call", Receipt: &execution.Receipt{State: "complete", Result: json.RawMessage(`{"verified":true}`)}}
			if m.RecordExecutionReceipt(task.ID, task.Actor, request, response) == nil {
				t.Fatal("accepted changed evidence")
			}
		})
	}
}

type executionReceiptReaderFixture struct {
	*runTestTools
	request execution.Request
	reads   int
}

func (f *executionReceiptReaderFixture) ReadExecutionReceipt(_ context.Context, t *Task) (execution.Response, error) {
	f.reads++
	return execution.Response{Version: 1, ID: f.request.ID, Receipt: &execution.Receipt{State: "complete", Result: json.RawMessage(`{"verified":true}`)}}, nil
}

func TestExplicitReceiptRecoveryOnlyReadsOriginalIntent(t *testing.T) {
	m, task, request := executionReceiptFixture(t, t.TempDir())
	_, err := m.updateTask(task.ID, func(t *Task) error { t.Status = TaskUncertain; return nil })
	if err != nil {
		t.Fatal(err)
	}
	tools := &executionReceiptReaderFixture{runTestTools: &runTestTools{}, request: request}
	r := NewRunner(m, tools, nil)
	if _, err = r.RecoverExecution(context.Background(), task.ID, "bob"); err != ErrTaskNotFound {
		t.Fatal(err)
	}
	got, err := r.RecoverExecution(context.Background(), task.ID, task.Actor)
	if err != nil || got.Status != TaskInterrupted || tools.reads != 1 || tools.calls.Load() != 0 || got.Result != "" {
		t.Fatal(got, err, tools.reads)
	}
	if _, err = r.RecoverExecution(context.Background(), task.ID, task.Actor); err == nil || tools.reads != 1 {
		t.Fatal("duplicate recovery queried/dispatched again")
	}
}
