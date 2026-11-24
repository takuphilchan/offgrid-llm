package agents

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func processBudgetFixture(t *testing.T) (*Manager, *Task, execution.Request) {
	t.Helper()
	m, task, request := executionReceiptFixture(t, t.TempDir())
	args := `{"program":"go","arguments":["test","."],"timeout_seconds":10}`
	d, _ := execution.Descriptor("workspace_process_start")
	var err error
	task, err = m.updateTask(task.ID, func(t *Task) error {
		t.Checkpoint = &Checkpoint{ExecutingCall: "start", ExecutingCapability: &d, Calls: []api.ToolCall{{ID: "start", Type: "function", Function: api.FunctionCall{Name: "workspace_process_start", Arguments: args}}}}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	request, err = execution.NewIntent(request.Binding, "start", "process_start", []byte(args), false)
	if err != nil {
		t.Fatal(err)
	}
	return m, task, request
}

func processBudgetResponse(request execution.Request, handle, state, receipt string, elapsed time.Duration, settled bool) execution.Response {
	var exit *int
	if settled {
		code := 0
		exit = &code
	}
	descendants := &execution.ProcessDescendants{Version: 1, State: "unknown"}
	if state == "running" {
		descendants.State = "pending"
	} else if settled {
		descendants.State = "settled"
	}
	data, _ := json.Marshal(execution.ProcessResult{Handle: handle, State: state, ExitCode: exit, Runtime: &execution.ProcessRuntime{Version: 1, ElapsedNS: int64(elapsed), Settled: settled}, Descendants: descendants})
	return execution.Response{Version: 1, ID: request.ID, Receipt: &execution.Receipt{State: receipt, Result: data}}
}

func TestProcessBudgetReservesAndSettlesAuthenticatedExitOnce(t *testing.T) {
	m, stale, request := processBudgetFixture(t)
	for i := 0; i < 2; i++ {
		if err := m.RecordExecutionIntent(stale.ID, stale.Actor, "workspace_process_start", request.Arguments, request); err != nil {
			t.Fatal(err)
		}
	}
	saved, _ := m.GetTask(stale.ID)
	b := saved.Checkpoint.ProcessBudget
	if b.ReservedStarts != 1 || b.ChargedNS != int64(10*time.Second) || b.UnsettledStarts != 1 {
		t.Fatal(b)
	}
	if err := m.RecordExecutionReceipt(stale.ID, stale.Actor, request, processBudgetResponse(request, "owned", "running", "dispatched", time.Millisecond, false)); err != nil {
		t.Fatal(err)
	}
	// A stale model snapshot cannot erase the adapter's committed reservation.
	r := NewRunner(m, &runTestTools{}, nil)
	if err := r.persist(stale); err != nil {
		t.Fatal(err)
	}
	if stale.Checkpoint.ProcessBudget.ReservedStarts != 1 {
		t.Fatal("snapshot lost host accounting")
	}
	_, err := m.updateTask(stale.ID, func(t *Task) error { t.Status = TaskUncertain; return nil })
	if err != nil {
		t.Fatal(err)
	}
	terminal := processBudgetResponse(request, "owned", "cancelled", "complete", time.Second, true)
	for i := 0; i < 2; i++ {
		if err = m.RecordExecutionReceipt(stale.ID, stale.Actor, request, terminal); err != nil {
			t.Fatal(err)
		}
	}
	saved, _ = m.GetTask(stale.ID)
	b = saved.Checkpoint.ProcessBudget
	if saved.Status != TaskUncertain || b.ChargedNS != int64(time.Second) || b.MeasuredNS != int64(time.Second) || b.UnsettledStarts != 0 {
		t.Fatal("late terminal receipt revived work or mischarged", saved.Status, b)
	}
	loaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err = loaded.StorageError(); err != nil {
		t.Fatal(err)
	}
	restored, _ := loaded.GetTask(stale.ID)
	if !reflect.DeepEqual(b, restored.Checkpoint.ProcessBudget) {
		t.Fatal("restart changed settled usage")
	}
}

func TestProcessBudgetObservationsRequireOwnedHandleAndBinding(t *testing.T) {
	m, task, start := processBudgetFixture(t)
	if err := m.RecordExecutionIntent(task.ID, task.Actor, "workspace_process_start", start.Arguments, start); err != nil {
		t.Fatal(err)
	}
	if err := m.RecordExecutionReceipt(task.ID, task.Actor, start, processBudgetResponse(start, "owned", "running", "dispatched", time.Millisecond, false)); err != nil {
		t.Fatal(err)
	}
	d, _ := execution.Descriptor("workspace_process_wait")
	args := `{"handle":"owned","wait_ms":1000}`
	_, err := m.updateTask(task.ID, func(t *Task) error {
		t.Checkpoint.ExecutingCall = "wait"
		t.Checkpoint.ExecutingCapability = &d
		t.Checkpoint.Calls = []api.ToolCall{{ID: "wait", Function: api.FunctionCall{Name: "workspace_process_wait", Arguments: args}}}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	request, _ := execution.NewIntent(start.Binding, "wait", "process_wait", []byte(args), false)
	response := processBudgetResponse(request, "owned", "exited", "complete", 2*time.Second, true)
	for _, bad := range []string{"job", "grant", "handle", "time", "terminal"} {
		copyReq, copyResp := request, response
		switch bad {
		case "job":
			copyReq.Binding.Job = "foreign"
		case "grant":
			copyReq.Binding.Grant = "foreign"
		case "handle":
			copyResp = processBudgetResponse(request, "foreign", "exited", "complete", time.Second, true)
		case "time":
			copyResp = processBudgetResponse(request, "owned", "exited", "complete", -time.Second, true)
		case "terminal":
			copyResp = processBudgetResponse(request, "owned", "running", "complete", time.Second, true)
		}
		if m.RecordProcessObservation(task.ID, task.Actor, copyReq, copyResp) == nil {
			t.Fatal("invalid usage accepted", bad)
		}
	}
	for i := 0; i < 2; i++ {
		if err = m.RecordProcessObservation(task.ID, task.Actor, request, response); err != nil {
			t.Fatal(err)
		}
	}
	saved, _ := m.GetTask(task.ID)
	if b := saved.Checkpoint.ProcessBudget; b.ChargedNS != int64(2*time.Second) || b.MeasuredNS != int64(2*time.Second) || b.UnsettledStarts != 0 {
		t.Fatal(b)
	}
}

func TestProcessBudgetUnknownAndOverrunRetainCapacity(t *testing.T) {
	m, task, request := processBudgetFixture(t)
	if err := m.RecordExecutionIntent(task.ID, task.Actor, "workspace_process_start", request.Arguments, request); err != nil {
		t.Fatal(err)
	}
	if err := m.RecordExecutionReceipt(task.ID, task.Actor, request, processBudgetResponse(request, "owned", "running", "dispatched", 12*time.Second, false)); err != nil {
		t.Fatal(err)
	}
	saved, _ := m.GetTask(task.ID)
	if b := saved.Checkpoint.ProcessBudget; b.ChargedNS != int64(12*time.Second) || b.MeasuredNS != int64(12*time.Second) || b.UnsettledStarts != 1 {
		t.Fatal(b)
	}
	loaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err := loaded.StorageError(); err != nil {
		t.Fatal(err)
	}
	restored, _ := loaded.GetTask(task.ID)
	if restored.Status != TaskUncertain || !reflect.DeepEqual(saved.Checkpoint.ProcessBudget, restored.Checkpoint.ProcessBudget) {
		t.Fatal("unknown process refunded after restart")
	}
}

func TestProcessBudgetFailedReservationOrReceiptNeverRefunds(t *testing.T) {
	for _, failure := range []string{"reservation", "receipt"} {
		t.Run(failure, func(t *testing.T) {
			m, task, request := processBudgetFixture(t)
			db, err := m.controlDatabase()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			condition := `json_extract(NEW.snapshot,'$.checkpoint.process_budget.reserved_starts')>0`
			if failure == "receipt" {
				if err = m.RecordExecutionIntent(task.ID, task.Actor, "workspace_process_start", request.Arguments, request); err != nil {
					t.Fatal(err)
				}
				condition = `json_extract(NEW.snapshot,'$.checkpoint.process_budget.unsettled_starts')=0`
			}
			if _, err = db.Exec(`CREATE TRIGGER reject_process_budget BEFORE UPDATE ON agent_tasks WHEN ` + condition + ` BEGIN SELECT RAISE(ABORT,'injected'); END`); err != nil {
				t.Fatal(err)
			}
			if failure == "reservation" {
				err = m.RecordExecutionIntent(task.ID, task.Actor, "workspace_process_start", request.Arguments, request)
			} else {
				err = m.RecordExecutionReceipt(task.ID, task.Actor, request, processBudgetResponse(request, "owned", "exited", "complete", time.Second, true))
			}
			if err == nil {
				t.Fatal("uncommitted accounting accepted")
			}
			if _, err = db.Exec(`DROP TRIGGER reject_process_budget`); err != nil {
				t.Fatal(err)
			}
			loaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
			if err = loaded.StorageError(); err != nil {
				t.Fatal(err)
			}
			restored, _ := loaded.GetTask(task.ID)
			if failure == "reservation" {
				if restored.Checkpoint.ProcessBudget != nil {
					t.Fatal("failed reservation survived")
				}
			} else if b := restored.Checkpoint.ProcessBudget; b.UnsettledStarts != 1 || b.ChargedNS != int64(10*time.Second) || b.MeasuredNS != 0 {
				t.Fatal("failed receipt exposed refund", b)
			}
		})
	}
}
