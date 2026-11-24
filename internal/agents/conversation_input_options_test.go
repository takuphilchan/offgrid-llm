package agents

import (
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestConversationInputOptionsAndSourceOnlyBoundaries(t *testing.T) {
	command := api.ConversationInputCommand{RequestID: "request", TargetID: "conversation", ExpectedRevision: 1, Kind: "submit_input", Payload: api.ConversationInboxInput{Content: "", SourceSession: "source", Attachments: []api.ConversationAttachment{{ID: "file-source", Revision: strings.Repeat("a", 64)}}, ResponseProfile: "interactive", MaxTokens: 256}}
	if err := validateConversationInput(command); err != nil {
		t.Fatal(err)
	}
	for _, intent := range []string{"correction", "new_work", "answer"} {
		bad := command
		bad.Payload.Intent = intent
		if validateConversationInput(bad) == nil {
			t.Fatal("textless control accepted", intent)
		}
	}
	for _, change := range []func(*api.ConversationInboxInput){
		func(p *api.ConversationInboxInput) { p.Attachments = nil },
		func(p *api.ConversationInboxInput) { p.MaxTokens = 4097 },
		func(p *api.ConversationInboxInput) { p.MaxTokens = -1 },
		func(p *api.ConversationInboxInput) { p.ResponseProfile = "unknown" },
	} {
		bad := command
		change(&bad.Payload)
		if validateConversationInput(bad) == nil {
			t.Fatal("invalid input accepted", bad)
		}
	}
	entry := &api.ConversationInboxEntry{Input: command.Payload}
	if validateConversationDecision(entry, &api.ConversationDecision{Kind: "new_work", ExpectedResult: "Invented objective"}) == nil {
		t.Fatal("source-only caption acquired a work objective")
	}
	if validateConversationDecision(entry, &api.ConversationDecision{Kind: "answered"}) != nil {
		t.Fatal("source-only answer unavailable")
	}
	config := DefaultAgentConfig()
	config.InputAttachments = command.Payload.Attachments
	config.InputEvidence = "Authorized source excerpt"
	if _, err := prepareConversationRun("", "model", "owner", config, nil); err == nil {
		t.Fatal("generic empty job broadened")
	}
	old := command
	old.Payload.Content = "Question"
	old.Payload.Attachments = nil
	old.Payload.SourceSession = ""
	old.Payload.ResponseProfile = ""
	old.Payload.MaxTokens = 0
	changed := old
	changed.Payload.MaxTokens = 256
	if conversationInputDigest(old) == conversationInputDigest(changed) {
		t.Fatal("output option omitted from durable identity")
	}
}
