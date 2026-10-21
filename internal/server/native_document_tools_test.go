package server

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/computer"
)

func TestDocumentReadSchemaCannotInventPaginationBeforeSnapshot(t *testing.T) {
	task, _, _, read := documentGuardFixture("insert_text")
	all := append([]agents.Step(nil), task.Steps...)
	properties := func() map[string]any {
		t.Helper()
		for _, tool := range stagedDocumentTools(task) {
			if tool.Function.Name == "computer_read_text" {
				return tool.Function.Parameters["properties"].(map[string]any)
			}
		}
		t.Fatal("read tool missing")
		return nil
	}
	task.Steps = all[:1]
	if _, exists := properties()["page"]; exists {
		t.Fatal("first read invited invented snapshot")
	}
	read.Page = &computer.DocumentTextPage{Snapshot: read.Snapshot, Binding: read.Binding, Text: "prefix suffix", End: 13, TotalScalars: 14, SourceComplete: true}
	next := 13
	read.Page.Next = &next
	read.Bytes = 14
	read.TextSHA256 = computer.DocumentTextDigest("prefix suffix.")
	all[1].ToolResult = documentJSON(read)
	task.Steps = all
	p := properties()
	b, _ := json.Marshal(p["page"])
	if !strings.Contains(string(b), read.Snapshot) || !strings.Contains(string(b), read.Binding.Revision) || !strings.Contains(string(b), `"const":13`) {
		t.Fatal("unbound continuation page", string(b))
	}
	for _, tool := range stagedDocumentTools(task) {
		if tool.Function.Name == "computer_read_text" {
			required, _ := json.Marshal(tool.Function.Parameters["required"])
			if !strings.Contains(string(required), `"page"`) {
				t.Fatal("continuation may restart the completed first page")
			}
		}
	}
	read.Page.Next = nil
	read.Page.Text = "prefix suffix."
	read.Page.End = 14
	all[1].ToolResult = documentJSON(read)
	if availableNames(stagedDocumentTools(task))["computer_read_text"] {
		t.Fatal("completed snapshot invited repeated paging")
	}
	task.Steps = append(task.Steps, agents.Step{ID: 3, Type: "action", ToolName: "computer_documents", ToolResult: all[0].ToolResult})
	if !availableNames(stagedDocumentTools(task))["computer_read_text"] {
		t.Fatal("fresh inventory cannot be read")
	}
}

func TestDocumentSchemasHaveArrayRequirements(t *testing.T) {
	for _, tool := range nativeDocumentTools() {
		encoded, err := json.Marshal(tool)
		if err != nil || strings.Contains(string(encoded), `"required":null`) {
			t.Fatal("invalid JSON Schema required field", tool.Function.Name, err)
		}
	}
}

func TestDocumentToolsRequireNegotiatedHostAndKeepMCP(t *testing.T) {
	for _, version := range []int{0, 1, 2} {
		for _, feature := range []int{0, 1} {
			s, adapter, _, _, _ := compositionFixture(t)
			code, _ := s.browserHub.PairCode("alice")
			target := computer.TargetIdentity{ID: "window", OSSession: "os", ProcessGeneration: "process", Surface: "surface", Driver: "windows-uia"}
			host, token, err := s.browserHub.PairNativeFeatures(code, "Fixture", target, 2, computer.ApprovalFullTask, computer.NativeFeatures{Documents: feature})
			if err != nil {
				t.Fatal(err)
			}
			cfg := compositionConfig(target.Driver, true)
			cfg.ComputerSession, cfg.NativeWorkflowVersion = host.ID, version
			runner := agents.NewRunner(s.agentManager, adapter, nil)
			task, err := runner.Create("Edit a new document", "model", "alice", cfg)
			if err != nil {
				t.Fatal(err)
			}
			tools := adapter.ToolsForTask(task)
			names := availableNames(tools)
			want := version == 2 && feature == 1
			if names["computer_documents"] != want || !names["mcp__docs__docs"] {
				t.Fatal("capability composition", version, feature, names)
			}
			if names["computer_insert_text"] {
				t.Fatal("edit before observation")
			}
			if names["computer_replace_text"] {
				t.Fatal("legacy editing offered before observation")
			}
			task.Steps = []agents.Step{{ID: 1, Type: "action", ToolName: "computer_observe"}}
			if !availableNames(adapter.ToolsForTask(task))["computer_replace_text"] {
				t.Fatal("legacy control editing disappeared after observation")
			}
			if !want {
				d, _ := adapter.Capability("computer_documents")
				err = adapter.Authorize(context.Background(), "computer_documents", json.RawMessage(`{}`), agents.ToolExecution{Actor: "alice", RunID: task.ID, CallID: "documents", ExpectedCapability: &d})
				if err == nil {
					t.Fatal("unnegotiated authorization")
				}
				if action, _ := s.browserHub.Poll(token); action != nil {
					t.Fatal("contacted unnegotiated host")
				}
			}
			foreign := *task
			foreign.Actor = "bob"
			if availableNames(adapter.ToolsForTask(&foreign))["computer_documents"] {
				t.Fatal("cross-actor feature discovery")
			}
		}
	}
}

func TestDocumentSchemasFollowObservedStage(t *testing.T) {
	task, _, _, read := documentGuardFixture("insert_text")
	read.Page = &computer.DocumentTextPage{Snapshot: read.Snapshot, Binding: read.Binding, SourceComplete: true, Text: "prefix suffix", End: 13, TotalScalars: 13}
	read.Selection = &computer.DocumentTextRange{Start: 7, End: 7}
	task.Steps[1].ToolResult = documentJSON(read)
	names := availableNames(stagedDocumentTools(task))
	if !names["computer_insert_text"] || names["computer_replace_selection"] || !names["computer_create_document"] {
		t.Fatal(names)
	}
	read.Selection.End = 8
	task.Steps[1].ToolResult = documentJSON(read)
	names = availableNames(stagedDocumentTools(task))
	if names["computer_insert_text"] || !names["computer_replace_selection"] {
		t.Fatal(names)
	}
	task.Steps = append(task.Steps, agents.Step{ID: 3, Type: "action", ToolName: "computer_select_text"})
	names = availableNames(stagedDocumentTools(task))
	if len(names) != 1 || !names["computer_documents"] {
		t.Fatal("stale schema", names)
	}
}

func TestDocumentSelectionSchemaExcludesAlreadyActiveDocument(t *testing.T) {
	task, _, inventory, _ := documentGuardFixture("insert_text")
	inventory.Documents = append(inventory.Documents, computer.NativeDocument{ID: "other", Generation: "other-generation", Container: "other-container", Capabilities: computer.DocumentCapabilities{Select: true}})
	task.Steps = task.Steps[:1]
	task.Steps[0].ToolResult = documentJSON(inventory)
	for _, tool := range stagedDocumentTools(task) {
		if tool.Function.Name != "computer_select_document" {
			continue
		}
		properties := tool.Function.Parameters["properties"].(map[string]any)
		ids := properties["document"].(map[string]any)["enum"].([]string)
		if len(ids) != 1 || ids[0] != "other" {
			t.Fatal("active or invented document offered", ids)
		}
		if properties["observation_id"].(map[string]any)["const"] != inventory.Observation.ID {
			t.Fatal("unbound selection")
		}
		return
	}
	t.Fatal("selectable other document missing")
}
