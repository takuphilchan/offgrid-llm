package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type conversationMetadataTools struct{ *discoveryTools }

func (d *conversationMetadataTools) ConversationToolMetadata(ctx context.Context, _ *Task) []capabilities.Descriptor {
	result := []capabilities.Descriptor{}
	for name, descriptor := range d.descriptors {
		if ctx.Err() == nil && !d.hidden[name] {
			result = append(result, descriptor)
		}
	}
	return result
}

func metadataScope(d *discoveryTools) *ConversationDiscoveryScope {
	scope := &ConversationDiscoveryScope{Version: 1, Capabilities: map[string]capabilities.Descriptor{}}
	for name, descriptor := range d.descriptors {
		identity, _ := ConversationMetadataIdentity(descriptor)
		scope.Capabilities[name] = identity
	}
	return scope
}

func TestConversationMetadataDiscoveryBoundsAndCeiling(t *testing.T) {
	d := &conversationMetadataTools{newDiscoveryTools(20)}
	for name, descriptor := range d.descriptors {
		descriptor.Description = strings.Repeat("Résumé 日本語 ", 200)
		d.descriptors[name] = descriptor
	}
	task := &Task{Config: taskConfig(), ConversationTurn: &ConversationTurnBinding{InputID: "input"}, Checkpoint: &Checkpoint{}}
	task.Config.ScopedCapabilities, task.Config.ConversationDiscovery = true, metadataScope(d.discoveryTools)
	r := NewRunner(nil, d, nil)
	read := func(query string) map[string]json.RawMessage {
		result, err := r.discoverTools(context.Background(), task, query)
		if err != nil {
			t.Fatal(err)
		}
		if len(result) > 12<<10 {
			t.Fatal("unbounded metadata", len(result))
		}
		var response map[string]json.RawMessage
		if json.Unmarshal([]byte(result), &response) != nil {
			t.Fatal(result)
		}
		return response
	}
	response := read(`{"query":""}`)
	var rows []map[string]any
	if json.Unmarshal(response["tools"], &rows) != nil || len(rows) != 8 || string(response["next_offset"]) != "8" || string(response["total_matches"]) != "20" || string(response["execution_authorized"]) != "false" || string(response["metadata_only"]) != "true" {
		t.Fatal(response)
	}
	for _, row := range rows {
		if row["model_schema_state"] != "work_admission_required" || row["parameters"] != nil || row["schema"] != nil {
			t.Fatal(row)
		}
	}
	if len(task.Checkpoint.DiscoveredTools) != 0 || len(r.authorizedTools(context.Background(), task)) != 0 {
		t.Fatal("metadata promoted executable grants")
	}
	response = read(`{"query":"","offset":16}`)
	json.Unmarshal(response["tools"], &rows)
	if len(rows) != 4 || string(response["next_offset"]) != "null" {
		t.Fatal(response)
	}
	d.hidden["mcp_docs_019"] = true
	changed := d.descriptors["mcp_docs_018"]
	changed.Revision = "replaced"
	d.descriptors["mcp_docs_018"] = changed
	d.descriptors["new-unpinned"] = capabilities.Descriptor{Name: "new-unpinned", Source: "mcp:other"}
	response = read(`{"query":""}`)
	if string(response["total_matches"]) != "18" {
		t.Fatal("revoked, changed or unpinned entry disclosed", response)
	}
	if _, err := r.discoverTools(context.Background(), task, `{"query":"","offset":`+fmt.Sprint(math.MaxInt)+`}`); err == nil {
		t.Fatal("invalid offset accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.discoverTools(ctx, task, `{"query":""}`); err == nil {
		t.Fatal("cancelled discovery accepted")
	}
	if d.calls.Load() != 0 {
		t.Fatal("metadata called provider")
	}
}

func TestConversationMetadataDiscoveryDoesNotExpandProtocols(t *testing.T) {
	d := &conversationMetadataTools{newDiscoveryTools(1)}
	r := NewRunner(nil, d, nil)
	for _, mode := range []string{"legacy", "unknown-version", "child", "text-only", "native", "invalid-native"} {
		t.Run(mode, func(t *testing.T) {
			task := &Task{Config: taskConfig(), ConversationTurn: &ConversationTurnBinding{InputID: "input"}, Checkpoint: &Checkpoint{}}
			task.Config.ScopedCapabilities, task.Config.ConversationDiscovery = true, metadataScope(d.discoveryTools)
			switch mode {
			case "legacy":
				task.Config.ConversationDiscovery = nil
			case "unknown-version":
				task.Config.ConversationDiscovery.Version = 99
			case "child":
				task.ParentID = "parent"
			case "text-only":
				task.Config.TextOnlyInference = true
			case "native":
				task.Config.ToolAdapter = &inference.AgentToolPin{Adapter: inference.NativeAgentToolAdapter}
			case "invalid-native":
				task.Config.ToolAdapter = &inference.AgentToolPin{Adapter: inference.NativeAgentToolAdapter, Scope: "unknown"}
			}
			found := false
			for _, tool := range r.relevantTools(task, runtimeTools(task)) {
				found = found || tool.Function.Name == "task_find_tools"
				if mode == "native" && tool.Function.Name == "conversation_decide" {
					t.Fatal("metadata enabled unqualified coordination")
				}
			}
			if found != (mode == "native") {
				t.Fatal("metadata changed protocol boundary", mode, found)
			}
			if len(r.authorizedTools(context.Background(), task)) != 0 {
				t.Fatal("foreground acquired provider schemas")
			}
		})
	}
}

func TestConversationMetadataDiscoveryDurabilityAndForgedDispatch(t *testing.T) {
	for _, forged := range []bool{false, true} {
		m, _, scope, conversation := controlFixture(t)
		d := &conversationMetadataTools{newDiscoveryTools(1)}
		calls := 0
		r := NewRunner(m, d, func(_ context.Context, task *Task, messages []api.ChatMessage, tools []api.Tool) (*api.ChatCompletionResponse, error) {
			calls++
			for _, tool := range tools {
				if tool.Function.Name == "mcp_docs_000" {
					t.Error("metadata promoted provider schema")
				}
			}
			if calls == 1 {
				return taskCall("task_find_tools", `{"query":"mcp_docs_000"}`), nil
			}
			if calls == 2 && forged {
				return taskCall("mcp_docs_000", `{}`), nil
			}
			return runAnswer(api.ChatMessage{Role: "assistant", Content: "The tool description is available; execution requires authorized work."}), nil
		})
		receipt, err := r.AcceptConversationInput(scope, inboxCommand(conversation, "metadata", "Which documentation tool could help?"))
		if err != nil {
			t.Fatal(err)
		}
		config := taskConfig()
		config.ConversationDiscovery = metadataScope(d.discoveryTools)
		task, err := r.AdmitConversationTurn(scope, conversation.ID, receipt.Input.ID, receipt.Conversation.Revision, "model", config)
		if err != nil {
			t.Fatal(err)
		}
		final, err := r.Continue(context.Background(), task.ID, scope.Actor, "start", "", false)
		if d.calls.Load() != 0 || calls < 2 || final == nil || len(final.Checkpoint.DiscoveredTools) != 0 {
			t.Fatal("provider executed or discovery failed", final, calls, err)
		}
		if !forged && (err != nil || final.Status != TaskCompleted || calls != 2) {
			t.Fatal(final, err)
		}
		foundReceipt := false
		for _, step := range final.Steps {
			foundReceipt = foundReceipt || step.ToolName == "task_find_tools" && strings.Contains(step.ToolResult, "work_admission_required")
		}
		if !foundReceipt {
			t.Fatal("durable metadata receipt missing", final.Steps)
		}
		restarted := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
		stored, ok := restarted.GetTask(task.ID)
		if !ok || stored.Config.ConversationDiscovery == nil || len(stored.Config.AllowedCapabilities) != 0 || len(stored.Checkpoint.DiscoveredTools) != 0 {
			t.Fatal("metadata became permission", stored)
		}
	}
}
