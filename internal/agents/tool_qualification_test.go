package agents

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agentcontext"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func scopedAgentPin() *inference.AgentToolPin {
	return &inference.AgentToolPin{Adapter: inference.NativeAgentToolAdapter, Scope: models.NativeWorkspaceScope, CapabilitySHA256: NativeWorkspaceCapabilitySHA256(), EvidenceID: strings.Repeat("a", 64), FingerprintID: strings.Repeat("b", 64)}
}

func TestReviewedContractScopePreservesCapabilityBoundary(t *testing.T) {
	for _, scope := range []string{models.NativeWorkspaceScope, models.NativeWorkspaceContractScope} {
		t.Run(scope, func(t *testing.T) {
			pin := scopedAgentPin()
			pin.Scope = scope
			task := &Task{Config: taskConfig(), Checkpoint: &Checkpoint{}}
			task.Config.ToolAdapter = pin
			read, _ := execution.Descriptor("workspace_read")
			if !qualifiedTool(task, "workspace_read", read, true) {
				t.Fatal("reviewed read excluded")
			}
			foreign := read
			foreign.Source = "mcp:foreign"
			if qualifiedTool(task, "workspace_read", foreign, true) {
				t.Fatal("foreign tool inherited authority")
			}
			for _, name := range []string{"computer_observe", "delegate_tasks", "memory_write", "workspace_process_stop"} {
				if qualifiedTool(task, name, capabilities.Descriptor{}, true) {
					t.Fatal("scope widened", name)
				}
			}
		})
	}
}

func scopedDiscoveryFixture() *discoveryTools {
	d := newDiscoveryTools(1)
	for _, tool := range append(execution.Tools(), agentcontext.Tools()...) {
		name := tool.Function.Name
		desc, ok := execution.Descriptor(name)
		if !ok {
			desc, _ = agentcontext.Descriptor(name)
		}
		d.tools = append(d.tools, tool)
		d.descriptors[name] = desc
	}
	d.tools = append(d.tools, execution.AccessRequestSchema())
	d.descriptors[execution.AccessRequestTool] = execution.AccessRequestDescriptor()
	return d
}

func TestScopedQualificationFiltersMenusDiscoveryAndForgedIdentity(t *testing.T) {
	d := scopedDiscoveryFixture()
	r := NewRunner(nil, d, nil)
	task := &Task{Config: taskConfig(), Checkpoint: &Checkpoint{}}
	task.Config.ToolAdapter = scopedAgentPin()
	tools := r.authorizedTools(context.Background(), task)
	for _, tool := range tools {
		if !scopedToolName(tool.Function.Name) {
			t.Fatal("unqualified menu tool", tool.Function.Name)
		}
	}
	if len(tools) == 0 {
		t.Fatal("qualified tools disappeared")
	}
	projected := append(append([]api.Tool{}, tools...), api.Tool{Function: api.FunctionDef{Name: "computer_observe"}}, api.Tool{Function: api.FunctionDef{Name: "delegate_tasks"}})
	if got := r.qualifiedTools(task, projected); len(got) != len(tools) {
		t.Fatal("projection bypassed qualification")
	}
	for _, tool := range runtimeTools(task) {
		if !scopedToolName(tool.Function.Name) {
			t.Fatal("unqualified runtime tool", tool.Function.Name)
		}
	}
	for _, name := range []string{"memory_write", "skill_draft", "mcp_docs_000", "computer_observe"} {
		result, err := r.discoverTools(context.Background(), task, `{"query":"`+name+`"}`)
		if err != nil || strings.Contains(result, `"name":"`+name+`"`) {
			t.Fatal("unqualified discovery", result, err)
		}
	}
	for _, mutate := range []func(*capabilities.Descriptor){func(d *capabilities.Descriptor) { d.Source = "mcp:fake" }, func(d *capabilities.Descriptor) { d.Revision = "changed" }, func(d *capabilities.Descriptor) { d.Kind = capabilities.Execute }, func(d *capabilities.Descriptor) { d.Location = "remote" }} {
		original := d.descriptors["skill_load"]
		forged := original
		mutate(&forged)
		d.descriptors["skill_load"] = forged
		for _, tool := range r.authorizedTools(context.Background(), task) {
			if tool.Function.Name == "skill_load" {
				t.Fatal("forged capability accepted")
			}
		}
		d.descriptors["skill_load"] = original
	}
	for _, change := range []func(*inference.AgentToolPin){func(p *inference.AgentToolPin) { p.Scope = "unknown" }, func(p *inference.AgentToolPin) { p.CapabilitySHA256 = strings.Repeat("c", 64) }} {
		task.Config.ToolAdapter = scopedAgentPin()
		change(task.Config.ToolAdapter)
		if ValidateAgentToolScope(task.Config.ToolAdapter) == nil || len(r.authorizedTools(context.Background(), task)) != 0 || len(runtimeTools(task)) != 0 {
			t.Fatal("stale/unknown scope accepted")
		}
	}
	task.Config.ToolAdapter = nil
	if len(r.authorizedTools(context.Background(), task)) <= len(tools) {
		t.Fatal("legacy catalog narrowed")
	}
}

