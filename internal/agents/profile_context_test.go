package agents

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestEffectiveAttachmentsOverridePersonaAndRespectRestrictions(t *testing.T) {
	cfg := taskConfig()
	cfg.ContextWindow, cfg.MaxTokens = 8192, 512
	cfg.Profile = &AgentProfile{ID: "parent", Revision: 3, Settings: ProfileSettings{Name: "Bumblebee", Knowledge: []string{"a", "b", "hidden"}}}
	selected := []string{"a", "b"}
	cfg.KnowledgeSelection = &selected
	cfg.KnowledgeDocuments = []KnowledgeDocument{{"a", "hello.txt"}, {"b", "日本語.txt"}, {"hidden", "PRIVATE"}}
	cfg.KnowledgeContext = "hello agent"
	cfg.ConversationContext = []api.ChatMessage{{Role: "assistant", Content: "I only know Autobots. I have no documents or agents."}}
	d := capabilities.Descriptor{Name: "calculator", Source: "builtin", Namespace: "tools", Kind: capabilities.Read, Risk: capabilities.RiskLow}
	cfg.AllowedCapabilities = map[string]capabilities.Descriptor{"calculator": d}
	cfg.DelegateProfiles = map[string]AgentConfig{"writer": {Profile: &AgentProfile{Revision: 2, Settings: ProfileSettings{Name: "Document specialist", Description: "Drafts documents"}}, AllowedCapabilities: map[string]capabilities.Descriptor{"calculator": d}}}
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	r := NewRunner(m, &runTestTools{}, nil)
	task, err := r.Create("What documents and agents are attached?", "model", "alice", cfg)
	if err != nil {
		t.Fatal(err)
	}
	// A plan must not overwrite the resource inventory; it must survive a
	// persisted round-trip and a computer protocol replacement as well.
	task.Config.SystemPrompt = "Computer protocol"
	task.Plan = []PlanItem{{Title: "Check resources"}}
	encoded, _ := json.Marshal(task)
	if err := json.Unmarshal(encoded, &task); err != nil {
		t.Fatal(err)
	}
	messages, err := r.prepareContext(task, nil)
	if err != nil {
		t.Fatal(err)
	}
	var inventory string
	for _, message := range messages {
		if message.Role == "system" && strings.Contains(message.StringContent(), "Effective attachments") {
			inventory = message.StringContent()
		}
	}
	for _, expected := range []string{`"attached_document_count":2`, `"attached_agent_count":1`, `"revision":3`, "hello.txt", "日本語.txt", "Document specialist", "Drafts documents", "calculator"} {
		if !strings.Contains(inventory, expected) {
			t.Errorf("missing %s in %s", expected, inventory)
		}
	}
	if strings.Contains(inventory, "PRIVATE") {
		t.Fatal("excluded Knowledge leaked")
	}
	if messages[len(messages)-1].StringContent() != task.Prompt || !strings.Contains(messages[len(messages)-2].StringContent(), "Effective attachments") {
		t.Fatal("current resource facts must follow old persona/history and precede the fresh question")
	}
	// Compaction is a running-worker operation; its durable write correctly
	// rejects a task that is still queued. Activate only this isolated fixture.
	if _, err := m.updateTask(task.ID, func(current *Task) error {
		current.Status = TaskRunning
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	task.Status = TaskRunning
	for i := 0; i < 12; i++ {
		task.Checkpoint.Messages = append(task.Checkpoint.Messages,
			api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{{ID: "read", Type: "function", Function: api.FunctionCall{Name: "read", Arguments: `{}`}}}},
			api.ChatMessage{Role: "tool", ToolCallID: "read", Content: strings.Repeat("archivable evidence ", 150)})
	}
	messages, err = r.prepareContext(task, nil)
	if err != nil || len(task.ContextArchive) == 0 {
		t.Fatalf("expected successful compaction: %v", err)
	}
	found := false
	for _, message := range messages {
		if message.Role == "system" && strings.Contains(message.StringContent(), `"attached_document_count":2`) {
			found = true
		}
	}
	if !found {
		t.Fatal("compaction lost the attachment inventory")
	}
	empty := []string{}
	cfg.KnowledgeSelection = &empty
	cfg.DelegateProfiles = nil
	inventory = profileResourceContext(cfg)
	if !strings.Contains(inventory, `"attached_document_count":0`) || !strings.Contains(inventory, `"attached_agent_count":0`) {
		t.Fatal(inventory)
	}
	if profileResourceContext(AgentConfig{}) != "" {
		t.Fatal("legacy task received fabricated inventory")
	}
}
