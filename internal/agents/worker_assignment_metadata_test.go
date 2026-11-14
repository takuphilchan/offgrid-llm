package agents

import (
	"context"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestWorkerAssignmentDeliverablePersistsAndReachesWorker(t *testing.T) {
	m, r, scope, conversation := controlFixture(t)
	r.caller = func(_ context.Context, task *Task, messages []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		if task.ParentID == "" {
			return runAnswer(api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{{ID: "assign", Type: "function", Function: api.FunctionCall{Name: "delegate_tasks", Arguments: `{"tasks":[{"key":"research","goal":"Assess the alternatives","expected_result":"Comparison with source citations and uncertainties","reason":"Independent source assessment benefits this comparison","depends_on":[],"tools":[]}]}`}}}}), nil
		}
		found := false
		for _, message := range messages {
			found = found || strings.Contains(message.StringContent(), "Comparison with source citations and uncertainties")
		}
		if !found {
			t.Error("worker lost the requested deliverable")
		}
		return runAnswer(api.ChatMessage{Role: "assistant", Content: "Comparison"}), nil
	}
	work := admitControlTestWork(t, r, scope, conversation)
	root, err := r.Continue(context.Background(), work.RunID, scope.Actor, "start", "", false)
	if err != nil || len(root.ChildHistory) != 1 {
		t.Fatalf("delegation: %v, %v", root, err)
	}
	child, _ := m.GetTask(root.ChildHistory[0].ID)
	reloaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	worker, err := reloaded.WorkerSession(scope, child.Worker.SessionID)
	if err != nil || worker.Assignment.ExpectedResult != "Comparison with source citations and uncertainties" || worker.Assignment.Reason == "" || worker.Assignment.Mode != "research" {
		t.Fatalf("lost durable assignment: %+v, %v", worker, err)
	}
	if _, err := r.Continue(context.Background(), child.ID, scope.Actor, "start", "", false); err != nil {
		t.Fatal(err)
	}
	worker, _ = m.WorkerSession(scope, worker.ID)
	receipt, err := r.CommandWorker(scope, api.WorkerCommand{RequestID: "assignment-followup", TargetID: worker.ID, ExpectedRevision: worker.Revision, Kind: "continue", Payload: api.WorkerCommandPayload{Content: "Explain the limitations"}})
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Worker.Assignment.ExpectedResult != "Explain the limitations" || receipt.Worker.Assignment.Reason != "" {
		t.Fatal("follow-up claimed stale deliverable", receipt.Worker.Assignment)
	}
}

func TestWorkerAssignmentBoundsAndReuse(t *testing.T) {
	valid := ChildSpec{Key: "research", Goal: "Review", ExpectedResult: "Evidence", Reason: "Independent review"}
	for _, text := range []string{" ", strings.Repeat("a", 4097), "bad\x00text"} {
		invalid := valid
		invalid.ExpectedResult = text
		if validateChildSpecs([]ChildSpec{invalid}) == nil {
			t.Errorf("accepted invalid expected result of length %d", len(text))
		}
	}
	invalid := valid
	invalid.Reason = strings.Repeat("a", 2049)
	if validateChildSpecs([]ChildSpec{invalid}) == nil {
		t.Fatal("accepted oversized rationale")
	}
	changed := valid
	changed.ExpectedResult = "Different evidence"
	if sameChildSpec(valid, changed) {
		t.Fatal("reused a different deliverable")
	}
	changed = valid
	changed.Reason = "Different reason"
	if sameChildSpec(valid, changed) {
		t.Fatal("reused a different assignment rationale")
	}
	if err := validateChildSpecs([]ChildSpec{{Key: "legacy", Goal: "Original goal"}}); err != nil {
		t.Fatal("legacy checkpoint incompatible", err)
	}
}
