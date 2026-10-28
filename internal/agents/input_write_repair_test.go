package agents

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func inputWriteRepairFixture(t *testing.T) *Task {
	t.Helper()
	ref := execution.Reference{Host: "host", Session: "session", Grant: "grant", Revision: 1}
	d, _ := execution.Descriptor(execution.InputWriteTool)
	args, _ := CanonicalArguments([]byte(`{"path":"result.txt","input":"request","after":"<a>","before":"</a>","revision":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`))
	task := &Task{ID: "run-fixture", Actor: "alice", Status: TaskRunning, Prompt: "Write <a>café 日本語\n</a>", Config: AgentConfig{TaskFirst: true, ExecutionRequestVersion: 1, InputWriteVersion: 1, Execution: &ref, ExecutionWorkspace: &execution.Resource{Reference: ref, Root: "/fixture", Backend: "local"}}, Checkpoint: &Checkpoint{PreEffectRepair: true, PreEffectRepairTool: "workspace_write", ProposalTool: "workspace_write", ProposalUnresolved: true, ProposalRepairs: 1, PreEffectRepairs: 1, ExecutingCall: "call-source", ExecutingCapability: &d, Calls: []api.ToolCall{{ID: "call-source", Type: "function", Function: api.FunctionCall{Name: execution.InputWriteTool, Arguments: string(args)}}}}}
	original, _ := json.Marshal(map[string]string{"path": "result.txt", "content": "café 日本語\n", "revision": ""})
	task.Steps = []Step{{Type: "rejected", ToolName: "workspace_write", ToolArgs: string(original), ToolResult: `{"effects":"none","error":"execution_write_precondition"}`}}
	payload, proof, err := ResolveInputWrite(task, args)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(payload)
	request, err := execution.NewIntent(execution.Binding{Actor: task.Actor, Workspace: "workspace", Job: task.ID, Host: ref.Host, Session: ref.Session, Grant: ref.Grant, Revision: ref.Revision}, "call-source", "file_write", data, false)
	if err != nil {
		t.Fatal(err)
	}
	written, _ := json.Marshal(execution.WriteResult{Location: execution.Location{Host: "host", Path: "/fixture/result.txt"}, Revision: proof.ContentDigest, Bytes: len(payload.Data), Verified: true})
	task.Checkpoint.ExecutionIntent = &ExecutionIntent{Tool: execution.InputWriteTool, Arguments: string(args), Request: request}
	task.Checkpoint.ExecutionReceipt = &ExecutionReceipt{Receipt: execution.Receipt{State: "complete", Result: written}, At: time.Now().UTC()}
	return task
}

func TestInputWriteRepairsOnlySameVerifiedNoEffectWrite(t *testing.T) {
	for _, kind := range []string{"matching", "other-path", "other-content", "uncertain", "partial", "wrong-receipt", "missing", "foreign-provider", "different-call", "different-grant", "changed-directory", "changed-attachment", "missing-rejection", "reverse"} {
		t.Run(kind, func(t *testing.T) {
			task := inputWriteRepairFixture(t)
			saved, err := copyTask(task)
			if err != nil {
				t.Fatal(err)
			}
			d, _ := execution.Descriptor(execution.InputWriteTool)
			switch kind {
			case "other-path":
				task.Steps[0].ToolArgs = strings.ReplaceAll(task.Steps[0].ToolArgs, "result.txt", "other.txt")
			case "other-content":
				task.Steps[0].ToolArgs = strings.ReplaceAll(task.Steps[0].ToolArgs, "café", "wrong")
			case "uncertain", "partial":
				saved.Checkpoint.ExecutionReceipt.Receipt.State = kind
			case "wrong-receipt":
				saved.Checkpoint.ExecutionReceipt.Receipt.Result = json.RawMessage(`{"verified":true,"bytes":1}`)
			case "missing":
				saved.Checkpoint.ExecutionReceipt = nil
			case "foreign-provider":
				d.Source = "mcp:foreign"
			case "different-call":
				saved.Checkpoint.ExecutingCall = "other-call"
			case "different-grant":
				saved.Config.Execution.Grant = "foreign"
			case "changed-directory":
				task.Steps = append(task.Steps, Step{Type: "action", ToolName: "workspace_change_directory"})
			case "changed-attachment":
				task.Steps = append(task.Steps, Step{Type: "input_resolved", ToolName: execution.AccessRequestTool})
			case "missing-rejection":
				task.Steps = nil
			case "reverse":
				task.Checkpoint.PreEffectRepairTool = execution.InputWriteTool
				d, _ = execution.Descriptor("workspace_write")
			}
			resolveInputWriteRepair(task, saved, d)
			if got := !task.Checkpoint.PreEffectRepair; got != (kind == "matching") {
				t.Fatal("incorrect repair settlement", kind, got)
			}
			if kind == "matching" && (task.Checkpoint.ProposalUnresolved || task.Checkpoint.ProposalRepairs != 0 || task.Checkpoint.PreEffectRepairs != 0) {
				t.Fatal("repair counters remained unresolved")
			}
		})
	}
}

func TestRecoveredInputWriteSettlesRepairWithoutReplay(t *testing.T) {
	task := inputWriteRepairFixture(t)
	task.Status = TaskUncertain
	if !recoverExecutionReceipt(task) || task.Status != TaskInterrupted || task.Checkpoint.PreEffectRepair || task.Checkpoint.ProposalUnresolved || len(task.Steps) != 2 {
		t.Fatal("saved verified equivalent write was not recovered", task.Status, task.Checkpoint)
	}
	if recoverExecutionReceipt(task) || len(task.Steps) != 2 {
		t.Fatal("receipt incorporated twice")
	}
}
