package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestDelegationChecksImplementationBeforeGrant(t *testing.T) {
	for _, name := range []string{"calculator", "current_time", "read_file", "list_files", "http_get"} {
		t.Run(name, func(t *testing.T) {
			registry := NewToolRegistry()
			builtin, _ := registry.Capability(name)
			if !delegatedCapability(name, builtin) {
				t.Fatal("built-in unavailable to delegation")
			}
			for _, change := range []func(*capabilities.Descriptor){
				func(d *capabilities.Descriptor) { d.Source = "user" },
				func(d *capabilities.Descriptor) { d.Namespace = "mcp" },
				func(d *capabilities.Descriptor) { d.Kind = capabilities.Execute },
				func(d *capabilities.Descriptor) { d.Risk = capabilities.RiskHigh },
			} {
				d := builtin
				change(&d)
				if delegatedCapability(name, d) {
					t.Fatalf("accepted widened capability: %+v", d)
				}
			}
			// A configured replacement exists BEFORE the parent grants the tool.
			data, _ := json.Marshal(ToolsConfig{Tools: []UserDefinedTool{{Name: name, Type: "shell", Command: "must-not-execute"}}})
			path := filepath.Join(t.TempDir(), "tools.json")
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			if err := registry.LoadUserTools(path); err != nil {
				t.Fatal(err)
			}
			m := NewManager(nil, nil, nil)
			r := NewRunner(m, registry, nil)
			task, err := r.Create("Bounded analysis", "model", "alice", taskConfig())
			if err != nil {
				t.Fatal(err)
			}
			task, err = m.updateTask(task.ID, func(task *Task) error { task.Status = TaskRunning; return nil })
			if err != nil {
				t.Fatal(err)
			}
			args, _ := json.Marshal(map[string]any{"tasks": []ChildSpec{{Key: "one", Goal: "Read only", Tools: []string{name}}}})
			call := api.ToolCall{ID: "delegate", Type: "function", Function: api.FunctionCall{Name: "delegate_tasks", Arguments: string(args)}}
			if err := r.beginDelegation(task, call); err == nil {
				t.Fatal("replacement acquired child grant")
			}
			if len(m.ListTasks()) != 1 || task.Delegation != nil {
				t.Fatal("rejected graph partially persisted")
			}
			// A previously persisted unsafe grant cannot be executed either.
			d, _ := registry.Capability(name)
			task.ParentID = "historical-parent"
			task.Config.AllowedTools = []string{name}
			task.Config.AllowedCapabilities = map[string]capabilities.Descriptor{name: d}
			task.Checkpoint.Calls = []api.ToolCall{{ID: "unsafe", Type: "function", Function: api.FunctionCall{Name: name, Arguments: `{}`}}}
			result, err := r.execute(context.Background(), task, nil)
			if err != nil || result.Status != TaskFailed || result.Checkpoint.ExecutingCall != "" || result.PendingApproval != nil {
				t.Fatalf("restored unsafe grant was not denied: %+v %v", result, err)
			}
		})
	}
}

func TestCoordinatorCandidatesExcludeHistoryWithoutCopyingPayloads(t *testing.T) {
	m := NewManager(nil, nil, nil)
	payload := strings.Repeat("history", 1<<18)
	for i := 0; i < 100; i++ {
		id := fmt.Sprintf("finished-%d", i)
		m.tasks[id] = &Task{ID: id, Status: TaskCompleted, Checkpoint: &Checkpoint{Messages: []api.ChatMessage{{Role: "tool", Content: payload}}}, ContextArchive: []ArchivedContext{{ID: "archive", Messages: []api.ChatMessage{{Role: "tool", Content: payload}}}}}
	}
	if allocations := testing.AllocsPerRun(20, func() {
		if len(m.coordinationCandidates()) != 0 {
			t.Fatal("scheduled history")
		}
	}); allocations != 0 {
		t.Fatalf("idle history allocated %.0f objects", allocations)
	}
	now := time.Now()
	m.tasks["deleted"] = &Task{ID: "deleted", Status: TaskChildren, DeletedAt: &now}
	m.tasks["waiting"] = &Task{ID: "waiting", Status: TaskPending}
	m.tasks["ready"] = &Task{ID: "ready", Actor: "alice", Status: TaskPending, SchedulerReady: true}
	m.tasks["parent"] = &Task{ID: "parent", Actor: "bob", Status: TaskChildren}
	candidates := m.coordinationCandidates()
	if len(candidates) != 2 {
		t.Fatalf("wrong candidates: %+v", candidates)
	}
	for _, c := range candidates {
		if c.ID != "ready" && c.ID != "parent" {
			t.Fatalf("ineligible: %+v", c)
		}
	}
	candidates[0].Actor = "changed"
	if m.tasks[candidates[0].ID].Actor == "changed" {
		t.Fatal("candidate aliases stored state")
	}
}

func TestSteeringEligibilityUsesActiveDelegationNotHistory(t *testing.T) {
	m := NewManager(nil, nil, nil)
	r := NewRunner(m, NewToolRegistry(), nil)
	task, _ := r.Create("Parent", "model", "alice", taskConfig())
	task, err := m.updateTask(task.ID, func(t *Task) error {
		t.Status = TaskInterrupted
		t.ChildHistory = []ChildLink{{ID: "completed-child", Spec: ChildSpec{Key: "past", Goal: "Already done"}}}
		return nil
	})
	if err != nil || !CanSteerTask(task) {
		t.Fatal("completed child history blocks steering")
	}
	if _, err := r.Steer(task.ID, "alice", "follow-up-1", "Summarize the completed findings"); err != nil {
		t.Fatal(err)
	}
	task, _ = m.updateTask(task.ID, func(t *Task) error { t.Delegation = &Delegation{CallID: "active"}; return nil })
	if CanSteerTask(task) {
		t.Fatal("active delegation is steerable")
	}
	if _, err := r.Steer(task.ID, "alice", "follow-up-2", "Discard active work"); err == nil {
		t.Fatal("active graph discarded")
	}
	task.Delegation = nil
	task.Checkpoint.ExecutingCall = "uncertain"
	if CanSteerTask(task) {
		t.Fatal("uncertain call is steerable")
	}
}
