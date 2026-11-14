package agents

import (
	"context"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestWorkerContinuationRetainsStoppedAndFailedFilesWithoutRevival(t *testing.T) {
	for _, status := range []TaskStatus{TaskCancelled, TaskFailed} {
		t.Run(string(status), func(t *testing.T) {
			m, root, childID, capture := captureBudgetFixture(t)
			r := NewRunner(m, nil, nil)
			defer r.Shutdown(context.Background())
			child, err := m.updateTask(childID, func(task *Task) error {
				finishTask(task, status, "Retained partial outcome")
				task.Config.ExecutionWorkerMode = "edit"
				task.ProcessStopPending = true
				if status == TaskCancelled {
					task.ControlAction = "stop"
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err = m.ReserveWorkerCapture(root.ID, root.Actor, childID); err == nil {
				t.Fatal("capture before process quiescence")
			}
			if !workerCapturePending(root, child) {
				t.Fatal("parent could finish before partial capture")
			}
			child, err = m.updateTask(childID, func(task *Task) error { task.ProcessStopPending = false; return nil })
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err = m.ReserveWorkerCapture(root.ID, root.Actor, childID); err != nil {
				t.Fatal(err)
			}
			scope := ControlScope{Workspace: child.Worker.Workspace, Actor: child.Actor}
			worker, err := m.WorkerSession(scope, child.Worker.SessionID)
			if err != nil {
				t.Fatal(err)
			}
			command := api.WorkerCommand{RequestID: "retained-followup", TargetID: worker.ID, ExpectedRevision: worker.Revision, Kind: "continue", Payload: api.WorkerCommandPayload{Content: "Continue from the retained private result"}}
			if _, err = r.CommandWorker(scope, command); err == nil {
				t.Fatal("follow-up discarded uncaptured private input")
			}
			if err = m.RecordWorkerCapture(root.ID, root.Actor, childID, capture, sealedBudgetReply(capture, childID)); err != nil {
				t.Fatal(err)
			}
			child, _ = m.GetTask(childID)
			if child.Status != status {
				t.Fatal("capture revived or relabeled partial work")
			}
			current, _ := m.GetTask(root.ID)
			if workerCapturePending(current, child) || workerDependencyComplete(current, child) {
				t.Fatal("partial capture treated as successful dependency")
			}
			if status == TaskCancelled {
				if _, err = r.continueWorker(scope, command, false); err == nil {
					t.Fatal("automatic stopped-worker revival")
				}
			}
			accepted, err := r.CommandWorker(scope, command)
			if err != nil {
				t.Fatal(err)
			}
			next, _ := m.GetTask(accepted.Worker.CurrentRunID)
			if next.Checkpoint.WorkerContinuation == nil || next.Checkpoint.WorkerContinuation.PreviousRunID != childID || next.Config.Execution != nil || next.Checkpoint.WorkerEnvironment != nil {
				t.Fatal("follow-up lost file provenance or reused authority")
			}
		})
	}
}
