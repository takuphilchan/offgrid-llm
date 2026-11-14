package agents

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestWorkerContinuationInvalidatesDependenciesWithoutRewritingCompletion(t *testing.T) {
	m, r, scope, c := controlFixture(t)
	defer r.Shutdown(context.Background())
	var sourceCalls, writerCalls atomic.Int32
	r.caller = func(_ context.Context, task *Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		if task.ParentID == "" {
			return taskCall("delegate_tasks", `{"tasks":[{"key":"source","goal":"Source facts","depends_on":[],"tools":[]},{"key":"writer","goal":"Write from source","depends_on":["source"],"tools":[]}]}`), nil
		}
		if task.Prompt == "Write from source" {
			writerCalls.Add(1)
			return runAnswer(api.ChatMessage{Role: "assistant", Content: "Original writer output"}), nil
		}
		sourceCalls.Add(1)
		return runAnswer(api.ChatMessage{Role: "assistant", Content: task.Prompt}), nil
	}
	work := admitControlTestWork(t, r, scope, c)
	root, err := r.Continue(context.Background(), work.RunID, scope.Actor, "start", "", false)
	if err != nil {
		t.Fatal(err)
	}
	source, _ := m.GetTask(root.ChildHistory[0].ID)
	writer, _ := m.GetTask(root.ChildHistory[1].ID)
	if _, err = r.Continue(context.Background(), source.ID, scope.Actor, "start", "", false); err != nil {
		t.Fatal(err)
	}
	r.advanceDelegation(context.Background(), root.ID)
	writer = awaitTask(t, m, writer.ID, TaskCompleted)
	settleTask(t, r, writer.ID)
	worker, _ := m.WorkerSession(scope, source.Worker.SessionID)
	continued, err := r.CommandWorker(scope, api.WorkerCommand{RequestID: "new-source", TargetID: worker.ID, ExpectedRevision: worker.Revision, Kind: "continue", Payload: api.WorkerCommandPayload{Content: "Revised source facts"}})
	if err != nil {
		t.Fatal(err)
	}
	retained, _ := m.GetTask(writer.ID)
	if retained.Status != TaskCompleted || retained.Result != writer.Result {
		t.Fatal("prior completion was rewritten", retained)
	}
	updatedRoot, _ := m.GetTask(root.ID)
	if !updatedRoot.Delegation.Children[1].Invalidated {
		t.Fatal("dependent result remained current")
	}
	oldSource, _ := m.GetTask(source.ID)
	page, err := r.historyPage(context.Background(), updatedRoot, resultReference(oldSource), 0)
	if err != nil {
		t.Fatal(err)
	}
	var metadata struct {
		Stale bool `json:"stale"`
	}
	if json.Unmarshal([]byte(page), &metadata) != nil || !metadata.Stale {
		t.Fatal("superseded source lacked provenance", page)
	}
	newSource, err := r.Continue(context.Background(), continued.Worker.CurrentRunID, scope.Actor, "start", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.historyPage(context.Background(), writer, resultReference(newSource), 0); err == nil {
		t.Fatal("dependent read an unbound result revision")
	}
	r.advanceDelegation(context.Background(), root.ID)
	partial, _ := m.GetTask(root.ID)
	if partial.Status != TaskFailed || partial.ErrorCode != "delegation_partial" || !strings.Contains(partial.Result, "revalidation is required") || writerCalls.Load() != 1 || sourceCalls.Load() != 2 {
		t.Fatal("stale branch integrated or reran", partial, writerCalls.Load(), sourceCalls.Load())
	}
}

func TestWorkerContinuationRollbackKeepsIdentityAndAllocation(t *testing.T) {
	m, r, scope, root, child := workerFixture(t)
	if _, err := r.Continue(context.Background(), child.ID, scope.Actor, "start", "", false); err != nil {
		t.Fatal(err)
	}
	worker, _ := m.WorkerSession(scope, child.Worker.SessionID)
	parent, _ := m.GetTask(root.ID)
	db, err := m.controlDatabase()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TRIGGER fail_worker_followup BEFORE INSERT ON agent_control_commands WHEN NEW.request_id='follow-up' BEGIN SELECT RAISE(ABORT,'injected'); END`); err != nil {
		t.Fatal(err)
	}
	command := api.WorkerCommand{RequestID: "follow-up", TargetID: worker.ID, ExpectedRevision: worker.Revision, Kind: "continue", Payload: api.WorkerCommandPayload{Content: "Review again"}}
	if _, err = r.CommandWorker(scope, command); err == nil {
		t.Fatal("injected failure ignored")
	}
	after, _ := m.GetTask(root.ID)
	saved, _ := m.WorkerSession(scope, worker.ID)
	if len(after.ChildHistory) != len(parent.ChildHistory) || after.Checkpoint.ReservedChildIterations != parent.Checkpoint.ReservedChildIterations || saved.CurrentRunID != child.ID || saved.Revision != worker.Revision {
		t.Fatal("rolled-back follow-up published", after, saved)
	}
	var count int
	if err = db.QueryRow(`SELECT count(*) FROM agent_tasks`).Scan(&count); err != nil || count != 2 {
		t.Fatal(count, err)
	}
}
