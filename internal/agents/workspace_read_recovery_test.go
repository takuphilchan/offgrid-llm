package agents

import (
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
)

func TestWorkspaceReadRecoveryCannotAuthorizeForeignToolsOrSettleWrites(t *testing.T) {
	d, _ := execution.Descriptor("workspace_read")
	for _, name := range []string{"namespace", "source", "revision", "kind", "write", "unknown-outcome", "no-guidance", "pending-mutation"} {
		t.Run(name, func(t *testing.T) {
			descriptor := d
			failure := &ToolFailure{Code: "execution_rejected", Recovery: "Local read guidance"}
			task := &Task{Checkpoint: &Checkpoint{}}
			switch name {
			case "namespace":
				descriptor.Namespace = "mcp"
			case "source":
				descriptor.Source = "provider"
			case "revision":
				descriptor.Revision = "unreviewed"
			case "kind":
				descriptor.Kind = capabilities.Write
			case "write":
				descriptor, _ = execution.Descriptor("workspace_write")
			case "unknown-outcome":
				failure.Code = "transport_lost"
			case "no-guidance":
				failure.Recovery = ""
			case "pending-mutation":
				task.Checkpoint.ExecutionIntent = &ExecutionIntent{Tool: "workspace_write"}
			}
			// A nil runner proves no persistence/dispatch path was entered.
			var runner *Runner
			if handled, err := runner.repairWorkspaceReadFailure(task, descriptor, failure); handled || err != nil || task.Checkpoint.WorkspaceReadFailures != 0 {
				t.Fatal("untrusted or uncertain effect admitted to read recovery", handled, err)
			}
		})
	}
}
