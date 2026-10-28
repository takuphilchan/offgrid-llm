package agents

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type executionStopFixture struct {
	*runTestTools
	started chan struct{}
	release chan struct{}
	fail    bool
}

func (f *executionStopFixture) StopOwnedExecution(ctx context.Context, task *Task) error {
	if !HasOwnedExecutionProcesses(task) {
		return errors.New("wrong job")
	}
	close(f.started)
	select {
	case <-f.release:
	case <-ctx.Done():
		return ctx.Err()
	}
	if f.fail {
		return errors.New("fixture host disconnected")
	}
	return nil
}

func TestStopDoesNotAcknowledgeBeforeOwnedProcessTermination(t *testing.T) {
	for _, fail := range []bool{false, true} {
		for _, inFlight := range []bool{false, true} {
			m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
			f := &executionStopFixture{runTestTools: &runTestTools{}, started: make(chan struct{}), release: make(chan struct{}), fail: fail}
			r := NewRunner(m, f, nil)
			ref := execution.Reference{Host: "host", Session: "session", Grant: "grant", Revision: 1}
			config := DefaultAgentConfig()
			config.Execution, config.ExecutionWorkspace = &ref, &execution.Resource{Reference: ref, Backend: "local", Root: "/fixture", Directory: ".", Permissions: execution.Permissions{Processes: true}}
			task, err := r.Create("test project", "model", "alice", config)
			if err != nil {
				t.Fatal(err)
			}
			_, err = m.updateTask(task.ID, func(current *Task) error {
				current.Status = TaskRunning
				current.Steps = append(current.Steps, Step{Type: "action", ToolName: "workspace_process_start"})
				if inFlight {
					current.Checkpoint.ExecutingCall = "pending-start"
					current.Checkpoint.Calls = []api.ToolCall{{ID: "pending-start", Function: api.FunctionCall{Name: "workspace_process_start"}}}
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { _, err := r.Stop(task.ID, "alice", "cancel", ""); done <- err }()
			select {
			case <-f.started:
			case <-time.After(time.Second):
				t.Fatal("stop controller not called")
			}
			current, _ := m.GetTask(task.ID)
			if current.Status == TaskCancelled || current.CompletedAt != nil || !inFlight && current.ErrorCode != "execution_stopping" || inFlight && current.Status != TaskUncertain {
				t.Fatal("premature stop acknowledgment", current)
			}
			close(f.release)
			if err = <-done; err != nil {
				t.Fatal(err)
			}
			current, _ = m.GetTask(task.ID)
			if fail {
				if current.Status != TaskUncertain || current.ErrorCode != "execution_stop_unconfirmed" || current.CompletedAt != nil {
					t.Fatal(current)
				}
			} else if inFlight {
				if current.Status != TaskUncertain || current.ErrorCode != "execution_effects_unknown" || current.CompletedAt != nil {
					t.Fatal("confirmed stop erased unknown effects", current)
				}
			} else if current.Status != TaskCancelled || current.CompletedAt == nil {
				t.Fatal(current)
			}
		}
	}
}
