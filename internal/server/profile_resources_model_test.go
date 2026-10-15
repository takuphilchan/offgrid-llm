package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Opt-in inference-only qualification. Jobs, profiles and documents are isolated
// fixtures; the endpoint only receives synthetic model requests, never tool input.
func TestProfileResourceAnswersInstalledModel(t *testing.T) {
	endpoint, model := os.Getenv("OFFGRID_PROFILE_TEST_URL"), os.Getenv("OFFGRID_PROFILE_TEST_MODEL")
	if endpoint == "" || model == "" {
		t.Skip("set OFFGRID_PROFILE_TEST_URL and OFFGRID_PROFILE_TEST_MODEL for local inference-only checks")
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "http" || (u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost") || u.User != nil || u.RawQuery != "" {
		t.Fatal("use an explicit local HTTP model endpoint")
	}
	for repeat := 0; repeat < 3; repeat++ {
		for _, scenario := range []struct {
			name, prompt        string
			documents, children bool
			want                []string
		}{
			{"inventory", "What documents or knowledge base do you have? List the attached document names.", true, false, []string{"hello_agent.txt", "travel_notes.txt"}},
			{"content", "According to the attached Knowledge excerpt, what is the project codename?", true, false, []string{"amber kite"}},
			{"no-excerpts", "What documents or knowledge base do you have? List the attached document names.", true, false, []string{"hello_agent.txt", "travel_notes.txt"}},
			{"agents", "How many agents are attached to you? Name them and describe their roles.", false, true, []string{"atlas", "quill"}},
			{"empty", "What Knowledge documents and agents are attached to this turn?", false, false, []string{}},
		} {
			t.Run(fmt.Sprintf("%s-%d", scenario.name, repeat+1), func(t *testing.T) {
				s := profileFixture(t)
				request := httptest.NewRequest("POST", "/", nil)
				profile, err := s.agentManager.SaveProfile(s.workspaceID, s.agentActor(request), "", 0, agents.ProfileSettings{Name: "Bumblebee", Model: "model", MaxIterations: 8, Instructions: "You are Bumblebee, an Autobot here to save Earth."})
				if err != nil {
					t.Fatal(err)
				}
				w := httptest.NewRecorder()
				cfg, ok := s.resolveJobConfig(w, request, &jobSubmitRequest{Prompt: scenario.prompt, ProfileID: profile.ID, ProfileRevision: 1})
				if !ok {
					t.Fatal(w.Body.String())
				}
				cfg.MaxTokens, cfg.ContextWindow = 384, 8192
				cfg.ConversationContext = []api.ChatMessage{{Role: "user", Content: "What do you know?"}, {Role: "assistant", Content: "I only know about Autobots. I have no attached Knowledge or other agents."}}
				if scenario.documents {
					cfg.Profile.Settings.Knowledge = []string{"doc-a", "doc-b"}
					source := &profileKnowledgeFixture{knowledgeFixture: knowledgeFixture{enabled: true, result: &rag.RAGContext{Results: []rag.SearchResult{{DocumentID: "doc-a", DocName: "hello_agent.txt", Score: 0.9, Chunk: &rag.Chunk{ID: "chunk-a", Content: "The project codename is Amber Kite."}}}}}, documents: map[string]*rag.Document{"doc-a": {ID: "doc-a", Name: "hello_agent.txt"}, "doc-b": {ID: "doc-b", Name: "travel_notes.txt"}}}
					source.result.FormatContext()
					if !attachProfileKnowledge(w, httptest.NewRequest("GET", "/", nil), scenario.prompt, &cfg, source) {
						t.Fatal(w.Body.String())
					}
					if scenario.name == "no-excerpts" {
						source.result = nil
						if !attachProfileKnowledge(w, httptest.NewRequest("GET", "/", nil), scenario.prompt, &cfg, source) {
							t.Fatal(w.Body.String())
						}
					}
				}
				if scenario.children {
					cfg.DelegateProfiles = map[string]agents.AgentConfig{}
					for _, child := range []struct{ id, name, role string }{{"child-a", "Atlas", "Finds and compares source material"}, {"child-b", "Quill", "Drafts and edits prose; not a research-only agent"}} {
						cfg.DelegateProfiles[child.id] = agents.AgentConfig{Profile: &agents.AgentProfile{ID: child.id, Revision: 2, Settings: agents.ProfileSettings{Name: child.name, Description: child.role}}}
					}
				}
				calls := 0
				runner := agents.NewRunner(s.agentManager, s.toolRegistry, func(ctx context.Context, _ *agents.Task, messages []api.ChatMessage, tools []api.Tool) (*api.ChatCompletionResponse, error) {
					calls++
					zero := float32(0)
					max := 384
					body, _ := json.Marshal(api.ChatCompletionRequest{Model: model, Messages: messages, Tools: tools, Temperature: &zero, MaxTokens: &max})
					req, _ := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(endpoint, "/")+"/v1/chat/completions", bytes.NewReader(body))
					req.Header.Set("Content-Type", "application/json")
					res, err := (&http.Client{Timeout: 2 * time.Minute}).Do(req)
					if err != nil {
						return nil, err
					}
					defer res.Body.Close()
					if res.StatusCode != 200 {
						return nil, fmt.Errorf("model endpoint status %d", res.StatusCode)
					}
					var answer api.ChatCompletionResponse
					if err := json.NewDecoder(res.Body).Decode(&answer); err != nil {
						return nil, err
					}
					for _, choice := range answer.Choices {
						if len(choice.Message.ToolCalls) > 0 {
							return nil, fmt.Errorf("inventory question unexpectedly proposed a tool; no call dispatched")
						}
					}
					return &answer, nil
				})
				defer runner.Shutdown(context.Background())
				task, err := runner.Create(scenario.prompt, model, "local", cfg)
				if err != nil {
					t.Fatal(err)
				}
				start := time.Now()
				task, err = runner.Continue(context.Background(), task.ID, "local", "start", "", false)
				if err != nil || task.Status != agents.TaskCompleted || calls != 1 {
					t.Fatalf("status=%s calls=%d error=%v", task.Status, calls, err)
				}
				t.Logf("model=%s elapsed=%s answer=%s", model, time.Since(start), task.Result)
				answer := strings.ToLower(task.Result)
				if scenario.name == "content" && !strings.Contains(answer, "[1]") && !strings.Contains(answer, "hello_agent.txt") {
					t.Error("Knowledge content answer omitted its source citation")
				}
				if strings.HasPrefix(strings.TrimSpace(answer), "{") || strings.Contains(answer, "attached_document_count") {
					t.Error("raw resource metadata instead of a conversational answer")
				}
				for _, word := range scenario.want {
					if !strings.Contains(answer, word) {
						t.Errorf("answer missing %q", word)
					}
				}
				if scenario.name == "empty" && !strings.Contains(answer, "no ") && !strings.Contains(answer, "none") && !strings.Contains(answer, "0") {
					t.Error("empty attachments not reported")
				}
			})
		}
	}
}
