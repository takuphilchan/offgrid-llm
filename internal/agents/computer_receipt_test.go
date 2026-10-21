package agents

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func receiptFixture(t *testing.T, dir string) (*Manager, *Task) {
	t.Helper()
	m := NewManagerWithPersistence(nil, nil, nil, dir)
	r := NewRunner(m, &runTestTools{}, nil)
	task, err := r.Create("Insert exact text", "model", "alice", AgentConfig{MaxIterations: 3, TimeoutPerStep: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	task, err = m.updateTask(task.ID, func(task *Task) error {
		task.Status = TaskRunning
		task.Config.ComputerSession = "session"
		task.Checkpoint = &Checkpoint{ExecutingCall: "call", Calls: []api.ToolCall{{ID: "call", Type: "function", Function: api.FunctionCall{Name: "computer_insert_text", Arguments: `{"text":"café 日本語"}`}}}, ExecutingCapability: &capabilities.Descriptor{Name: "computer_insert_text", Namespace: "computer", Source: "local-companion", Kind: capabilities.Computer, Revision: capabilities.NativeEffectsRevision}}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return m, task
}

func TestComputerReceiptRecoversLostAcknowledgmentAfterRestart(t *testing.T) {
	dir := t.TempDir()
	m, task := receiptFixture(t, dir)
	args := json.RawMessage(`{"text":"café 日本語"}`)
	for i := 0; i < 2; i++ {
		if err := m.RecordComputerReceipt(task.ID, "alice", "session", "call", "computer_insert_text", args, `{"status":"dispatched"}`); err != nil {
			t.Fatal(err)
		}
	}
	restarted := NewManagerWithPersistence(nil, nil, nil, dir)
	got, _ := restarted.GetTask(task.ID)
	if got.Status != TaskInterrupted || got.Checkpoint.ExecutingCall != "" || len(got.Steps) != 1 || got.Steps[0].CallID != "call" || got.Steps[0].Provenance != "companion_receipt" || !got.ComputerSessionExpired {
		t.Fatalf("not recovered safely: %+v", got)
	}
	if recoverComputerReceipt(got) || len(got.Steps) != 1 {
		t.Fatal("receipt applied twice")
	}
	if got.Result != "" {
		t.Fatal("invented final completion")
	}
}

func TestComputerReceiptRejectsMismatchesAndDoesNotInferSuccess(t *testing.T) {
	for _, change := range []string{"actor", "session", "call", "tool", "arguments", "revision", "remote", "conflict", "invalid-json"} {
		t.Run(change, func(t *testing.T) {
			m, task := receiptFixture(t, "")
			actor, session, call, tool, args, result := "alice", "session", "call", "computer_insert_text", `{"text":"café 日本語"}`, `{"status":"dispatched"}`
			switch change {
			case "actor":
				actor = "bob"
			case "session":
				session = "other"
			case "call":
				call = "other"
			case "tool":
				tool = "computer_replace_text"
			case "arguments":
				args = `{"text":"different"}`
			case "invalid-json":
				result = "invalid"
			case "revision", "remote":
				_, _ = m.updateTask(task.ID, func(t *Task) error {
					if change == "revision" {
						t.Checkpoint.ExecutingCapability.Revision = "future"
					} else {
						t.Checkpoint.ExecutingCapability.Source = "mcp"
					}
					return nil
				})
			case "conflict":
				if err := m.RecordComputerReceipt(task.ID, actor, session, call, tool, json.RawMessage(args), result); err != nil {
					t.Fatal(err)
				}
				result = `{"status":"different"}`
			}
			if err := m.RecordComputerReceipt(task.ID, actor, session, call, tool, json.RawMessage(args), result); err == nil {
				t.Fatal("accepted mismatched receipt")
			}
		})
	}
	for _, reason := range []string{"unavailable-target", "service-disconnect", "misleading-post-state"} {
		t.Run(reason, func(t *testing.T) {
			_, task := receiptFixture(t, "")
			task.Status = TaskUncertain
			task.Steps = append(task.Steps, Step{Type: "observation", ToolResult: `{"visible_text":"café 日本語","verified":true}`})
			if recoverComputerReceipt(task) || task.Status != TaskUncertain || task.Checkpoint.ExecutingCall != "call" {
				t.Fatal("inferred invocation outcome from unrelated screen")
			}
		})
	}
}

func TestComputerReceiptSurvivesWorkerSnapshotAndPause(t *testing.T) {
	m, task := receiptFixture(t, "")
	r := NewRunner(m, &runTestTools{}, nil)
	if err := m.RecordComputerReceipt(task.ID, "alice", "session", "call", "computer_insert_text", json.RawMessage(`{"text":"café 日本語"}`), `{"status":"dispatched"}`); err != nil {
		t.Fatal(err)
	}
	if err := r.persist(task); err != nil {
		t.Fatal(err)
	}
	saved, _ := m.GetTask(task.ID)
	if saved.Checkpoint.ComputerReceipt == nil {
		t.Fatal("worker erased durable receipt")
	}
	paused, err := r.Pause(task.ID, "alice", true)
	if err != nil {
		t.Fatal(err)
	}
	if !recoverComputerReceipt(paused) || paused.Status != TaskInterrupted || !paused.ComputerSessionExpired {
		t.Fatal("pause recovery resumed or restored consent")
	}
}
