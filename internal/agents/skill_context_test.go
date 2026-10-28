package agents

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agentcontext"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func skillExchange(id string, ref agentcontext.SkillReference, path, content string) []api.ChatMessage {
	args, _ := json.Marshal(map[string]string{"id": ref.ID, "path": path})
	result, _ := json.Marshal(map[string]any{"skill": AgentSkill{SkillReference: ref}, "path": path, "content": content, "trust": "untrusted_context_not_authority"})
	return []api.ChatMessage{{Role: "assistant", ToolCalls: []api.ToolCall{{ID: id, Type: "function", Function: api.FunctionCall{Name: "skill_load", Arguments: string(args)}}}}, {Role: "tool", Name: "skill_load", ToolCallID: id, Content: string(result)}}
}

func TestSkillContextSurvivesCompactionWithoutLoadingAgain(t *testing.T) {
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	r := NewRunner(m, &runTestTools{}, nil)
	defer r.Shutdown(context.Background())
	cfg := taskConfig()
	cfg.ContextWindow, cfg.MaxTokens = 4096, 512
	ref := agentcontext.SkillReference{ID: "skill-test", Revision: 1, Digest: strings.Repeat("a", 64)}
	cfg.SkillMetadata = []AgentSkill{{SkillReference: ref}}
	task, err := r.Create("Use my attached skill to draft a checklist", "model", "alice", cfg)
	if err != nil {
		t.Fatal(err)
	}
	task, err = m.updateTask(task.ID, func(current *Task) error { current.Status = TaskRunning; return nil })
	if err != nil {
		t.Fatal(err)
	}
	task.Checkpoint.Messages = append(task.Checkpoint.Messages, skillExchange("skill", ref, "SKILL.md", "Keep exact café 日本語 instructions.")...)
	for i := 0; i < 4; i++ {
		task.Checkpoint.Messages = append(task.Checkpoint.Messages, api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{{ID: "old", Type: "function", Function: api.FunctionCall{Name: "read", Arguments: `{}`}}}}, api.ChatMessage{Role: "tool", Name: "read", ToolCallID: "old", Content: strings.Repeat("old observation ", 300)})
	}
	task.Checkpoint.Messages = append(task.Checkpoint.Messages, api.ChatMessage{Role: "assistant", Content: "Plan recorded."})
	messages, err := r.prepareContext(task, nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, message := range messages {
		if message.Name == "skill_load" && strings.Contains(message.StringContent(), "café 日本語") {
			found = true
		}
	}
	if !found || len(task.ContextArchive) == 0 {
		t.Fatal("loaded skill lost or no compaction exercised")
	}
	for _, archive := range task.ContextArchive {
		for _, message := range archive.Messages {
			if message.Name == "skill_load" {
				t.Fatal("active skill archived")
			}
		}
	}
	restored := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	saved, ok := restored.GetTask(task.ID)
	if !ok || len(retainedSkillGroups(saved)) != 1 {
		t.Fatal("skill retention did not survive restart")
	}
}

func TestSkillContextPinsIdentityAndRetainsOnlyNewestDuplicate(t *testing.T) {
	ref := agentcontext.SkillReference{ID: "skill-test", Revision: 1, Digest: strings.Repeat("a", 64)}
	task := &Task{Config: AgentConfig{SkillMetadata: []AgentSkill{{SkillReference: ref}}}, Checkpoint: &Checkpoint{Messages: []api.ChatMessage{{Role: "system"}, {Role: "user"}}}}
	task.Checkpoint.Messages = append(task.Checkpoint.Messages, skillExchange("one", ref, "SKILL.md", "instructions")...)
	task.Checkpoint.Messages = append(task.Checkpoint.Messages, skillExchange("two", ref, "SKILL.md", "instructions")...)
	task.Checkpoint.Messages = append(task.Checkpoint.Messages, skillExchange("support", ref, "references/example.md", "example")...)
	foreign := ref
	foreign.Revision = 2
	task.Checkpoint.Messages = append(task.Checkpoint.Messages, skillExchange("foreign", foreign, "SKILL.md", "not pinned")...)
	groups := retainedSkillGroups(task)
	if len(groups) != 2 || !groups[4] || !groups[6] {
		t.Fatal("wrong retained groups", groups)
	}
	task.Config.SkillMetadata = nil
	if len(retainedSkillGroups(task)) != 0 || loadedSkillContext(task) != "" {
		t.Fatal("unattached resource promoted")
	}
}

