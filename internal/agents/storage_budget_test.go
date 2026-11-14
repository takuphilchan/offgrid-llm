package agents

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func storageBudgetFixture(t *testing.T) (*Manager, *Runner, *Task, execution.WorkingCopyRequest) {
	t.Helper()
	m, r, scope, root, child := workerFixture(t)
	t.Cleanup(func() { r.Shutdown(context.Background()) })
	ref := execution.Reference{Host: "host", Session: "session", Grant: "grant", Revision: 1}
	root, err := m.updateTask(root.ID, func(task *Task) error {
		task.Config.Execution = &ref
		task.Config.ExecutionWorkspace = &execution.Resource{Reference: ref, Backend: "local", Root: "fixture", Directory: ".", Permissions: execution.Permissions{Read: true, Write: true}}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	req := execution.WorkingCopyRequest{RequestID: "prepare", Parent: execution.Binding{Actor: root.Actor, Workspace: scope.Workspace, Host: ref.Host, Session: ref.Session, Grant: ref.Grant, Revision: 1, Job: root.ID}, WorkerRunID: child.ID, InstructionVersion: root.InstructionVersion, ControlEpoch: root.ControlEpoch, Permissions: execution.Permissions{Read: true, Write: true}, MaxBytes: 100}
	return m, r, root, req
}

func TestWorkerStorageReservationRecoveryAndRelease(t *testing.T) {
	m, _, root, req := storageBudgetFixture(t)
	if err := m.ReserveWorkerStorage(root.ID, root.Actor, req, 150); err != nil {
		t.Fatal(err)
	}
	if err := m.ReserveWorkerStorage(root.ID, root.Actor, req, 150); err != nil {
		t.Fatal("retry", err)
	}
	changed := req
	changed.MaxBytes++
	if err := m.ReserveWorkerStorage(root.ID, root.Actor, changed, 150); err == nil {
		t.Fatal("changed retry accepted")
	}
	loaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err := loaded.StorageError(); err != nil {
		t.Fatal(err)
	}
	saved, _ := loaded.GetTask(root.ID)
	b := saved.Checkpoint.StorageBudget
	if b == nil || b.ChargedBytes != 100 || len(b.Reservations) != 1 || b.Reservations[storageReservationKey(req)].UsageMeasured {
		t.Fatal("unknown preparation refunded on restart", b)
	}
	record := execution.WorkingCopyRecord{ID: storageReservationKey(req), State: "ready", Source: req.Parent, WorkerRunID: req.WorkerRunID, ReservedBytes: 100, MeasuredBytes: 20, UsageMeasured: true}
	if err := m.RecordWorkerStorage(root.ID, root.Actor, req, record); err != nil {
		t.Fatal(err)
	}
	saved, _ = m.GetTask(root.ID)
	if saved.Checkpoint.StorageBudget.ChargedBytes != 100 {
		t.Fatal("live reserved capacity refunded")
	}
	record.ReservedBytes, record.MeasuredBytes = 180, 180
	if err := m.RecordWorkerStorage(root.ID, root.Actor, req, record); err != nil {
		t.Fatal("observed overrun was hidden", err)
	}
	saved, _ = m.GetTask(root.ID)
	if saved.Checkpoint.StorageBudget.ChargedBytes != 180 {
		t.Fatal("growth not charged")
	}
	record.State, record.ReservedBytes = "released", 0
	if err := m.RecordWorkerStorage(root.ID, root.Actor, req, record); err != nil {
		t.Fatal(err)
	}
	if err := m.RecordWorkerStorage(root.ID, root.Actor, req, record); err != nil {
		t.Fatal("release retry", err)
	}
	saved, _ = m.GetTask(root.ID)
	if saved.Checkpoint.StorageBudget.ChargedBytes != 0 {
		t.Fatal("confirmed cleanup not refunded")
	}
	record.State = "ready"
	if err := m.RecordWorkerStorage(root.ID, root.Actor, req, record); err == nil {
		t.Fatal("released receipt rolled backwards")
	}
}

func TestWorkerStorageRejectsForeignStaleAndUncommittedAdmission(t *testing.T) {
	m, _, root, req := storageBudgetFixture(t)
	for _, mutate := range []func(*execution.WorkingCopyRequest){func(r *execution.WorkingCopyRequest) { r.Parent.Actor = "foreign" }, func(r *execution.WorkingCopyRequest) { r.WorkerRunID = "foreign" }, func(r *execution.WorkingCopyRequest) { r.ControlEpoch++ }, func(r *execution.WorkingCopyRequest) { r.Permissions.Processes = true }, func(r *execution.WorkingCopyRequest) { r.MaxBytes = 151 }} {
		bad := req
		mutate(&bad)
		if err := m.ReserveWorkerStorage(root.ID, root.Actor, bad, 150); err == nil {
			t.Fatal("invalid reservation accepted", bad)
		}
	}
	db, err := m.controlDatabase()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TRIGGER storage_failure BEFORE UPDATE ON agent_tasks BEGIN SELECT RAISE(FAIL,'fixture'); END`); err != nil {
		t.Fatal(err)
	}
	if err = m.ReserveWorkerStorage(root.ID, root.Actor, req, 150); err == nil {
		t.Fatal("reservation acknowledged before commit")
	}
	saved, _ := m.GetTask(root.ID)
	if saved.Checkpoint.StorageBudget != nil {
		t.Fatal("failed write published allowance")
	}
	if _, err = db.Exec(`DROP TRIGGER storage_failure`); err != nil {
		t.Fatal(err)
	}
	// Storage failure intentionally latches this manager closed to dispatch.
	// Verify persisted recovery with a fresh manager, without clearing its latch.
	m = NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err = m.StorageError(); err != nil {
		t.Fatal(err)
	}
	if _, err = m.updateTask(root.ID, func(task *Task) error { task.Status = TaskChildren; return nil }); err != nil {
		t.Fatal(err)
	}
	if _, err = m.updateTask(req.WorkerRunID, func(task *Task) error { task.Status = TaskPending; return nil }); err != nil {
		t.Fatal(err)
	}
	if err = m.ReserveWorkerStorage(root.ID, root.Actor, req, 150); err != nil {
		t.Fatal(err)
	}
	record := execution.WorkingCopyRecord{ID: strings.Repeat("a", 64), State: "released", Source: req.Parent, WorkerRunID: req.WorkerRunID}
	if err = m.RecordWorkerStorage(root.ID, root.Actor, req, record); err == nil {
		t.Fatal("foreign copy freed reservation")
	}
	if _, err = m.updateTask(root.ID, func(t *Task) error { t.ControlAction = "stop"; return nil }); err != nil {
		t.Fatal(err)
	}
	if err = m.ReserveWorkerStorage(root.ID, root.Actor, req, 150); err == nil {
		t.Fatal("retry bypassed accepted Stop")
	}
}

func TestWorkerStorageStaleModelAndSchemaUpgradeKeepCharges(t *testing.T) {
	m, r, root, req := storageBudgetFixture(t)
	stale, _ := m.GetTask(root.ID)
	if err := m.ReserveWorkerStorage(root.ID, root.Actor, req, 150); err != nil {
		t.Fatal(err)
	}
	if _, err := m.updateTask(root.ID, func(task *Task) error { task.Status = TaskRunning; return nil }); err != nil {
		t.Fatal(err)
	}
	stale.Status = TaskRunning
	if err := r.persist(stale); err != nil {
		t.Fatal(err)
	}
	saved, _ := m.GetTask(root.ID)
	if saved.Checkpoint.StorageBudget == nil || saved.Checkpoint.StorageBudget.ChargedBytes != 100 {
		t.Fatal("model checkpoint overwrote service accounting")
	}
	db, err := m.controlDatabase()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE agent_schema SET version=13`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	loaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err = loaded.StorageError(); err != nil {
		t.Fatal(err)
	}
	restored, _ := loaded.GetTask(root.ID)
	if !reflect.DeepEqual(restored.Checkpoint.StorageBudget, saved.Checkpoint.StorageBudget) {
		t.Fatal("schema upgrade discarded unknown charges")
	}
}

func TestWorkerStorageContinuationKeepsReservations(t *testing.T) {
	m, r, scope, root, child := completedWorkerWork(t)
	root, err := m.updateTask(root.ID, func(saved *Task) error {
		saved.Checkpoint.StorageBudget = &StorageBudget{Version: 1, LimitBytes: 150, ChargedBytes: 100, Reservations: map[string]StorageReservation{"unknown": {State: "reserved", ChargedBytes: 100}}}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	worker, _ := m.WorkerSession(scope, child.Worker.SessionID)
	accepted, err := r.CommandWorker(scope, api.WorkerCommand{RequestID: "storage-followup", TargetID: worker.ID, ExpectedRevision: worker.Revision, Kind: "continue", Payload: api.WorkerCommandPayload{Content: "Follow up"}})
	if err != nil {
		t.Fatal(err)
	}
	next, _ := m.GetTask(accepted.Worker.RootRunID)
	if !reflect.DeepEqual(root.Checkpoint.StorageBudget, next.Checkpoint.StorageBudget) {
		t.Fatal("new attempt reopened storage capacity")
	}
}

func TestWorkerStorageSharesCapacityAcrossBranches(t *testing.T) {
	m, _, root, req := storageBudgetFixture(t)
	child, _ := m.GetTask(req.WorkerRunID)
	child.ID += "-second"
	child.Worker.SessionID += "-second"
	root.ChildHistory = append(root.ChildHistory, ChildLink{ID: child.ID, WorkerID: child.Worker.SessionID, Spec: ChildSpec{Key: "second", Goal: "Independent branch"}, Status: TaskPending})
	root.Delegation.Children = append(root.Delegation.Children, root.ChildHistory[len(root.ChildHistory)-1])
	m.mu.Lock()
	err := m.saveTaskBatch([]*Task{root, child})
	if err == nil {
		m.tasks[root.ID], m.tasks[child.ID] = root, child
	}
	m.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if err = m.ReserveWorkerStorage(root.ID, root.Actor, req, 150); err != nil {
		t.Fatal(err)
	}
	second := req
	second.RequestID, second.WorkerRunID = "second", child.ID
	if err = m.ReserveWorkerStorage(root.ID, root.Actor, second, 150); err == nil {
		t.Fatal("branches exceeded root capacity")
	}
	record := execution.WorkingCopyRecord{ID: storageReservationKey(req), State: "ready", Source: req.Parent, WorkerRunID: req.WorkerRunID, ReservedBytes: 100, UsageMeasured: true}
	if err = m.RecordWorkerStorage(root.ID, root.Actor, req, record); err != nil {
		t.Fatal(err)
	}
	current, _ := m.GetTask(root.ID)
	if !HasOwnedExecutionProcesses(current) {
		t.Fatal("preparation omitted from Stop cleanup")
	}
	record.State, record.ReservedBytes = "released", 0
	if err = m.RecordWorkerStorage(root.ID, root.Actor, req, record); err != nil {
		t.Fatal(err)
	}
	if err = m.ReserveWorkerStorage(root.ID, root.Actor, second, 150); err != nil {
		t.Fatal("confirmed cleanup did not restore unused storage", err)
	}
}
