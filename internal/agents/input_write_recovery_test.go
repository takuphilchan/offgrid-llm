package agents

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestInputWriteRecoveryPinsOriginalBytesAndRejectsFalseReceipt(t *testing.T) {
	for _, tampered := range []bool{false, true} {
		t.Run(map[bool]string{false: "matching", true: "wrong-content"}[tampered], func(t *testing.T) {
			dir := t.TempDir()
			m := NewManagerWithPersistence(nil, nil, nil, dir)
			r := NewRunner(m, &runTestTools{}, nil)
			ref := execution.Reference{Host: "host", Session: "session", Grant: "grant", Revision: 1}
			cfg := DefaultAgentConfig()
			cfg.InputWriteVersion = 1
			cfg.Execution = &ref
			cfg.ExecutionWorkspace = &execution.Resource{Reference: ref, Backend: "local", Root: "/fixture", Directory: ".", Permissions: execution.Permissions{Read: true, Write: true}}
			task, err := r.Create("Use <a>café 日本語\r\n</a>", "model", "alice", cfg)
			if err != nil {
				t.Fatal(err)
			}
			args := `{"path":"result.txt","input":"request","after":"<a>","before":"</a>"}`
			d, _ := execution.Descriptor(execution.InputWriteTool)
			task, err = m.updateTask(task.ID, func(saved *Task) error {
				saved.Status = TaskRunning
				saved.Checkpoint = &Checkpoint{ExecutingCall: "call-source", ExecutingCapability: &d, Calls: []api.ToolCall{{ID: "call-source", Type: "function", Function: api.FunctionCall{Name: execution.InputWriteTool, Arguments: args}}}}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			payload, proof, err := ResolveInputWrite(task, []byte(args))
			if err != nil {
				t.Fatal(err)
			}
			data, _ := json.Marshal(payload)
			intent, err := execution.NewIntent(execution.Binding{Actor: "alice", Workspace: "workspace", Job: task.ID, Host: ref.Host, Session: ref.Session, Grant: ref.Grant, Revision: ref.Revision}, "call-source", "file_write", data, false)
			if err != nil {
				t.Fatal(err)
			}
			forged := payload
			forged.Data = []byte("not requested")
			bad, _ := json.Marshal(forged)
			forgedIntent, _ := execution.NewIntent(intent.Binding, intent.ID, intent.Operation, bad, false)
			if m.RecordExecutionIntent(task.ID, task.Actor, execution.InputWriteTool, []byte(args), forgedIntent) == nil {
				t.Fatal("substituted source bytes admitted")
			}
			if err = m.RecordExecutionIntent(task.ID, task.Actor, execution.InputWriteTool, []byte(args), intent); err != nil {
				t.Fatal(err)
			}
			written := execution.WriteResult{Location: execution.Location{Host: "host", Path: "/fixture/result.txt"}, Revision: proof.ContentDigest, Bytes: len(payload.Data), Created: true, Verified: true}
			if tampered {
				written.Revision = strings.Repeat("0", 64)
			}
			result, _ := json.Marshal(written)
			response := execution.Response{Version: 1, ID: intent.ID, Receipt: &execution.Receipt{State: "complete", Result: result}}
			if err = m.RecordExecutionReceipt(task.ID, task.Actor, intent, response); err != nil {
				t.Fatal(err)
			}
			restored := NewManagerWithPersistence(nil, nil, nil, dir)
			saved, ok := restored.GetTask(task.ID)
			if !ok {
				t.Fatal("saved intent lost")
			}
			if tampered {
				if saved.Status != TaskUncertain || len(saved.Steps) != 0 || saved.CompletedAt != nil {
					t.Fatal("false receipt established completion", saved.Status)
				}
			} else {
				if saved.Status != TaskInterrupted || len(saved.Steps) != 1 || ValidateInputWrites(saved) != nil || !strings.Contains(saved.Steps[0].ToolResult, `"historical":true`) {
					t.Fatal("matching receipt not recovered as historical evidence", saved.Status)
				}
				if recoverExecutionReceipt(saved) {
					t.Fatal("source write incorporated twice")
				}
			}
			// Recovery is evidence-only; there is no executor or live host in this fixture.
			r.Shutdown(context.Background())
		})
	}
}
