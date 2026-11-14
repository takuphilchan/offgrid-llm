package server

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
)

func TestConversationForegroundProfileDoesNotRequestHostAccess(t *testing.T) {
	s, scope, c := contextInboxFixture(t)
	s.config.DefaultModel = "model"
	settings := agents.ProfileSettings{Name: "Optional specialist", Model: "model", Instructions: "Use concise explanations", MaxIterations: 8, Computer: true}
	response := profileHTTP(s, "POST", "/api/v2/agent-profiles", map[string]any{"settings": settings})
	var profile agents.AgentProfile
	if response.Code != 201 || json.Unmarshal(response.Body.Bytes(), &profile) != nil {
		t.Fatal(response.Code, response.Body.String())
	}
	configured, err := s.agentManager.ApplyConversationCommand(scope, agents.ConversationCommand{RequestID: "specialize", TargetID: c.ID, ExpectedRevision: c.Revision, Kind: "configure_conversation", Payload: agents.ConversationCommandPayload{Preferences: &agents.ConversationPreferences{ProfileID: profile.ID, ProfileRevision: profile.Revision}}})
	if err != nil {
		t.Fatal(err)
	}
	input := acceptContextInput(t, s, scope, configured.TargetID, "input", "Explain this briefly")
	task, err := s.prepareConversationTurn(context.Background(), scope, c.ID, input.ID)
	if err != nil {
		t.Fatal(err)
	}
	if task.Config.Profile == nil || task.Config.Profile.Digest != profile.Digest || !strings.Contains(task.Config.SystemPrompt, settings.Instructions) || task.PendingInput != nil || task.Config.RequestedComputerScope != "" || task.Config.ComputerSession != "" || task.Config.ComputerAccessOnDemand || len(task.Config.AllowedCapabilities) != 0 {
		t.Fatal("specialization introduced host setup or lost its revision/instructions", task)
	}
}
