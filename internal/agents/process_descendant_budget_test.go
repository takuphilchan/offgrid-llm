package agents

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
)

func TestProcessMissingOrUnknownDescendantsCannotRefundMeasuredBudget(t *testing.T) {
	for _, proof := range []*execution.ProcessDescendants{nil, {Version: 1, State: "unknown"}, {Version: 2, State: "settled"}, {Version: 1, State: "invalid"}} {
		m, task, request := processBudgetFixture(t)
		if err := m.RecordExecutionIntent(task.ID, task.Actor, "workspace_process_start", request.Arguments, request); err != nil {
			t.Fatal(err)
		}
		response := processBudgetResponse(request, "owned", "exited", "complete", time.Second, true)
		var result execution.ProcessResult
		if err := json.Unmarshal(response.Receipt.Result, &result); err != nil {
			t.Fatal(err)
		}
		result.Descendants = proof
		response.Receipt.Result, _ = json.Marshal(result)
		if err := m.RecordExecutionReceipt(task.ID, task.Actor, request, response); err != nil {
			t.Fatal(err)
		}
		saved, _ := m.GetTask(task.ID)
		budget := saved.Checkpoint.ProcessBudget
		if budget.UnsettledStarts != 1 || budget.ChargedNS != int64(10*time.Second) || budget.MeasuredNS != int64(time.Second) {
			t.Fatal("unproved tree refunded measured reservation", proof, budget)
		}
	}
}
