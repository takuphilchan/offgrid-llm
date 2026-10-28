package agents

import (
	"context"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestLocalWorkspaceArtifactRequiresDiscoveryWithoutRemovingDownloads(t *testing.T) {
	d := newDiscoveryTools(0)
	desc := capabilities.Descriptor{Name: "task_save_artifact", Namespace: "workspace", Source: "workspace", Location: "service", Kind: capabilities.Write}
	d.descriptors[desc.Name] = desc
	d.tools = []api.Tool{{Type: "function", Function: api.FunctionDef{Name: desc.Name, Description: "Create an OffGrid download", Parameters: map[string]any{"type": "object"}}}, execution.AccessRequestSchema()}
	d.descriptors[execution.AccessRequestTool] = execution.AccessRequestDescriptor()
	r := NewRunner(nil, d, nil)
	task := &Task{Prompt: "create file in this directory named zemo", Config: taskConfig(), Checkpoint: &Checkpoint{}}
	task.Config.ExecutionRequestVersion = execution.AccessRequestVersion
	visible := func() map[string]bool {
		names := map[string]bool{}
		for _, tool := range r.relevantTools(task, append(r.authorizedTools(context.Background(), task), runtimeTools(task)...)) {
			names[tool.Function.Name] = true
		}
		return names
	}
	if names := visible(); names[desc.Name] || !names[execution.AccessRequestTool] || !names["task_find_tools"] {
		t.Fatal("download competed with local access", names)
	}
	if _, err := r.discoverTools(context.Background(), task, `{"query":"task_save_artifact"}`); err != nil {
		t.Fatal(err)
	}
	if !visible()[desc.Name] {
		t.Fatal("explicit download lost")
	}
	d.descriptors[desc.Name] = capabilities.Descriptor{Name: desc.Name, Namespace: "workspace", Source: "workspace", Kind: capabilities.Write, Revision: "changed"}
	if visible()[desc.Name] {
		t.Fatal("stale discovery retained")
	}
	task.Config.ExecutionRequestVersion = 0
	if !visible()[desc.Name] {
		t.Fatal("legacy presentation changed")
	}
	if d.calls.Load() != 0 {
		t.Fatal("discovery dispatched a write")
	}
}

func TestWorkspaceContextDistinguishesDownloadsAndExactFilename(t *testing.T) {
	cfg := taskConfig()
	cfg.ExecutionRequestVersion = execution.AccessRequestVersion
	view := executionResourceContext(cfg)
	for _, want := range []string{"create or edit files", "use edit", "Do not substitute", "add an extension", "downloadable OffGrid artifact"} {
		if !strings.Contains(view, want) {
			t.Fatal("missing destination guidance", want)
		}
	}
	cfg.Execution = &execution.Reference{Host: "host", Session: "session", Grant: "grant", Revision: 1}
	cfg.ExecutionWorkspace = &execution.Resource{Root: "/fixture", Permissions: execution.Permissions{Read: true, Write: true}}
	view = executionResourceContext(cfg)
	for _, want := range []string{"no extension", "never creates a host file", "actual receipts"} {
		if !strings.Contains(view, want) {
			t.Fatal("missing destination guidance", want)
		}
	}
}

func TestInputWriteGuidanceDoesNotRequireDelimitersForOrdinaryWriting(t *testing.T) {
	task := &Task{Config: AgentConfig{InputWriteVersion: execution.InputWriteVersion, ExecutionWorkspace: &execution.Resource{Permissions: execution.Permissions{Write: true}}}}
	view := inputWriteContext(task)
	for _, want := range []string{"workspace_write for ordinary requested text", "actual unique opening AND closing delimiters", "Never invent markers", "preimage revision", "create-only"} {
		if !strings.Contains(view, want) {
			t.Fatal("missing input selection boundary", want)
		}
	}
}
