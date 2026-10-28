package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agentcontext"
	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestSkillDiscoveryOmitsOnlyRetainedImmutableFiles(t *testing.T) {
	s := profileFixture(t)
	skill, err := s.agentManager.ImportSkill(s.workspaceID, "local-admin", "", 0, []agentcontext.SkillFile{{Path: "SKILL.md", Content: []byte("---\nname: checklist\ndescription: Make a checklist\n---\nUse references/example.md when needed.\n")}, {Path: "references/example.md", Content: []byte("Supporting instructions.")}}, "")
	if err != nil {
		t.Fatal(err)
	}
	profile, err := s.agentManager.SaveProfile(s.workspaceID, "local-admin", "", 0, agents.ProfileSettings{Name: "Checklist", Model: "model", MaxIterations: 6, Skills: []agentcontext.SkillReference{skill.SkillReference}})
	if err != nil {
		t.Fatal(err)
	}
	req := jobSubmitRequest{Prompt: "Use the checklist", ProfileID: profile.ID, ProfileRevision: profile.Revision}
	cfg, ok := s.resolveJobConfig(httptest.NewRecorder(), httptest.NewRequest("POST", "/api/v2/jobs", nil), &req)
	if !ok {
		t.Fatal("admission")
	}
	task, err := s.agentRunner.Create(req.Prompt, "model", "local-admin", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.agentManager.StartTask(task.ID); err != nil {
		t.Fatal(err)
	}
	task, _ = s.agentManager.GetTask(task.ID)
	tools := &browserRunTools{RunTools: s.toolRegistry, server: s}
	schema := func() string {
		for _, tool := range tools.contextTools(context.Background(), task) {
			if tool.Function.Name == "skill_load" {
				data, _ := json.Marshal(tool.Function.Parameters)
				return string(data)
			}
		}
		return ""
	}
	if got := schema(); !strings.Contains(got, "SKILL.md") || !strings.Contains(got, "references/example.md") || !strings.Contains(got, skill.ID) {
		t.Fatal("initial paths", got)
	}
	load := func(id, path string) {
		args, _ := json.Marshal(map[string]string{"id": skill.ID, "path": path})
		d, _ := contextDescriptor("skill_load")
		result, err := tools.executeContext(context.Background(), task, "skill_load", args, agents.ToolExecution{Actor: task.Actor, RunID: task.ID, CallID: id, ExpectedCapability: &d})
		if err != nil {
			t.Fatal(err)
		}
		task.Checkpoint.Messages = append(task.Checkpoint.Messages, api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{{ID: id, Type: "function", Function: api.FunctionCall{Name: "skill_load", Arguments: string(args)}}}}, api.ChatMessage{Role: "tool", Name: "skill_load", ToolCallID: id, Content: result})
	}
	load("first", "SKILL.md")
	if got := schema(); strings.Contains(got, "SKILL.md") || !strings.Contains(got, "references/example.md") {
		t.Fatal("supporting file not independently available", got)
	}
	load("second", "references/example.md")
	if got := schema(); got != "" {
		t.Fatal("all retained files offered again", got)
	}
	// A later task has no loaded context and must obtain its own authorized read.
	task.Checkpoint.Messages = task.Checkpoint.Messages[:2]
	if got := schema(); !strings.Contains(got, "SKILL.md") {
		t.Fatal("load state leaked across task contexts", got)
	}
	d, _ := contextDescriptor("skill_draft")
	_, err = tools.executeContext(context.Background(), task, "skill_draft", json.RawMessage(`{"skill_md":"an ordinary answer without skill metadata"}`), agents.ToolExecution{Actor: task.Actor, RunID: task.ID, CallID: "invalid-draft", ExpectedCapability: &d})
	var failure *agents.ToolFailure
	if !errors.As(err, &failure) || !failure.BeforeEffects || failure.Code != "skill_draft_invalid" {
		t.Fatal("validation rejection treated as unknown effects", err)
	}
	stored, err := s.agentManager.ListSkills(s.workspaceID, task.Actor)
	if err != nil || len(stored) != 1 {
		t.Fatal("invalid draft stored", err)
	}
}
