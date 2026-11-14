package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agentcontext"
	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestConversationResourcesPinOwnedSkillsWithoutProfile(t *testing.T) {
	s := profileFixture(t)
	s.config.EnableConversationPreview = true
	skill, err := s.agentManager.ImportSkill(s.workspaceID, "local-admin", "", 0, []agentcontext.SkillFile{{Path: "SKILL.md", Content: []byte("---\nname: format\ndescription: Format a result\n---\nUse a table.\n")}}, "")
	if err != nil {
		t.Fatal(err)
	}
	scope := agents.ControlScope{Workspace: s.workspaceID, Actor: "local-admin"}
	project, err := s.agentManager.ApplyProjectCommand(scope, api.ProjectCommand{RequestID: "project", Kind: "create_project", Payload: api.ProjectCommandPayload{Name: "Project", Resources: []api.ProjectResourceReference{{Kind: "skill", ID: skill.ID, Revision: "1", Required: true}}}})
	if err != nil {
		t.Fatal(err)
	}
	created, err := s.agentManager.ApplyConversationCommand(scope, agents.ConversationCommand{RequestID: "conversation", Kind: "create_conversation", Payload: agents.ConversationCommandPayload{ProjectID: project.TargetID, Preferences: &agents.ConversationPreferences{Model: "model"}}})
	if err != nil {
		t.Fatal(err)
	}
	// A project pin must keep revision one after the library is edited.
	_, err = s.agentManager.ImportSkill(s.workspaceID, "local-admin", skill.ID, 1, []agentcontext.SkillFile{{Path: "SKILL.md", Content: []byte("---\nname: format\ndescription: Format a result\n---\nUse bullets.\n")}}, "")
	if err != nil {
		t.Fatal(err)
	}
	tools := &browserRunTools{RunTools: s.toolRegistry, server: s}
	s.agentRunner = agents.NewRunner(s.agentManager, tools, nil)
	defer s.agentRunner.Shutdown(context.Background())
	command := agents.ConversationCommand{RequestID: "work", TargetID: created.TargetID, ExpectedRevision: 1, Kind: "create_work", Payload: agents.ConversationCommandPayload{Input: &agents.ConversationInput{Content: "Format this"}}}
	w := controlHTTP(s, "POST", "/api/v2/conversations/"+created.TargetID+"/commands", command)
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	var receipt agents.ConversationCommandReceipt
	if err := json.Unmarshal(w.Body.Bytes(), &receipt); err != nil {
		t.Fatal(err)
	}
	task, _ := s.agentManager.GetTask(receipt.Work.RunID)
	if task.Config.WorkerQuestionsVersion != 1 {
		t.Fatal("conversation admission did not pin resumable clarification support")
	}
	if task.Config.Profile != nil || len(task.Config.SkillMetadata) != 1 || task.Config.SkillMetadata[0].SkillReference != skill.SkillReference {
		t.Fatalf("unscoped or unpinned skill: %+v", task.Config.SkillMetadata)
	}
	task.Status = agents.TaskRunning
	offered := tools.contextTools(context.Background(), task)
	if len(offered) != 1 || offered[0].Function.Name != "skill_load" {
		t.Fatalf("wrong tools: %+v", offered)
	}
	d, _ := agentcontext.Descriptor("skill_load")
	result, err := tools.executeContext(context.Background(), task, d.Name, []byte(`{"id":"`+skill.ID+`","path":"SKILL.md"}`), agents.ToolExecution{Actor: task.Actor, RunID: task.ID, ExpectedCapability: &d})
	if err != nil {
		t.Fatal(err)
	}
	var loaded struct {
		Content string `json:"content"`
	}
	if json.Unmarshal([]byte(result), &loaded) != nil || loaded.Content != "---\nname: format\ndescription: Format a result\n---\nUse a table.\n" {
		t.Fatal(result)
	}
	for _, name := range []string{"skill_draft", "memory_read", "memory_write"} {
		d, _ := agentcontext.Descriptor(name)
		if tools.contextAccess(context.Background(), task, name, agents.ToolExecution{Actor: task.Actor, RunID: task.ID, ExpectedCapability: &d}) == nil {
			t.Fatal("profile-free work gained", name)
		}
	}
}

func TestConversationResourceLimitsAreInspectableAndBlockRequiredAdmission(t *testing.T) {
	s := profileFixture(t)
	scope := agents.ControlScope{Workspace: s.workspaceID, Actor: "local-admin"}
	for i, required := range []bool{false, true} {
		p, err := s.agentManager.ApplyProjectCommand(scope, api.ProjectCommand{RequestID: "project" + strconv.Itoa(i), Kind: "create_project", Payload: api.ProjectCommandPayload{Name: "Project", Resources: []api.ProjectResourceReference{{Kind: "instructions", ID: "AGENTS.md", Required: required}}}})
		if err != nil {
			t.Fatal(err)
		}
		c, err := s.agentManager.ApplyConversationCommand(scope, agents.ConversationCommand{RequestID: "conversation" + strconv.Itoa(i), Kind: "create_conversation", Payload: agents.ConversationCommandPayload{ProjectID: p.TargetID, Preferences: &agents.ConversationPreferences{Model: "model"}}})
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest("POST", "/", nil)
		e, err := s.effectiveConversationPreferences(r, c.Conversation, "")
		if err != nil || len(e.Resources) != 1 || e.Resources[0].Selected || e.Resources[0].Reason == "" {
			t.Fatal(e, err)
		}
		w := httptest.NewRecorder()
		_, _, ok := s.resolveConversationRun(w, r, c.Conversation, agents.ConversationCommand{Payload: agents.ConversationCommandPayload{Input: &agents.ConversationInput{Content: "Help"}}})
		if ok == required || required && w.Code != 422 {
			t.Fatal(required, ok, w.Code, w.Body.String())
		}
	}
}
