package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestDefaultConversationOptionsPinBeforeQualificationAndSurviveDisable(t *testing.T) {
	for _, profile := range []string{"interactive", "extended", ""} {
		t.Run(profile, func(t *testing.T) {
			t.Setenv("OFFGRID_ENABLE_CONVERSATION_PREVIEW", "")
			s, scope, conversation := contextInboxFixture(t)
			s.config.EnableConversationPreview = config.LoadConfig().EnableConversationPreview
			s.config.DefaultModel, s.config.MaxContextSize, s.config.ChatContextSize, s.config.AdaptiveContext = "model", 8192, 4096, false
			var measured []int
			lookup := func(_ context.Context, _ string, window int) (*models.CapabilityEvidenceTarget, error) {
				measured = append(measured, window)
				return nil, nil
			}
			policy, err := inference.NewAgentToolPolicy(models.NewCapabilityEvidenceStore(t.TempDir()), lookup, lookup)
			if err != nil {
				t.Fatal(err)
			}
			s.agentToolPolicy = policy
			command := api.ConversationInputCommand{RequestID: "options-once", TargetID: conversation.ID, ExpectedRevision: conversation.Revision, Kind: "submit_input", Payload: api.ConversationInboxInput{Content: "Hello", ResponseProfile: profile, MaxTokens: 256}}
			response := controlHTTP(s, "POST", "/api/v2/conversations/"+conversation.ID+"/inputs", command)
			var receipt api.ConversationCommandReceipt
			if response.Code != 202 || json.Unmarshal(response.Body.Bytes(), &receipt) != nil {
				t.Fatal(response.Code, response.Body.String())
			}
			task, err := s.prepareConversationTurn(context.Background(), scope, conversation.ID, receipt.Input.ID)
			if err != nil {
				t.Fatal(err)
			}
			want := 8192
			if profile == "interactive" {
				want = 4096
			}
			if task.Config.ContextWindow != want || task.Config.MaxTokens != 256 || task.Config.ConversationProfile != profile || task.Config.Profile != nil {
				t.Fatal("request options not pinned", task.Config)
			}
			if len(measured) == 0 {
				t.Fatal("qualification was not consulted")
			}
			for _, window := range measured {
				if window != want {
					t.Fatal("qualification preceded response context", measured, want)
				}
			}
			s.config.EnableConversationPreview = false
			retry := controlHTTP(s, "POST", "/api/v2/conversations/"+conversation.ID+"/inputs", command)
			if retry.Code != 200 {
				t.Fatal("accepted retry blocked by disable", retry.Code, retry.Body.String())
			}
			changed := command
			changed.Payload.MaxTokens = 1024
			if response := controlHTTP(s, "POST", "/api/v2/conversations/"+conversation.ID+"/inputs", changed); response.Code != 409 {
				t.Fatal("changed retry accepted", response.Code)
			}
			current, _ := s.agentManager.Conversation(scope, conversation.ID)
			command.RequestID, command.ExpectedRevision = "new-disabled", current.Revision
			if response := controlHTTP(s, "POST", "/api/v2/conversations/"+conversation.ID+"/inputs", command); response.Code != 503 {
				t.Fatal("new input bypassed disable", response.Code)
			}
			identity := httptest.NewRecorder()
			s.handleSystemIdentity(identity, httptest.NewRequest("GET", "/api/v2/system", nil))
			var advertised struct {
				Capabilities []string `json:"capabilities"`
			}
			if json.Unmarshal(identity.Body.Bytes(), &advertised) != nil || !slices.Contains(advertised.Capabilities, "conversation-input-options-v1") || slices.Contains(advertised.Capabilities, "conversation-input-admission-v1") {
				t.Fatal("format support confused with admission", identity.Body.String())
			}
			control := api.ConversationInputControlCommand{RequestID: "stop-disabled", TargetID: receipt.Input.ID, ExpectedRevision: current.Revision, Kind: "stop"}
			if response := controlHTTP(s, "POST", "/api/v2/conversations/"+conversation.ID+"/inputs/"+receipt.Input.ID+"/commands", control); response.Code != 202 {
				t.Fatal("disabled Stop unavailable", response.Code, response.Body.String())
			}
		})
	}
}

