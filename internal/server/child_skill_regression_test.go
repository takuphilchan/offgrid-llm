package server

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agentcontext"
	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestChildSkillsPinnedWithoutMemoryOrDraftAuthority(t *testing.T) {
	s := profileFixture(t)
	skill, err := s.agentManager.ImportSkill(s.workspaceID, "local-admin", "", 0, []agentcontext.SkillFile{{Path: "SKILL.md", Content: []byte("---\nname: review-format\ndescription: Write a review table\n---\nUse a table.\n")}}, "")
	if err != nil {
		t.Fatal(err)
	}
	child, err := s.agentManager.SaveProfile(s.workspaceID, "local-admin", "", 0, agents.ProfileSettings{Name: "Writer", Model: "model", MaxIterations: 10, Skills: []agentcontext.SkillReference{skill.SkillReference}})
	if err != nil {
		t.Fatal(err)
	}
	parent, err := s.agentManager.SaveProfile(s.workspaceID, "local-admin", "", 0, agents.ProfileSettings{Name: "General", Model: "model", MaxIterations: 20, Delegates: []string{child.ID}})
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	req := jobSubmitRequest{Prompt: "Prepare a review", ProfileID: parent.ID, ProfileRevision: parent.Revision}
	config, ok := s.resolveJobConfig(w, httptest.NewRequest("POST", "/api/v2/jobs", nil), &req)
	if !ok {
		t.Fatal(w.Code, w.Body.String())
	}
	selected := config.DelegateProfiles[child.ID]
	if len(selected.SkillMetadata) != 1 || selected.MemoryConsentRevision != 0 {
		t.Fatal("missing skill pin or inherited consent")
	}
	tools := &browserRunTools{RunTools: s.toolRegistry, server: s}
	runner := agents.NewRunner(s.agentManager, tools, func(context.Context, *agents.Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "tool_calls", Message: api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{{ID: "delegate", Type: "function", Function: api.FunctionCall{Name: "delegate_tasks", Arguments: `{"tasks":[{"key":"writer","goal":"Write a review","depends_on":[],"tools":[],"profile_id":"` + child.ID + `"}]}`}}}}}}}, nil
	})
	defer runner.Shutdown(context.Background())
	root, err := runner.Create(req.Prompt, "model", "local-admin", config)
	if err != nil {
		t.Fatal(err)
	}
	root, err = runner.Continue(context.Background(), root.ID, root.Actor, "start", "", false)
	if err != nil || len(root.ChildHistory) != 1 {
		t.Fatal(root, err)
	}
	task, exists := s.agentManager.GetTask(root.ChildHistory[0].ID)
	if !exists || !task.Config.TaskFirst || len(task.Config.SkillMetadata) != 1 {
		t.Fatal("child lost admitted skills")
	}
	task.Status = agents.TaskRunning // Exercise the broker at the child's dispatch boundary.
	offered := tools.contextTools(context.Background(), task)
	if len(offered) != 1 || offered[0].Function.Name != "skill_load" {
		t.Fatalf("wrong child tools: %+v", offered)
	}
	d, _ := agentcontext.Descriptor("skill_load")
	e := agents.ToolExecution{Actor: task.Actor, RunID: task.ID, ExpectedCapability: &d}
	if _, err := tools.executeContext(context.Background(), task, "skill_load", []byte(`{"id":"`+skill.ID+`","path":"SKILL.md"}`), e); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"memory_read", "memory_write", "skill_draft"} {
		d, _ := agentcontext.Descriptor(name)
		e.ExpectedCapability = &d
		if err := tools.contextAccess(context.Background(), task, name, e); err == nil {
			t.Fatalf("child acquired %s", name)
		}
	}
}