func TestScopedQualificationDoesNotGrantProfileAuthority(t *testing.T) {
	d := scopedDiscoveryFixture()
	r := NewRunner(nil, d, nil)
	task := &Task{Config: taskConfig(), Checkpoint: &Checkpoint{}}
	task.Config.ToolAdapter = scopedAgentPin()
	task.Config.Profile = &AgentProfile{}
	if got := r.authorizedTools(context.Background(), task); len(got) != 0 {
		t.Fatal("qualification granted profile resources", got)
	}
	desc, _ := execution.Descriptor("workspace_read")
	task.Config.AllowedCapabilities = map[string]capabilities.Descriptor{"workspace_read": desc}
	got := r.authorizedTools(context.Background(), task)
	if len(got) != 1 || got[0].Function.Name != "workspace_read" {
		t.Fatal("profile intersection changed", got)
	}
}

func TestNativeToolPresentationOrderIsStableWithoutCatalogMutation(t *testing.T) {
	d := scopedDiscoveryFixture()
	r := NewRunner(nil, d, nil)
	task := &Task{Config: taskConfig(), Checkpoint: &Checkpoint{}}
	task.Config.ToolAdapter = scopedAgentPin()
	var expected string
	for _, names := range [][]string{{"workspace_write", "task_plan", "workspace_read"}, {"workspace_read", "workspace_write", "task_plan"}, {"task_plan", "workspace_read", "workspace_write"}} {
		var catalog []api.Tool
		for _, name := range names {
			catalog = append(catalog, api.Tool{Type: "function", Function: api.FunctionDef{Name: name, Parameters: map[string]any{"type": "object"}}})
		}
		before, _ := json.Marshal(catalog)
		projected, _ := json.Marshal(r.qualifiedTools(task, catalog))
		if expected == "" {
			expected = string(projected)
		}
		if string(projected) != expected {
			t.Fatal("order depends on provider enumeration")
		}
		after, _ := json.Marshal(catalog)
		if string(before) != string(after) {
			t.Fatal("provider catalog mutated")
		}
		task.Config.ToolAdapter = nil
		legacy, _ := json.Marshal(r.qualifiedTools(task, catalog))
		if string(legacy) != string(before) {
			t.Fatal("legacy order changed")
		}
		task.Config.ToolAdapter = scopedAgentPin()
	}
}

func TestScopedQualificationRejectsPersistedCallsBeforeAnyDispatch(t *testing.T) {
	for _, name := range []string{"delegate_tasks", "task_expectations", "computer_observe", "memory_write", "mcp_docs_000", "skill_load"} {
		t.Run(name, func(t *testing.T) {
			m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
			d := scopedDiscoveryFixture()
			if name == "skill_load" {
				forged := d.descriptors[name]
				forged.Source = "mcp:forged"
				d.descriptors[name] = forged
			}
			authorizations := 0
			d.authorize = func(string, json.RawMessage, ToolExecution) error { authorizations++; return nil }
			r := NewRunner(m, d, nil)
			defer r.Shutdown(context.Background())
			cfg := taskConfig()
			cfg.ToolAdapter = scopedAgentPin()
			task, err := r.Create("fixture", "model", "alice", cfg)
			if err != nil {
				t.Fatal(err)
			}
			// Enter with an already persisted call, without asking a model to choose.
			task, err = m.updateTask(task.ID, func(saved *Task) error {
				saved.Status = TaskRunning
				saved.Checkpoint = &Checkpoint{Calls: []api.ToolCall{{ID: "saved", Type: "function", Function: api.FunctionCall{Name: name, Arguments: `{}`}}}}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			result, err := r.executeUntilBoundary(context.Background(), task, nil)
			if err != nil || result == nil || result.Status != TaskFailed || d.calls.Load() != 0 || authorizations != 0 || !strings.Contains(result.Error, "not qualified") {
				t.Fatal(result, err, authorizations, d.calls.Load())
			}
		})
	}
}
