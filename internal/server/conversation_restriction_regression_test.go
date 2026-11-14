package server

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
)

func TestDisabledUnavailableProfileResourceDoesNotBlockConversation(t *testing.T) {
	s := profileFixture(t)
	d, _ := s.toolRegistry.Capability("calculator")
	p, err := s.agentManager.SaveProfile(s.workspaceID, "local-admin", "", 0, agents.ProfileSettings{
		Name: "General", Model: "model", MaxIterations: 10,
		Tools: []agents.ProfileToolReference{{Provider: d.Source, Tool: d.Name, SchemaRevision: d.Revision, Location: d.Location}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.toolRegistry.DisableTool("calculator"); err != nil {
		t.Fatal(err)
	}
	none := []agents.ProfileToolReference{}
	req := jobSubmitRequest{Prompt: "Hello", ProfileID: p.ID, ProfileRevision: p.Revision, Restrictions: &agentResourceRestrictions{Tools: &none}}
	w := httptest.NewRecorder()
	config, ok := s.resolveJobConfig(w, httptest.NewRequest("POST", "/api/v2/jobs", nil), &req)
	if !ok {
		t.Fatalf("disabled resource blocked admission: %d %s", w.Code, w.Body.String())
	}
	if _, allowed := config.AllowedCapabilities[d.Name]; allowed {
		t.Fatal("disabled tool regained authority")
	}
	if config.Profile.Digest != p.Digest || len(config.Profile.Settings.Tools) != 1 {
		t.Fatal("restriction rewrote immutable profile")
	}
	req.Restrictions = nil
	w = httptest.NewRecorder()
	if _, ok = s.resolveJobConfig(w, httptest.NewRequest("POST", "/api/v2/jobs", nil), &req); ok {
		t.Fatal("unrestricted unavailable reference silently admitted")
	}
}

func TestConversationRestrictionsSurviveReloadAndProfileRevision(t *testing.T) {
	f := newConversationFixture(t)
	d, _ := f.s.toolRegistry.Capability("calculator")
	settings := agents.ProfileSettings{Name: "Assistant", Model: "model", MaxIterations: 8, Tools: []agents.ProfileToolReference{{Provider: d.Source, Tool: d.Name, SchemaRevision: d.Revision, Location: d.Location}}}
	p, err := f.s.agentManager.SaveProfile(f.s.workspaceID, "local-admin", f.request.Agent.ID, f.request.Agent.Revision, settings)
	if err != nil {
		t.Fatal(err)
	}
	none := []agents.ProfileToolReference{}
	noComputer := false
	f.request.Agent = &sessions.AgentSelection{ID: p.ID, Revision: p.Revision}
	f.request.Restrictions = &agentResourceRestrictions{Tools: &none, Computer: &noComputer}
	if w := f.submit(f.request); w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	first := f.waitComplete(t)
	if err = f.s.agentRunner.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	f.s.agentManager = agents.NewManagerWithPersistence(nil, nil, nil, f.agentDirectory)
	if err = f.s.agentManager.StorageError(); err != nil {
		t.Fatal(err)
	}
	f.installRunner(t)
	saved, _ := f.s.agentManager.GetTask(first.Turn.JobID)
	configuration := saved.Config.ConversationConfiguration
	if configuration == nil || configuration.Version != 1 || configuration.Restrictions.Tools == nil || len(*configuration.Restrictions.Tools) != 0 || configuration.Restrictions.Computer == nil || *configuration.Restrictions.Computer {
		t.Fatal("reload lost explicit restrictions")
	}
	if taskResponse(saved)["conversation_configuration"] == nil {
		t.Fatal("inspection omitted saved configuration")
	}
	settings.Instructions = "New revision; still respect conversation restrictions."
	p, err = f.s.agentManager.SaveProfile(f.s.workspaceID, "local-admin", p.ID, p.Revision, settings)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.s.toolRegistry.DisableTool("calculator"); err != nil {
		t.Fatal(err)
	}
	next := f.request
	next.RequestID = "conversation-request-2"
	next.Agent = &sessions.AgentSelection{ID: p.ID, Revision: p.Revision}
	next.Restrictions = &agentResourceRestrictions{} // omitted fields must inherit
	if w := f.submit(next); w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	second := f.waitComplete(t)
	updated, _ := f.s.agentManager.GetTask(second.Turn.JobID)
	if updated.ID == saved.ID || updated.Config.Profile.Revision != p.Revision {
		t.Fatal("new specialization did not create its requested turn")
	}
	if _, allowed := updated.Config.AllowedCapabilities[d.Name]; allowed {
		t.Fatal("profile revision reopened disabled tool authority")
	}
	if updated.Config.ConversationConfiguration.Restrictions.Computer == nil || *updated.Config.ConversationConfiguration.Restrictions.Computer {
		t.Fatal("profile revision lost computer restriction")
	}
	// The original idempotency result must still use its original pin, even
	// though the conversation now has a newer accepted configuration.
	if w := f.submit(f.request); w.Code != 202 {
		t.Fatal("retry resolved newer restrictions", w.Code, w.Body.String())
	}
	if len(f.s.agentManager.ListTasks()) != 2 {
		t.Fatal("retry created another execution")
	}
}

func TestDisabledUnavailableChildResourceDoesNotBlockConversation(t *testing.T) {
	s := profileFixture(t)
	d, _ := s.toolRegistry.Capability("calculator")
	refs := []agents.ProfileToolReference{{Provider: d.Source, Tool: d.Name, SchemaRevision: d.Revision, Location: d.Location}}
	child, err := s.agentManager.SaveProfile(s.workspaceID, "local-admin", "", 0, agents.ProfileSettings{Name: "Research", Model: "model", MaxIterations: 4, Tools: refs})
	if err != nil {
		t.Fatal(err)
	}
	parent, err := s.agentManager.SaveProfile(s.workspaceID, "local-admin", "", 0, agents.ProfileSettings{Name: "General", Model: "model", MaxIterations: 12, Tools: refs, Delegates: []string{child.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.toolRegistry.DisableTool("calculator"); err != nil {
		t.Fatal(err)
	}
	none := []agents.ProfileToolReference{}
	req := jobSubmitRequest{Prompt: "Hello", ProfileID: parent.ID, ProfileRevision: parent.Revision, Restrictions: &agentResourceRestrictions{Tools: &none}}
	w := httptest.NewRecorder()
	config, ok := s.resolveJobConfig(w, httptest.NewRequest("POST", "/api/v2/jobs", nil), &req)
	if !ok {
		t.Fatalf("disabled child resource blocked conversation: %s", w.Body.String())
	}
	selected := config.DelegateProfiles[child.ID]
	if _, allowed := selected.AllowedCapabilities[d.Name]; allowed {
		t.Fatal("child restriction widened authority")
	}
	if selected.Profile == nil || selected.Profile.Digest != child.Digest {
		t.Fatal("child immutable identity changed")
	}
}
