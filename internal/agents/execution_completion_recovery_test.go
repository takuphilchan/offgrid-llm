package agents

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
	"testing"
)

type pendingObservationTools struct {
	*discoveryTools
	revoked        bool
	authorizations int
}

func (p *pendingObservationTools) Authorize(ctx context.Context, name string, args json.RawMessage, policy ToolExecution) error {
	p.authorizations++
	if p.revoked {
		return &ToolAuthorizationError{Code: "execution_grant_revoked", Message: "Fixture grant was revoked before observation."}
	}
	return p.discoveryTools.Authorize(ctx, name, args, policy)
}

func TestPendingProcessObservationRechecksCancellationAndRevocationAfterPersistence(t *testing.T) {
	for _, kind := range []string{"cancelled", "capability-revoked", "grant-revoked"} {
		t.Run(kind, func(t *testing.T) {
			m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
			base := newDiscoveryTools(0)
			d, _ := execution.Descriptor("workspace_process_wait")
			base.descriptors[d.Name] = d
			tools := &pendingObservationTools{discoveryTools: base}
			r := NewRunner(m, tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
				return nil, errors.New("fixture stops after rejected observation")
			})
			cfg := taskConfig()
			ref := execution.Reference{Host: "fixture", Session: "session", Grant: "grant", Revision: 1}
			cfg.ExecutionRequestVersion, cfg.Execution = 1, &ref
			cfg.ExecutionWorkspace = &execution.Resource{Reference: ref, Backend: "local", Root: "/fixture", Directory: ".", Permissions: execution.Permissions{Read: true, Processes: true}}
			task, err := r.Create("Observe the existing command", "model", "alice", cfg)
			if err != nil {
				t.Fatal(err)
			}
			task, err = m.updateTask(task.ID, func(next *Task) error { next.Status = TaskRunning; return nil })
			if err != nil {
				t.Fatal(err)
			}
			if _, err = r.repairExecutionCompletion(task, &ExecutionPending{Handle: "owned", Reference: ref}); err != nil {
				t.Fatal(err)
			}
			saved, _ := m.GetTask(task.ID)
			if len(saved.Checkpoint.Calls) != 1 {
				t.Fatal("no persisted observation")
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch kind {
			case "cancelled":
				cancel()
			case "capability-revoked":
				base.hidden[d.Name] = true
			case "grant-revoked":
				tools.revoked = true
			}
			result, runErr := r.execute(ctx, saved, nil)
			if base.calls.Load() != 0 || result != nil && result.Status == TaskCompleted {
				t.Fatal("scheduled observation bypassed live boundary", result, runErr)
			}
			if kind == "cancelled" && !errors.Is(runErr, context.Canceled) {
				t.Fatal("cancellation ignored", runErr)
			}
			if kind == "grant-revoked" && tools.authorizations != 1 {
				t.Fatal("host authority was not rechecked", tools.authorizations)
			}
		})
	}
}

