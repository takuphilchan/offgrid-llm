package agents

import (
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestWorkerEndedAssignmentResolvesUnappliedMailbox(t *testing.T) {
	for _, ending := range []string{"worker_stop", "work_stop", "failure"} {
		for _, delivered := range []bool{false, true} {
			t.Run(ending+map[bool]string{true: "/delivered", false: "/accepted"}[delivered], func(t *testing.T) {
				m, r, scope, root, child := workerFixture(t)
				worker, _ := m.WorkerSession(scope, child.Worker.SessionID)
				command := api.WorkerCommand{RequestID: "unapplied", TargetID: worker.ID, ExpectedRevision: worker.Revision, Kind: "message", Payload: api.WorkerCommandPayload{Content: "Keep the pending evidence"}}
				if _, err := r.CommandWorker(scope, command); err != nil {
					t.Fatal(err)
				}
				if delivered {
					if err := m.deliverWorkerMessages(child.ID); err != nil {
						t.Fatal(err)
					}
				}
				var err error
				switch ending {
				case "worker_stop":
					worker, _ = m.WorkerSession(scope, worker.ID)
					_, err = r.CommandWorker(scope, api.WorkerCommand{RequestID: "stop", TargetID: worker.ID, ExpectedRevision: worker.Revision, Kind: "stop"})
				case "work_stop":
					work, e := m.WorkItem(scope, root.WorkID)
					if e != nil {
						t.Fatal(e)
					}
					_, err = r.ControlWork(scope, api.WorkCommand{RequestID: "stop", TargetID: work.ID, ExpectedRevision: work.Revision, Kind: "stop"})
				case "failure":
					_, err = m.updateTask(child.ID, func(task *Task) error { finishTask(task, TaskFailed, "Model unavailable"); return nil })
				}
				if err != nil {
					t.Fatal(err)
				}
				page, err := m.WorkerMessages(scope, worker.ID, 0, 10)
				if err != nil || len(page.Items) != 1 {
					t.Fatal(page, err)
				}
				message := page.Items[0]
				if message.State != "abandoned" || message.AbandonedAt == nil || message.AppliedAt != nil || (message.DeliveredAt != nil) != delivered || message.RunID != child.ID || message.Content != command.Payload.Content || message.ResolutionReason == "" {
					t.Fatal("unapplied message lost or falsely applied", message)
				}
				// Retried acceptance is the original receipt, not another delivery.
				if _, err := r.CommandWorker(scope, command); err != nil {
					t.Fatal(err)
				}
				worker, _ = m.WorkerSession(scope, worker.ID)
				closed, err := r.CommandWorker(scope, api.WorkerCommand{RequestID: "close", TargetID: worker.ID, ExpectedRevision: worker.Revision, Kind: "close"})
				if err != nil || !closed.Worker.Closed {
					t.Fatal(closed, err)
				}
				loaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
				page, err = loaded.WorkerMessages(scope, worker.ID, 0, 10)
				if err != nil || len(page.Items) != 1 || page.Items[0].State != "abandoned" {
					t.Fatal(page, err)
				}
			})
		}
	}
}

func TestWorkerPauseKeepsMailboxPending(t *testing.T) {
	m, r, scope, root, child := workerFixture(t)
	worker, _ := m.WorkerSession(scope, child.Worker.SessionID)
	if _, err := r.CommandWorker(scope, api.WorkerCommand{RequestID: "pending", TargetID: worker.ID, ExpectedRevision: worker.Revision, Kind: "message", Payload: api.WorkerCommandPayload{Content: "Apply after continuation"}}); err != nil {
		t.Fatal(err)
	}
	work, _ := m.WorkItem(scope, root.WorkID)
	if _, err := r.ControlWork(scope, api.WorkCommand{RequestID: "pause", TargetID: work.ID, ExpectedRevision: work.Revision, Kind: "pause"}); err != nil {
		t.Fatal(err)
	}
	page, err := m.WorkerMessages(scope, worker.ID, 0, 10)
	if err != nil || len(page.Items) != 1 || page.Items[0].State != "accepted" || page.Items[0].AbandonedAt != nil {
		t.Fatal(page, err)
	}
}
