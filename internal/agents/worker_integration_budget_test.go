package agents

import (
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func integrationBudgetFixture(t *testing.T) (*Manager, *Task, string) {
	t.Helper()
	m, root, child, seal := captureBudgetFixture(t)
	if err := m.RecordWorkerCapture(root.ID, root.Actor, child, seal, readyCandidateReply(seal, child)); err != nil {
		t.Fatal(err)
	}
	if _, err := m.updateTask(child, func(task *Task) error { task.Config.ExecutionWorkerMode = "test"; return nil }); err != nil {
		t.Fatal(err)
	}
	editor, err := m.CreateTask("integration-editor", "Edit", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.updateTask(editor.ID, func(task *Task) error {
		task.Actor, task.ParentID, task.InstructionVersion = root.Actor, root.ID, root.InstructionVersion
		task.Config.ExecutionWorkerMode = "edit"
		finishTask(task, TaskCompleted, "Edited")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	root, err = m.updateTask(root.ID, func(task *Task) error {
		task.Status = TaskRunning
		d, _ := execution.Descriptor(execution.IntegrateTool)
		task.Checkpoint.ExecutingCall, task.Checkpoint.ExecutingCapability = "integrate", &d
		task.Checkpoint.Calls = []api.ToolCall{{ID: "integrate", Type: "function", Function: api.FunctionCall{Name: execution.IntegrateTool, Arguments: `{"worker_run_id":"` + child + `"}`}}}
		b := task.Checkpoint.StorageBudget
		b.LimitBytes = 1000
		for key, entry := range b.Reservations {
			prior := entry
			prior.Request.WorkerRunID, prior.Request.RequestID = editor.ID, "edit-copy"
			prior.CopyID, prior.ChargedBytes, prior.Integration = "edit-copy", 100, nil
			prior.Capture = &WorkerCaptureReservation{Result: &WorkerCapturedOutput{State: "sealed", ChangeSetID: "edit-set", OperationsDigest: strings.Repeat("e", 64), OperationCount: 1, InstructionVersion: task.InstructionVersion, ControlEpoch: task.ControlEpoch}}
			entry.Request.Prerequisites = []execution.ChangeSetReference{{ID: "edit-set", OperationsDigest: prior.Capture.Result.OperationsDigest, InstructionVersion: task.InstructionVersion, ControlEpoch: task.ControlEpoch, SourceJob: task.ID}}
			b.Reservations[key] = entry
			b.Reservations["edit-copy"] = prior
			b.ChargedBytes += prior.ChargedBytes
			break
		}
		task.ChildHistory = append(task.ChildHistory, ChildLink{ID: editor.ID})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return m, root, child
}

func TestWorkerUnsupportedIntegrationKeepsReviewableOutcome(t *testing.T) {
	m, root, child := integrationBudgetFixture(t)
	req, _, err := m.ReserveWorkerIntegration(root.ID, root.Actor, child, "integrate")
	if err != nil {
		t.Fatal(err)
	}
	page := execution.IntegrationPage{ID: execution.IntegrationRequestID(req), ChangeSetID: req.ChangeSetID, State: "abandoned", Total: 1, Error: execution.ErrUnsupported.Error()}
	reply := execution.WorkerArtifactReply{State: "abandoned", Integration: &page}
	page.Integrated = 1
	if err = m.RecordWorkerIntegration(root.ID, root.Actor, child, req, reply); err == nil {
		t.Fatal("claimed no mutation despite integrated file")
	}
	page.Integrated = 0
	if err = m.RecordWorkerIntegration(root.ID, root.Actor, child, req, reply); err != nil {
		t.Fatal(err)
	}
	stored, _ := m.GetTask(root.ID)
	result := WorkerIntegrationOutput(stored, child)
	if result == nil || result.State != "abandoned" || result.Integrated != 0 {
		t.Fatal(result)
	}
	if _, fresh, err := m.ReserveWorkerIntegration(root.ID, root.Actor, child, "integrate"); err != nil || fresh {
		t.Fatal("unsupported integration was retried", err)
	}
}

func TestWorkerIntegrationAdmissionAndLateSettlement(t *testing.T) {
	m, root, child := integrationBudgetFixture(t)
	var owners atomic.Int32
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, fresh, err := m.ReserveWorkerIntegration(root.ID, root.Actor, child, "integrate")
			if err != nil {
				t.Error(err)
			}
			if fresh {
				owners.Add(1)
			}
		}()
	}
	wg.Wait()
	req, fresh, err := m.ReserveWorkerIntegration(root.ID, root.Actor, child, "integrate")
	if err != nil || fresh || owners.Load() != 1 {
		t.Fatal("duplicate dispatch ownership", fresh, owners.Load(), err)
	}
	before, _ := m.GetTask(root.ID)
	if before.Checkpoint.StorageBudget.ChargedBytes != 420 || req.MaxReservedBytes != 320 {
		t.Fatal("incorrect root reservation", before.Checkpoint.StorageBudget, req)
	}
	if err = m.RecordWorkerIntegration(root.ID, root.Actor, child, req, execution.WorkerArtifactReply{State: "uncertain"}); err != nil {
		t.Fatal(err)
	}
	if _, err = m.updateTask(root.ID, func(task *Task) error { finishTask(task, TaskCancelled, "Stopped"); return nil }); err != nil {
		t.Fatal(err)
	}
	restored := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err = restored.StorageError(); err != nil {
		t.Fatal(err)
	}
	current, _ := restored.GetTask(root.ID)
	if current.Checkpoint.StorageBudget.ChargedBytes != 420 || restored.WorkerIntegrationControlCurrent(root.ID, root.Actor, child, "integrate", req) {
		t.Fatal("restart refunded unknown or enabled dispatch")
	}
	reply := execution.WorkerArtifactReply{State: "integrated", Integration: &execution.IntegrationPage{ID: execution.IntegrationRequestID(req), ChangeSetID: req.ChangeSetID, State: "integrated", Total: 1, Integrated: 1}}
	for _, entry := range current.Checkpoint.StorageBudget.Reservations {
		if entry.Integration != nil {
			reply.Copy = &execution.WorkingCopyRecord{ID: entry.CopyID, Source: req.Source, WorkerRunID: child, Purpose: entry.Request.Purpose, State: "ready", ReservedBytes: 150, MeasuredBytes: 140, UsageMeasured: true}
		}
	}
	for range 2 {
		if err = restored.RecordWorkerIntegration(root.ID, root.Actor, child, req, reply); err != nil {
			t.Fatal(err)
		}
	}
	current, _ = restored.GetTask(root.ID)
	if current.Status != TaskCancelled || current.Checkpoint.StorageBudget.ChargedBytes != 250 || WorkerIntegrationOutput(current, child).State != "integrated" {
		t.Fatal("late receipt revived work or lost settlement")
	}
	reply.Integration.Integrated = 0
	if err = restored.RecordWorkerIntegration(root.ID, root.Actor, child, req, reply); err == nil {
		t.Fatal("contradictory completion accepted")
	}
}

func TestWorkerIntegrationRejectsObsoleteOrIncompleteCandidates(t *testing.T) {
	for _, mode := range []string{"stop", "correction", "epoch", "unvalidated", "omitted_edit", "duplicate_changed_digest", "foreign_source", "cycle"} {
		t.Run(mode, func(t *testing.T) {
			m, root, child := integrationBudgetFixture(t)
			_, err := m.updateTask(root.ID, func(task *Task) error {
				switch mode {
				case "stop":
					task.ControlAction = "stop"
				case "correction":
					task.InstructionVersion++
				case "epoch":
					task.ControlEpoch++
				}
				for key, entry := range task.Checkpoint.StorageBudget.Reservations {
					if entry.Request.WorkerRunID != child {
						continue
					}
					switch mode {
					case "unvalidated":
						entry.Capture.Result.Validation.State = "failed"
					case "omitted_edit":
						entry.Request.Prerequisites = nil
					case "duplicate_changed_digest":
						ref := entry.Request.Prerequisites[0]
						ref.OperationsDigest = strings.Repeat("f", 64)
						entry.Request.Prerequisites = append(entry.Request.Prerequisites, ref)
					case "foreign_source":
						entry.Request.Prerequisites[0].SourceJob = "other-root"
					case "cycle":
						prior := task.Checkpoint.StorageBudget.Reservations["edit-copy"]
						prior.Request.Prerequisites = entry.Request.Prerequisites
						task.Checkpoint.StorageBudget.Reservations["edit-copy"] = prior
					}
					task.Checkpoint.StorageBudget.Reservations[key] = entry
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err = m.ReserveWorkerIntegration(root.ID, root.Actor, child, "integrate"); err == nil {
				t.Fatal("invalid candidate admitted")
			}
			current, _ := m.GetTask(root.ID)
			if current.Checkpoint.StorageBudget.ChargedBytes != 220 {
				t.Fatal("rejection charged storage")
			}
		})
	}
}

func TestWorkerIntegrationUnknownUsageNeverRefundsAndChargesOverrun(t *testing.T) {
	m, root, child := integrationBudgetFixture(t)
	req, _, err := m.ReserveWorkerIntegration(root.ID, root.Actor, child, "integrate")
	if err != nil {
		t.Fatal(err)
	}
	current, _ := m.GetTask(root.ID)
	var record execution.WorkingCopyRecord
	for _, entry := range current.Checkpoint.StorageBudget.Reservations {
		if entry.Integration != nil {
			record = execution.WorkingCopyRecord{ID: entry.CopyID, Source: req.Source, WorkerRunID: child, Purpose: entry.Request.Purpose, State: "ready", UsageMeasured: true}
		}
	}
	for _, bytes := range []int64{150, 1100} {
		record.ReservedBytes, record.MeasuredBytes = bytes, bytes
		if err = m.RecordWorkerIntegration(root.ID, root.Actor, child, req, execution.WorkerArtifactReply{State: "uncertain", Copy: &record}); err != nil {
			t.Fatal(err)
		}
		current, _ = m.GetTask(root.ID)
		if current.Checkpoint.StorageBudget.ChargedBytes != max(int64(320), bytes)+100 {
			t.Fatal("unknown reservation refunded or overrun hidden")
		}
	}
	if _, _, err = m.ReserveWorkerIntegration(root.ID, root.Actor, child, "integrate"); err == nil {
		t.Fatal("overrun admitted new dispatch")
	}
}
