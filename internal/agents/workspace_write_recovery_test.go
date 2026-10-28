package agents

import (
	"encoding/json"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
)

func TestWriteRepairRequiresPersistedMatchingNoEffectReceipt(t *testing.T) {
	for _, kind := range []string{"exists", "revision", "missing", "uncertain", "partial", "complete", "wrong-code", "foreign-descriptor", "different-call", "legacy"} {
		t.Run(kind, func(t *testing.T) {
			m, task, request := executionReceiptFixture(t, t.TempDir())
			var err error
			task, err = m.updateTask(task.ID, func(next *Task) error {
				next.Config.TaskFirst = true
				if kind != "legacy" {
					next.Config.ExecutionRequestVersion = 1
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			state, code := "rejected", "execution_target_exists"
			switch kind {
			case "revision":
				code = "execution_revision_changed"
			case "uncertain", "partial", "complete":
				state = kind
			case "wrong-code":
				code = "execution_authority_denied"
			}
			if kind != "missing" {
				data, _ := json.Marshal(map[string]string{"error": code})
				if err = m.RecordExecutionReceipt(task.ID, task.Actor, request, execution.Response{Version: 1, ID: request.ID, Receipt: &execution.Receipt{State: state, Result: data}}); err != nil {
					t.Fatal(err)
				}
			}
			d, _ := execution.Descriptor("workspace_write")
			if kind == "foreign-descriptor" {
				d.Source = "mcp:untrusted"
			}
			if kind == "different-call" {
				task.Checkpoint.ExecutingCall = "other"
			}
			r := NewRunner(m, nil, nil)
			handled, err := r.repairWorkspaceWritePrecondition(task, d, &ToolFailure{Code: "execution_write_precondition", Recovery: "Read the current preimage", Message: "Rejected before changes"})
			if kind == "exists" || kind == "revision" {
				if err != nil || !handled || task.Checkpoint.ExecutingCall != "" || task.Checkpoint.ExecutionIntent != nil || task.Checkpoint.PreEffectRepairs != 1 || len(task.Steps) != 1 || !task.Checkpoint.PreEffectRepair {
					t.Fatal(handled, err, task.Checkpoint)
				}
				saved, _ := m.GetTask(task.ID)
				if saved.Checkpoint.ExecutionIntent != nil || saved.Status != TaskRunning {
					t.Fatal("unsafe persisted checkpoint")
				}
			} else if handled || err != nil {
				t.Fatal("unproven outcome allowed repair", handled, err)
			}
		})
	}
}
