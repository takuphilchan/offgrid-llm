package agents

import (
	"context"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
)

type integrationReceiptReader struct {
	RunTools
	reply execution.WorkerArtifactReply
	want  execution.IntegrateChangeSetRequest
	reads int
}

func (r *integrationReceiptReader) ReadWorkerIntegrationReceipt(_ context.Context, _ *Task, req execution.IntegrateChangeSetRequest) (execution.WorkerArtifactReply, error) {
	r.reads++
	if req != r.want {
		return execution.WorkerArtifactReply{}, execution.ErrConflict
	}
	return r.reply, nil
}

func TestWorkerIntegrationRecoveryRequiresExplicitContinuation(t *testing.T) {
	for _, mode := range []string{"saved", "host", "unknown", "stop"} {
		t.Run(mode, func(t *testing.T) {
			m, root, child := integrationBudgetFixture(t)
			req, _, err := m.ReserveWorkerIntegration(root.ID, root.Actor, child, "integrate")
			if err != nil {
				t.Fatal(err)
			}
			if items := m.WorkerIntegrationObservations("", 16); len(items) != 0 {
				t.Fatal("observer competing with active tool")
			}
			_, err = m.updateTask(root.ID, func(task *Task) error {
				task.Status = TaskUncertain
				if mode == "stop" {
					task.ControlAction = "stop"
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			items := m.WorkerIntegrationObservations("", 16)
			if len(items) != 1 || items[0].Request != req || len(m.WorkerIntegrationObservations(items[0].Key, 16)) != 0 {
				t.Fatal("unresolved observation cursor", items)
			}
			reply := execution.WorkerArtifactReply{State: "integrated", Integration: &execution.IntegrationPage{ID: execution.IntegrationRequestID(req), ChangeSetID: req.ChangeSetID, State: "integrated", Total: 1, Integrated: 1}}
			if mode == "unknown" {
				reply = execution.WorkerArtifactReply{State: "uncertain"}
			}
			if mode == "saved" {
				if err = m.RecordWorkerIntegration(root.ID, root.Actor, child, req, reply); err != nil {
					t.Fatal(err)
				}
			}
			reader := &integrationReceiptReader{reply: reply, want: req}
			r := NewRunner(m, reader, nil)
			defer r.Shutdown(context.Background())
			_, err = r.RecoverExecution(context.Background(), root.ID, root.Actor)
			current, _ := m.GetTask(root.ID)
			if mode == "unknown" || mode == "stop" {
				if err == nil || current.Status != TaskUncertain || current.Checkpoint.ExecutingCall == "" {
					t.Fatal("unknown or stopped work resumed", err, current.Status)
				}
			} else {
				if err != nil || current.Status != TaskInterrupted || current.Checkpoint.ExecutingCall != "" || current.Steps[len(current.Steps)-1].ToolName != execution.IntegrateTool {
					t.Fatal("lost recovery result", err)
				}
				if _, err = r.RecoverExecution(context.Background(), root.ID, root.Actor); err == nil {
					t.Fatal("recovery applied twice")
				}
			}
			wantReads := 1
			if mode == "saved" {
				wantReads = 0
			}
			if reader.reads != wantReads {
				t.Fatal("unexpected host reads", reader.reads)
			}
			if mode != "unknown" && len(m.WorkerIntegrationObservations("", 16)) != 0 {
				t.Fatal("settled receipt still queued")
			}
		})
	}
}
