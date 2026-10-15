package agents

import (
	"context"
	"fmt"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type applicationDeclarationTools struct{ workflowTestTools }

func (*applicationDeclarationTools) ValidateDeclaredExpectations(*Task, []OutcomeExpectation) error {
	return fmt.Errorf("application_id does not match the latest unique name lookup")
}

func TestApplicationDeclarationRejectionDoesNotFreezeWrongOutcome(t *testing.T) {
	tools := &applicationDeclarationTools{}
	calls := 0
	r := NewRunner(NewManagerWithPersistence(nil, nil, nil, t.TempDir()), tools, func(_ context.Context, task *Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		if task.Workflow != nil && len(task.Workflow.Checks) != 0 {
			t.Fatal("rejected application check was saved")
		}
		return taskCall("task_expectations", `{"expectations":[{"id":"open","title":"Open Settings","kind":"application_open","application_id":"access"}]}`), nil
	})
	task, err := r.Create("Open Settings", "fixture", "alice", workflowConfig())
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.Continue(context.Background(), task.ID, "alice", "start", "", false)
	if err != nil || got.Status != TaskInterrupted || got.ErrorCode != proposalRepairLimitCode || calls != 4 || tools.calls.Load() != 0 {
		t.Fatalf("unbounded or dispatched declaration repair: calls=%d status=%s code=%s err=%v", calls, got.Status, got.ErrorCode, err)
	}
	if len(got.Workflow.Checks) != 0 || got.Checkpoint.ExecutingCall != "" {
		t.Fatal("rejection retained an outcome or executing call")
	}
}
