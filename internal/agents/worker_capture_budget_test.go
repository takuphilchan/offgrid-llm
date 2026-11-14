package agents

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
)

func captureBudgetFixture(t *testing.T) (*Manager, *Task, string, execution.SealChangeSetRequest) {
	t.Helper()
	m, _, root, copyReq := storageBudgetFixture(t)
	if err := m.ReserveWorkerStorage(root.ID, root.Actor, copyReq, 300); err != nil {
		t.Fatal(err)
	}
	record := execution.WorkingCopyRecord{ID: storageReservationKey(copyReq), State: "ready", Source: copyReq.Parent, WorkerRunID: copyReq.WorkerRunID, ReservedBytes: 100, MeasuredBytes: 30, UsageMeasured: true}
	if err := m.RecordWorkerStorage(root.ID, root.Actor, copyReq, record); err != nil {
		t.Fatal(err)
	}
	resource := execution.Resource{Reference: execution.Reference{Host: copyReq.Parent.Host, Session: "child-session", Grant: "child-grant", Revision: 1}, Backend: "local", Root: "private-copy", Directory: ".", Permissions: copyReq.Permissions}
	if err := m.BindWorkerEnvironment(root.ID, root.Actor, copyReq, resource); err != nil {
		t.Fatal(err)
	}
	if _, err := m.updateTask(copyReq.WorkerRunID, func(child *Task) error { finishTask(child, TaskCompleted, "Result"); return nil }); err != nil {
		t.Fatal(err)
	}
	req, fresh, err := m.ReserveWorkerCapture(root.ID, root.Actor, copyReq.WorkerRunID)
	if err != nil || !fresh || req.MaxReservedBytes != 200 {
		t.Fatal(req, fresh, err)
	}
	return m, root, copyReq.WorkerRunID, req
}

func TestWorkerCaptureConcurrentAdmissionHasOneDispatchOwner(t *testing.T) {
	m, root, child, _ := captureBudgetFixture(t)
	if _, err := m.updateTask(root.ID, func(task *Task) error {
		budget := task.Checkpoint.StorageBudget
		for key, reservation := range budget.Reservations {
			budget.ChargedBytes -= reservation.ChargedBytes - reservation.Capture.BaseBytes
			reservation.ChargedBytes = reservation.Capture.BaseBytes
			reservation.Capture = nil
			budget.Reservations[key] = reservation
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var freshCount atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, fresh, err := m.ReserveWorkerCapture(root.ID, root.Actor, child)
			if err != nil {
				t.Error(err)
			}
			if fresh {
				freshCount.Add(1)
			}
		}()
	}
	wg.Wait()
	current, _ := m.GetTask(root.ID)
	if freshCount.Load() != 1 || current.Checkpoint.StorageBudget.ChargedBytes != 200 {
		t.Fatal("duplicate capture dispatch/allocation", freshCount.Load())
	}
}

func sealedBudgetReply(req execution.SealChangeSetRequest, child string) execution.WorkerArtifactReply {
	identity, _ := json.Marshal(struct {
		Source            execution.Binding
		CopyID, RequestID string
	}{req.Source, req.CopyID, req.RequestID})
	hash := sha256.Sum256(identity)
	return execution.WorkerArtifactReply{State: "sealed", ChangeSet: &execution.ChangeSet{Version: 1, ID: hex.EncodeToString(hash[:]), CopyID: req.CopyID, Source: req.Source, WorkerRunID: child, InstructionVersion: req.InstructionVersion, ControlEpoch: req.ControlEpoch, State: "sealed", OperationsDigest: strings.Repeat("a", 64), OperationCount: 1}, Copy: &execution.WorkingCopyRecord{ID: req.CopyID, State: "ready", Source: req.Source, WorkerRunID: child, ReservedBytes: 120, MeasuredBytes: 80, UsageMeasured: true}}
}

func TestWorkerCaptureBudgetRetryUnknownRestartAndSettlement(t *testing.T) {
	m, root, child, req := captureBudgetFixture(t)
	again, fresh, err := m.ReserveWorkerCapture(root.ID, root.Actor, child)
	if err != nil || fresh || again != req {
		t.Fatal("capture was reserved twice", again, fresh, err)
	}
	if err = m.RecordWorkerCapture(root.ID, root.Actor, child, req, execution.WorkerArtifactReply{State: "uncertain"}); err != nil {
		t.Fatal(err)
	}
	restored := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err = restored.StorageError(); err != nil {
		t.Fatal(err)
	}
	unknown, _ := restored.GetTask(root.ID)
	if unknown.Checkpoint.StorageBudget.ChargedBytes != 200 {
		t.Fatal("unknown restart refunded capture")
	}
	// A late authenticated sealed receipt can settle actual usage even after
	// interruption, without changing the root's outcome or dispatching a run.
	reply := sealedBudgetReply(req, child)
	if err = restored.RecordWorkerCapture(root.ID, root.Actor, child, req, reply); err != nil {
		t.Fatal(err)
	}
	if err = restored.RecordWorkerCapture(root.ID, root.Actor, child, req, reply); err != nil {
		t.Fatal("retry", err)
	}
	settled, _ := restored.GetTask(root.ID)
	if settled.Status != unknown.Status || settled.Checkpoint.StorageBudget.ChargedBytes != 120 || workerCapturedOutput(settled, child).ChangeSetID != reply.ChangeSet.ID {
		t.Fatal("settlement revived work or lost measured refund")
	}
}

func TestWorkerCaptureBudgetRejectsForgedOrUnmeasuredReceipts(t *testing.T) {
	for _, kind := range []string{"actor", "identity", "version", "unmeasured", "overrun", "instruction"} {
		t.Run(kind, func(t *testing.T) {
			m, root, child, req := captureBudgetFixture(t)
			reply := sealedBudgetReply(req, child)
			actor := root.Actor
			switch kind {
			case "actor":
				actor = "foreign"
			case "identity":
				reply.ChangeSet.ID = strings.Repeat("b", 64)
			case "version":
				reply.ChangeSet.Version = 2
			case "unmeasured":
				reply.Copy.UsageMeasured = false
			case "overrun":
				reply.Copy.ReservedBytes = 201
			case "instruction":
				reply.ChangeSet.InstructionVersion++
			}
			if err := m.RecordWorkerCapture(root.ID, actor, child, req, reply); err == nil {
				t.Fatal("invalid capture receipt accepted")
			}
			current, _ := m.GetTask(root.ID)
			if current.Checkpoint.StorageBudget.ChargedBytes != 200 || workerCapturedOutput(current, child) != nil {
				t.Fatal("invalid receipt changed accounting")
			}
		})
	}
}
