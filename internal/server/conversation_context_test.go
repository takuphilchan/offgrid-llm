package server

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestConversationContextRetainsFailedAttemptAndCorrection(t *testing.T) {
	f := newConversationFixture(t)
	if err := f.s.agentRunner.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	r := agents.NewRunner(f.s.agentManager, f.s.toolRegistry, func(context.Context, *agents.Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		return nil, errors.New("fixture model unavailable")
	})
	defer r.Shutdown(context.Background())
	cfg := agents.DefaultAgentConfig()
	cfg.TaskFirst = true
	origin := agents.ConversationOrigin{Workspace: f.s.workspaceID, Conversation: "chat", TurnID: "history-turn"}
	task, _, err := r.CreateHeldRequest("Inspect only; retain dirty files", "model", "local-admin", cfg, "history-request", "digest", origin)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.ReleaseConversationAdmission(task.ID, task.Actor, "digest", origin); err != nil {
		t.Fatal(err)
	}
	if _, err = r.Pause(task.ID, task.Actor, false); err != nil {
		t.Fatal(err)
	}
	if _, err = r.Steer(task.ID, task.Actor, "correction-request", "Use the corrected source count of 94"); err != nil {
		t.Fatal(err)
	}
	if _, err = r.Continue(context.Background(), task.ID, task.Actor, "resume", "", false); err != nil {
		t.Fatal(err)
	}
	conversation := &sessions.Session{Name: "chat", Jobs: []sessions.ConversationJobLink{{TurnID: origin.TurnID, JobID: task.ID}}}
	var config agents.AgentConfig
	if err := f.s.assembleConversationContext(conversation, task.Actor, &config); err != nil {
		t.Fatal(err)
	}
	var history strings.Builder
	for _, message := range config.ConversationContext {
		history.WriteString(message.StringContent())
	}
	for _, want := range []string{"retain dirty files", "corrected source count of 94", "failed", "must not be represented as successful completion"} {
		if !strings.Contains(history.String(), want) {
			t.Fatalf("history lost %q: %s", want, history.String())
		}
	}
	if err := f.s.assembleConversationContext(conversation, "another-actor", &agents.AgentConfig{}); err == nil {
		t.Fatal("cross-actor linked history accepted")
	}
}

func TestConversationContextDoesNotDropHistoryAfterLongAnswer(t *testing.T) {
	f := newConversationFixture(t)
	conversation := &sessions.Session{Name: "chat", Messages: []sessions.Message{{Role: "user", Content: "Preserve my constraints"}, {Role: "assistant", Content: strings.Repeat("x", 30000)}}}
	var config agents.AgentConfig
	if err := f.s.assembleConversationContext(conversation, "local-admin", &config); err != nil {
		t.Fatal(err)
	}
	if len(config.ConversationContext) != 2 || config.ConversationContext[0].StringContent() != "Preserve my constraints" || len(config.ConversationContext[1].StringContent()) != 30000 {
		t.Fatal("history cut off before model accounting")
	}
}

func TestAgentHistoryDisclosesUnavailableImageWithoutDroppingText(t *testing.T) {
	f := newConversationFixture(t)
	conversation := &sessions.Session{Name: "chat", Messages: []sessions.Message{{Role: "user", Content: "Preserve the constraint", Images: []api.SessionImage{testSessionImage(t)}}}}
	var config agents.AgentConfig
	if err := f.s.assembleConversationContext(conversation, "local-admin", &config); err != nil {
		t.Fatal(err)
	}
	content := config.ConversationContext[0].StringContent()
	if !strings.Contains(content, "Preserve the constraint") || !strings.Contains(content, unavailableHistoricalImages) || strings.Contains(content, "data:image") {
		t.Fatal("agent history lost constraint or claimed unseen image access")
	}
}

func TestCanonicalContextRevocationRetainsUnrelatedHistory(t *testing.T) {
	f := newConversationFixture(t)
	conversation := &sessions.Session{Name: "chat", Messages: []sessions.Message{
		{Role: "user", Content: "Preserve my independent constraint"},
		{Role: "assistant", Content: "SECRET revoked excerpt", KnowledgeDocuments: []string{"revoked-source"}},
		{Role: "assistant", Content: "Independent authorized answer"},
	}}
	var config agents.AgentConfig
	if err := f.s.assembleConversationContextPolicy(conversation, "local-admin", &config, true); err != nil {
		t.Fatal(err)
	}
	var history strings.Builder
	for _, m := range config.ConversationContext {
		history.WriteString(m.StringContent())
	}
	if strings.Contains(history.String(), "SECRET") || len(config.KnowledgeDependencies) != 0 {
		t.Fatal("revoked excerpt or dependency reused", history.String())
	}
	for _, want := range []string{"independent constraint", "Independent authorized answer", "access changed"} {
		if !strings.Contains(history.String(), want) {
			t.Fatal("authorized context missing", want)
		}
	}
}