func TestDefaultConversationSourceOnlyPreservesCaptionAndAnswer(t *testing.T) {
	for _, kind := range []string{"file", "image"} {
		t.Run(kind, func(t *testing.T) {
			f := newConversationFixture(t)
			f.s.config.EnableConversationPreview = true
			f.s.bindCanonicalConversationGuard()
			r := httptest.NewRequest("POST", "/api/v2/conversations/import", nil)
			manifest, err := f.s.migrateConversation(r, "chat")
			if err != nil {
				t.Fatal(err)
			}
			input := api.ConversationInboxInput{Content: "", Model: "model", ResponseProfile: "interactive", MaxTokens: 256}
			if kind == "file" {
				upload := uploadChatFile(t, f.s.sessionHandlers, "source.txt", "Shipment 12345 is exempt.")
				var body struct {
					Attachment sessions.Attachment `json:"attachment"`
				}
				if upload.Code != 200 || json.Unmarshal(upload.Body.Bytes(), &body) != nil {
					t.Fatal(upload.Code, upload.Body.String())
				}
				input.SourceSession, input.Attachments = "chat", []api.ConversationAttachment{{ID: body.Attachment.ID, Revision: body.Attachment.Revision}}
			} else {
				input.Images = []api.SessionImage{testSessionImage(t)}
				projector := filepath.Join(t.TempDir(), "projector-fixture")
				if err = os.WriteFile(projector, []byte("fixture only"), 0600); err != nil {
					t.Fatal(err)
				}
				metadata, _ := json.Marshal(map[string]api.ModelMetadata{"model": {ID: "model", Type: "llm", ProjectorPath: projector}})
				registry := filepath.Join(t.TempDir(), "registry.json")
				if err = os.WriteFile(registry, metadata, 0600); err != nil {
					t.Fatal(err)
				}
				if err = f.s.registry.LoadRegistry(registry); err != nil {
					t.Fatal(err)
				}
			}
			calls := 0
			f.s.agentRunner = agents.NewRunner(f.s.agentManager, f.s.toolRegistry, func(_ context.Context, task *agents.Task, messages []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
				calls++
				if task.Prompt != "" || task.Config.Profile != nil || task.WorkID != "" {
					t.Error("source caption became invented work", task.Prompt, task.WorkID)
				}
				last := messages[len(messages)-1]
				if kind == "file" && !strings.Contains(last.StringContent(), "12345") {
					t.Error("source evidence lost")
				}
				if kind == "image" {
					hasImages, err := api.ValidateChatMessages(messages)
					if err != nil || !hasImages {
						t.Error("image lost", err)
					}
				}
				return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "stop", Message: api.ChatMessage{Role: "assistant", Content: "Source explanation"}}}}, nil
			})
			current, _ := f.s.agentManager.Conversation(f.s.controlScope(r), manifest.ConversationID)
			command := api.ConversationInputCommand{RequestID: "source-only", TargetID: current.ID, ExpectedRevision: current.Revision, Kind: "submit_input", Payload: input}
			response := controlHTTP(f.s, "POST", "/api/v2/conversations/"+current.ID+"/inputs", command)
			var receipt api.ConversationCommandReceipt
			if response.Code != 202 || json.Unmarshal(response.Body.Bytes(), &receipt) != nil {
				t.Fatal(response.Code, response.Body.String())
			}
			task, err := f.s.prepareConversationTurn(context.Background(), f.s.controlScope(r), current.ID, receipt.Input.ID)
			if err != nil {
				t.Fatal(err)
			}
			completed, err := f.s.agentRunner.Continue(context.Background(), task.ID, "local-admin", "start", "", false)
			if err != nil || completed.Status != agents.TaskCompleted || calls != 1 {
				t.Fatal("source answer failed", completed, calls, err)
			}
			entry, _ := f.s.agentManager.ConversationInput(f.s.controlScope(r), current.ID, receipt.Input.ID)
			if entry.Input.Content != "" || entry.Decision == nil || entry.Decision.Kind != "answered" || entry.CreatedWorkID != "" {
				t.Fatal("source input changed meaning", entry)
			}
			restarted := agents.NewManagerWithPersistence(nil, nil, nil, f.agentDirectory)
			retained, ok := restarted.GetTask(task.ID)
			if !ok || retained.Prompt != "" || retained.Config.MaxTokens != 256 || retained.Status != agents.TaskCompleted {
				t.Fatal("restart lost source-only outcome", retained)
			}
		})
	}
}