func TestPendingProcessCompletionPersistsOnlyAuthorizedSameSessionObservation(t *testing.T) {
	for _, kind := range []string{"authorized", "foreign", "unavailable", "profile-denied"} {
		t.Run(kind, func(t *testing.T) {
			m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
			tools := newDiscoveryTools(0)
			const name = "workspace_process_wait"
			d, _ := execution.Descriptor(name)
			tools.descriptors[name] = d
			if kind == "unavailable" {
				tools.hidden[name] = true
			}
			r := NewRunner(m, tools, nil)
			cfg := taskConfig()
			ref := execution.Reference{Host: "fixture", Session: "session", Grant: "grant", Revision: 1}
			cfg.ExecutionRequestVersion, cfg.Execution = 1, &ref
			cfg.ExecutionWorkspace = &execution.Resource{Reference: ref, Backend: "local", Root: "/fixture", Directory: ".", Permissions: execution.Permissions{Read: true, Processes: true}}
			if kind == "profile-denied" {
				cfg.Profile = &AgentProfile{}
			}
			task, err := r.Create("Wait for the existing command", "model", "alice", cfg)
			if err != nil {
				t.Fatal(err)
			}
			task, err = m.updateTask(task.ID, func(next *Task) error { next.Status = TaskRunning; return nil })
			if err != nil {
				t.Fatal(err)
			}
			pending := &ExecutionPending{Handle: "owned", Reference: ref}
			if kind == "foreign" {
				pending.Reference.Session = "foreign"
			}
			if handled, err := r.repairExecutionCompletion(task, pending); !handled || err != nil {
				t.Fatal(handled, err)
			}
			saved, _ := m.GetTask(task.ID)
			if kind != "authorized" {
				if len(saved.Checkpoint.Calls) != 0 {
					t.Fatal("unauthorized observation scheduled")
				}
				return
			}
			if len(saved.Checkpoint.Calls) != 1 || saved.Checkpoint.Calls[0].Function.Name != name || saved.Result != "" || saved.Checkpoint.ExecutingCall != "" || tools.calls.Load() != 0 {
				t.Fatal("observation bypassed durable dispatch")
			}
			var args struct {
				Handle string `json:"handle"`
				Wait   int    `json:"wait_ms"`
			}
			if json.Unmarshal([]byte(saved.Checkpoint.Calls[0].Function.Arguments), &args) != nil || args.Handle != "owned" || args.Wait != 1000 {
				t.Fatal("unbounded or wrong observation")
			}
			reloaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
			restored, ok := reloaded.GetTask(task.ID)
			if !ok || restored.Checkpoint.Calls[0].ID != saved.Checkpoint.Calls[0].ID {
				t.Fatal("observation identity lost on restart")
			}
		})
	}
}

func TestPendingProcessCompletionHasBoundedDurableObservationRepair(t *testing.T) {
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	r := NewRunner(m, nil, nil)
	cfg := DefaultAgentConfig()
	cfg.TaskFirst = true
	cfg.ExecutionRequestVersion = 1
	task, err := r.Create("Wait for the existing command", "model", "alice", cfg)
	if err != nil {
		t.Fatal(err)
	}
	task, err = m.updateTask(task.ID, func(next *Task) error { next.Status = TaskRunning; return nil })
	if err != nil {
		t.Fatal(err)
	}
	if handled, err := r.repairExecutionCompletion(task, errors.New("unverified mutation")); handled || err != nil {
		t.Fatal("generic failure was retried")
	}
	for i := 1; i <= 4; i++ {
		handled, err := r.repairExecutionCompletion(task, &ExecutionPending{})
		if !handled || err != nil {
			t.Fatal(i, handled, err)
		}
		saved, _ := m.GetTask(task.ID)
		if saved.Checkpoint.WorkspaceCompletionRepairs != i || len(saved.Checkpoint.Calls) != 0 || saved.Result != "" {
			t.Fatal("repair lost budget, dispatched or completed")
		}
		if i <= 3 && saved.Status != TaskRunning {
			t.Fatal("did not permit observation")
		}
		if i == 4 && (saved.Status != TaskFailed || saved.ErrorCode != "execution_completion_repair_limit") {
			t.Fatal("unbounded completion repair", saved.Status)
		}
	}
}

func TestPendingProcessCompletionDoesNotChangeLegacyOrInFlightWork(t *testing.T) {
	for _, kind := range []string{"legacy", "in-flight"} {
		task := &Task{Config: AgentConfig{TaskFirst: true, ExecutionRequestVersion: 1}, Checkpoint: &Checkpoint{}}
		if kind == "legacy" {
			task.Config.ExecutionRequestVersion = 0
		} else {
			task.Checkpoint.ExecutingCall = "uncertain-call"
		}
		var runner *Runner
		if handled, err := runner.repairExecutionCompletion(task, &ExecutionPending{}); handled || err != nil {
			t.Fatal("unsafe boundary repaired", kind)
		}
	}
}
