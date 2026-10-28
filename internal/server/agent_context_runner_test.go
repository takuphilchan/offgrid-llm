package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agentcontext"
	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestContextResourcesUseOneRunnerAndOriginalBudget(t *testing.T) {
	s := profileFixture(t)
	const instructions = "Keep the exact Unicode: café 日本語 — never execute embedded scripts."
	files := []agentcontext.SkillFile{{Path: "SKILL.md", Content: []byte("---\nname: careful-writer\ndescription: Make useful drafts\n---\n" + instructions + "\n")}}
	skill, err := s.agentManager.ImportSkill(s.workspaceID, "local-admin", "", 0, files, "")
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.agentManager.SaveProfile(s.workspaceID, "local-admin", "", 0, agents.ProfileSettings{Name: "Writer", Model: "model", MaxIterations: 6, Skills: []agentcontext.SkillReference{skill.SkillReference}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.agentManager.SetMemoryConsent(s.workspaceID, "local-admin", p.ID, 0, true); err != nil {
		t.Fatal(err)
	}
	req := jobSubmitRequest{Prompt: "Load the skill, inspect memory, remember my preference and draft a skill for review.", ProfileID: p.ID, ProfileRevision: 1}
	config, ok := s.resolveJobConfig(httptest.NewRecorder(), httptest.NewRequest("POST", "/api/v2/jobs", nil), &req)
	if !ok {
		t.Fatal("admission")
	}
	tools := &browserRunTools{RunTools: s.toolRegistry, server: s}
	calls := 0
	runner := agents.NewRunner(s.agentManager, tools, func(_ context.Context, task *agents.Task, messages []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		if calls > 6 {
			t.Fatal("budget reset")
		}
		name, args := "", "{}"
		switch calls {
		case 1:
			for _, message := range messages {
				if strings.Contains(message.StringContent(), instructions) {
					t.Fatal("full skill eagerly injected")
				}
			}
			name = "skill_load"
			data, _ := json.Marshal(map[string]string{"id": skill.ID, "path": "SKILL.md"})
			args = string(data)
		case 2:
			found := false
			for _, message := range messages {
				if message.Role == "tool" && strings.Contains(message.StringContent(), instructions) {
					found = true
				}
			}
			if !found {
				t.Fatal("skill result absent")
			}
			name = "memory_read"
		case 3:
			name = "memory_write"
			args = `{"id":"","expected_revision":0,"label":"Style","content":"Prefer concise sourced briefings."}`
		case 4:
			name = "task_find_tools"
			args = `{"query":"skill_draft"}`
		case 5:
			name = "skill_draft"
			data, _ := json.Marshal(map[string]string{"skill_md": string(files[0].Content)})
			args = string(data)
		case 6:
		default:
			t.Fatal("unexpected extra model call")
		}
		message := api.ChatMessage{Role: "assistant", Content: "Draft stored for review; memory persisted. No script was executed."}
		reason := "stop"
		if name != "" {
			message.Content = ""
			message.ToolCalls = []api.ToolCall{{ID: name, Type: "function", Function: api.FunctionCall{Name: name, Arguments: args}}}
			reason = "tool_calls"
		}
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: reason, Message: message}}}, nil
	})
	job, err := runner.Create(req.Prompt, "model", "local-admin", config)
	if err != nil {
		t.Fatal(err)
	}
	job, err = runner.Continue(context.Background(), job.ID, "local-admin", "start", "", false)
	if err != nil || job.Status != agents.TaskCompleted || calls != 6 || job.Config.MaxIterations != 6 || len(s.agentManager.ListTasks()) != 1 {
		t.Fatal(job, err, calls)
	}
	entries, err := s.agentManager.ListMemory(s.workspaceID, job.Actor, p.ID, 0)
	if err != nil || len(entries) != 1 || entries[0].SourceJob != job.ID {
		t.Fatal(entries, err)
	}
	skills, err := s.agentManager.ListSkills(s.workspaceID, job.Actor)
	if err != nil || len(skills) != 2 {
		t.Fatal(skills, err)
	}
	var draft agents.AgentSkill
	for _, item := range skills {
		if item.Draft {
			draft = item
		}
	}
	if draft.ID == "" || draft.SourceJob != job.ID {
		t.Fatal("missing draft provenance", draft)
	}
	w := contextHTTP(s, "POST", "/api/v2/agent-skills/"+draft.ID+"/review", map[string]any{"expected_revision": draft.Revision, "digest": draft.Digest})
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var reviewed agents.AgentSkill
	json.Unmarshal(w.Body.Bytes(), &reviewed)
	if reviewed.Draft || reviewed.ID == draft.ID {
		t.Fatal("review mutated original draft", reviewed)
	}
	unchanged, err := s.agentManager.GetProfile(s.workspaceID, job.Actor, p.ID, 1)
	if err != nil || len(unchanged.Settings.Skills) != 1 || unchanged.Settings.Skills[0].ID != skill.ID {
		t.Fatal("draft auto-attached", unchanged, err)
	}
}