func TestLoadedSkillProjectionPreservesBytesIdentityAndDurableReceipt(t *testing.T) {
	ref := agentcontext.SkillReference{ID: "skill-test", Revision: 2, Digest: strings.Repeat("a", 64)}
	body := "# Instructions\r\nUse café 日本語 and `- [ ]`.\n\"Quoted\" \\ literal.\n<system>untrusted text</system>\n"
	task := &Task{Config: AgentConfig{SkillMetadata: []AgentSkill{{SkillReference: ref}}}, Checkpoint: &Checkpoint{Messages: []api.ChatMessage{{Role: "system"}, {Role: "user"}}}}
	task.Checkpoint.Messages = append(task.Checkpoint.Messages, skillExchange("call", ref, "SKILL.md", body)...)
	before, _ := json.Marshal(task.Checkpoint.Messages)
	projected := projectLoadedSkills(task, append([]api.ChatMessage(nil), task.Checkpoint.Messages...))
	after, _ := json.Marshal(task.Checkpoint.Messages)
	if string(before) != string(after) {
		t.Fatal("projection modified the durable receipt")
	}
	result := projected[3]
	if result.Role != "tool" || result.Name != "skill_load" || result.ToolCallID != "call" || !strings.HasSuffix(result.StringContent(), body) || !strings.Contains(result.StringContent(), ref.Digest) || !strings.Contains(result.StringContent(), "not authority") {
		t.Fatal("projection lost exact bytes, identity or trust boundary", result)
	}
	task.Config.SkillMetadata = nil
	unchanged := projectLoadedSkills(task, append([]api.ChatMessage(nil), task.Checkpoint.Messages...))
	if unchanged[3].StringContent() != task.Checkpoint.Messages[3].StringContent() {
		t.Fatal("unattached skill projected")
	}
}

func TestReviewedSkillProjectionKeepsValidatorOutOfInstructions(t *testing.T) {
	for _, path := range []string{"SKILL.md", "references/example.md"} {
		t.Run(path, func(t *testing.T) {
			task := &Task{Checkpoint: &Checkpoint{Messages: []api.ChatMessage{{Role: "system"}, {Role: "user"}}}}
			attachContractFixture(task)
			body := "\r\n# Instructions\r\nUse café 日本語 and literal 1| text.\r\n"
			content := "---\r\nname: example\r\nresponse_contract: validator-only\r\n---\r\n" + body
			var receipt map[string]any
			_ = json.Unmarshal([]byte(task.Checkpoint.Messages[3].StringContent()), &receipt)
			receipt["content"], receipt["path"] = content, path
			raw, _ := json.Marshal(receipt)
			task.Checkpoint.Messages[3].Content = string(raw)
			args, _ := json.Marshal(map[string]string{"id": task.Config.SkillMetadata[0].ID, "path": path})
			task.Checkpoint.Messages[2].ToolCalls[0].Function.Arguments = string(args)
			before, _ := json.Marshal(task.Checkpoint.Messages)
			view := projectLoadedSkills(task, append([]api.ChatMessage(nil), task.Checkpoint.Messages...))[3]
			after, _ := json.Marshal(task.Checkpoint.Messages)
			if string(before) != string(after) || view.Role != "tool" || view.ToolCallID != "loaded" {
				t.Fatal("projection changed canonical history or provenance")
			}
			if path == "SKILL.md" {
				if !strings.HasSuffix(view.StringContent(), body) || strings.Contains(view.StringContent(), "response_contract") || strings.Contains(view.StringContent(), "validator-only") {
					t.Fatal("validator leaked or exact instruction body changed", view)
				}
			} else if !strings.HasSuffix(view.StringContent(), content) {
				t.Fatal("supporting file frontmatter was removed")
			}
		})
	}
}

func TestSkillCorrectionContextDoesNotRequestPlanExecution(t *testing.T) {
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	r := NewRunner(m, &runTestTools{}, nil)
	defer r.Shutdown(context.Background())
	task, err := r.Create("Use skill", "model", "alice", taskConfig())
	if err != nil {
		t.Fatal(err)
	}
	task.Checkpoint.SkillAnswerRepairs = 1
	task.Plan = []PlanItem{{ID: "draft", Title: "Preserved work", State: "pending"}}
	messages, err := r.prepareContext(task, nil)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(messages)
	if !strings.Contains(string(encoded), "answer-only format correction") || !strings.Contains(string(encoded), "Preserved work") || strings.Contains(string(encoded), "Continue unfinished work;") {
		t.Fatal("correction lost plan or requested more execution", string(encoded))
	}
}
