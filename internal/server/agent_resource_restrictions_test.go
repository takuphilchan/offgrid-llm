package server

import (
	"net/http/httptest"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
)

func TestProfileConversationRestrictionsCannotWidenAuthority(t *testing.T) {
	s := profileFixture(t)
	d, _ := s.toolRegistry.Capability("calculator")
	ref := agents.ProfileToolReference{Provider: d.Source, Tool: d.Name, SchemaRevision: d.Revision, Location: d.Location}
	for _, name := range []string{"forged_provider", "changed_schema", "wrong_location", "unselected_tool", "duplicate_tool", "unselected_knowledge", "duplicate_knowledge", "computer_escalation"} {
		t.Run(name, func(t *testing.T) {
			config := agents.DefaultAgentConfig()
			config.Profile = &agents.AgentProfile{Settings: agents.ProfileSettings{Knowledge: []string{"selected"}}}
			config.AllowedCapabilities = map[string]capabilities.Descriptor{d.Name: d}
			tools := []agents.ProfileToolReference{ref}
			restrictions := &agentResourceRestrictions{Tools: &tools}
			switch name {
			case "forged_provider":
				tools[0].Provider = "foreign"
			case "changed_schema":
				tools[0].SchemaRevision = "different"
			case "wrong_location":
				tools[0].Location = "host"
			case "unselected_tool":
				tools[0].Tool = "write_file"
			case "duplicate_tool":
				tools = append(tools, ref)
			case "unselected_knowledge":
				ids := []string{"foreign"}
				restrictions.Knowledge = &ids
			case "duplicate_knowledge":
				ids := []string{"selected", "selected"}
				restrictions.Knowledge = &ids
			case "computer_escalation":
				enabled := true
				restrictions.Computer = &enabled
			}
			w := httptest.NewRecorder()
			if restrictProfileConfig(w, restrictions, &config) || w.Code != 403 {
				t.Fatal("authority widened", name, w.Code)
			}
		})
	}
	config := agents.DefaultAgentConfig()
	config.Profile = &agents.AgentProfile{Settings: agents.ProfileSettings{Computer: true, Knowledge: []string{"selected"}}}
	config.AllowedCapabilities = map[string]capabilities.Descriptor{d.Name: d}
	tokens := []agents.ProfileToolReference{}
	documents := []string{}
	computer := false
	if !restrictProfileConfig(httptest.NewRecorder(), &agentResourceRestrictions{Tools: &tokens, Knowledge: &documents, Computer: &computer}, &config) {
		t.Fatal("narrowing rejected")
	}
	if len(config.AllowedCapabilities) != 0 || len(selectedProfileKnowledge(config)) != 0 || !config.RestrictComputer || agents.ProfileAllowsCapability(config, d.Name, d) {
		t.Fatal("empty selection inherited broader access")
	}
}

func TestProfileComputerDefaultsToOnDemandDesktopWithoutGrant(t *testing.T) {
	s := profileFixture(t)
	p, err := s.agentManager.SaveProfile(s.workspaceID, "local-admin", "", 0, agents.ProfileSettings{Name: "Desktop", Model: "model", Computer: true, MaxIterations: 5})
	if err != nil {
		t.Fatal(err)
	}
	for _, restricted := range []bool{false, true} {
		req := jobSubmitRequest{Prompt: "hello", ProfileID: p.ID, ProfileRevision: p.Revision}
		if restricted {
			disabled := false
			req.Restrictions = &agentResourceRestrictions{Computer: &disabled}
		}
		w := httptest.NewRecorder()
		config, ok := s.resolveJobConfig(w, httptest.NewRequest("POST", "/api/v2/jobs", nil), &req)
		if !ok {
			t.Fatal(w.Code, w.Body.String())
		}
		if config.ComputerSession != "" || !config.ComputerAccessOnDemand {
			t.Fatal("profile selection granted access")
		}
		if !restricted && (config.RequestedComputerScope != "desktop" || config.RequestedComputerApprovalMode != "full_task") {
			t.Fatal("profile fell back to legacy window flow")
		}
		if restricted && config.RequestedComputerScope != "" {
			t.Fatal("disabled computer requested desktop")
		}
	}
}
