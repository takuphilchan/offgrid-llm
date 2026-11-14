package agents

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
)

func TestExecutionWorkerCapabilityRequiresExactBindingAndMode(t *testing.T) {
	d, _ := execution.Descriptor("workspace_write")
	ref := execution.Reference{Host: "host", Session: "child", Grant: "grant", Revision: 1}
	task := &Task{ParentID: "parent", Worker: &WorkerBinding{SessionID: "worker"}, Config: AgentConfig{ExecutionWorkerMode: "edit", Execution: &ref, ExecutionWorkspace: &execution.Resource{Reference: ref}}, Checkpoint: &Checkpoint{WorkerEnvironment: &WorkerEnvironment{ParentRunID: "parent", RequestID: "prepare", CopyID: strings.Repeat("a", 64)}}}
	if !workerCapabilityAllowed(task, d.Name, d) {
		t.Fatal("bound edit tool unavailable")
	}
	for _, kind := range []string{"unbound", "test", "foreign-parent", "provider"} {
		t.Run(kind, func(t *testing.T) {
			copy, _ := copyTask(task)
			descriptor := d
			switch kind {
			case "unbound":
				copy.Checkpoint.WorkerEnvironment = nil
			case "test":
				copy.Config.ExecutionWorkerMode = "test"
			case "foreign-parent":
				copy.Checkpoint.WorkerEnvironment.ParentRunID = "other"
			case "provider":
				descriptor.Source = "mcp:forged"
			}
			if workerCapabilityAllowed(copy, d.Name, descriptor) {
				t.Fatal("unauthorized child write")
			}
		})
	}
}

func TestExecutionAssignmentNeedsHostOptInAndKeepsProfileCeilings(t *testing.T) {
	parent := &Task{WorkID: "work", Origin: &ConversationOrigin{Workspace: "workspace"}, Config: AgentConfig{Execution: &execution.Reference{}, ExecutionWorkspace: &execution.Resource{Backend: "local", PreparedWorkers: true, Permissions: execution.Permissions{Read: true, Write: true}}}}
	spec := ChildSpec{Mode: "edit", Tools: []string{"workspace_write"}}
	if err := validateExecutionAssignment(parent, spec); err != nil {
		t.Fatal(err)
	}
	parent.Config.ExecutionWorkspace.PreparedWorkers = false
	if err := validateExecutionAssignment(parent, spec); err == nil {
		t.Fatal("host opt-in bypassed")
	}
	parent.Config.ExecutionWorkspace.PreparedWorkers = true
	parent.Config.ExecutionPolicy = &execution.Permissions{Read: true}
	if err := validateExecutionAssignment(parent, spec); err == nil {
		t.Fatal("conversation write restriction bypassed")
	}
	parent.Config.ExecutionPolicy = nil
	parent.Config.ExecutionWorkspace.Permissions.Write = false
	if err := validateExecutionAssignment(parent, ChildSpec{Mode: "test", Tools: []string{"workspace_process_start"}}); err == nil {
		t.Fatal("native process bypassed write restriction")
	}
	parent.Config.ExecutionWorkspace.Permissions.Write = true
	spec.Tools = []string{"read_file"}
	if err := validateExecutionAssignment(parent, spec); err == nil {
		t.Fatal("service filesystem offered to workspace worker")
	}
}

func TestCanonicalTemporaryDelegationSchemaIncludesOnlyAvailableModes(t *testing.T) {
	d, _ := execution.Descriptor("workspace_write")
	task := &Task{WorkID: "work", Config: AgentConfig{ScopedCapabilities: true, AllowedCapabilities: map[string]capabilities.Descriptor{d.Name: d}, Execution: &execution.Reference{}, ExecutionWorkspace: &execution.Resource{Backend: "local", PreparedWorkers: true}}}
	tool := profileDelegationTool(task)
	if !strings.Contains(tool.Function.Description, "independent working copies") {
		t.Fatal("temporary worker discovery missing")
	}
	task.Config.ExecutionWorkspace.PreparedWorkers = false
	tool = profileDelegationTool(task)
	item := tool.Function.Parameters["properties"].(map[string]any)["tasks"].(map[string]any)["items"].(map[string]any)
	modes := item["properties"].(map[string]any)["mode"].(map[string]any)["enum"].([]string)
	if len(modes) != 1 || modes[0] != "research" {
		t.Fatal("unavailable execution modes offered", modes)
	}
}

type slowAssignmentPreparation struct {
	RunTools
	entered, exited chan struct{}
	calls           atomic.Int32
}

func (s *slowAssignmentPreparation) PrepareExecutionAssignment(ctx context.Context, _, _ *Task, _ ChildSpec) (bool, error) {
	s.calls.Add(1)
	close(s.entered)
	<-ctx.Done()
	close(s.exited)
	return false, ctx.Err()
}

func TestExecutionPreparationDoesNotBlockForegroundCoordinatorOrShutdown(t *testing.T) {
	m, r, _, root, child := workerFixture(t)
	if _, err := m.updateTask(child.ID, func(task *Task) error { task.Config.ExecutionWorkerMode = "edit"; return nil }); err != nil {
		t.Fatal(err)
	}
	slow := &slowAssignmentPreparation{RunTools: r.tools, entered: make(chan struct{}), exited: make(chan struct{})}
	r.tools = slow
	foreground := make(chan struct{})
	r.Coordinate = func(context.Context, time.Time) error { close(foreground); return nil }
	r.coordinateWorkerPreparations(context.Background())
	select {
	case <-slow.entered:
	case <-time.After(time.Second):
		t.Fatal("preparation not admitted")
	}
	for i := 0; i < 5; i++ {
		r.coordinateWorkerPreparations(context.Background())
	}
	r.coordinateOccurrences(context.Background(), time.Now())
	select {
	case <-foreground:
	case <-time.After(time.Second):
		t.Fatal("host blocked foreground input")
	}
	r.mu.Lock()
	active := len(r.active)
	r.mu.Unlock()
	if active != 0 {
		t.Fatal("preparation occupied generation slot")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := r.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case <-slow.exited:
	default:
		t.Fatal("shutdown did not drain preparation")
	}
	if slow.calls.Load() != 1 {
		t.Fatal("preparation overlapped")
	}
	saved, _ := m.GetTask(root.ID)
	if saved.Status != TaskChildren {
		t.Fatal("observation restarted parent")
	}
}
