package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/internal/users"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type selectedKnowledgeFixture struct {
	knowledgeFixture
	selected   []string
	broadCalls int
}

type selectedProfileSource struct{ *selectedKnowledgeFixture }

func (f selectedProfileSource) GetDocument(id string) *rag.Document {
	return &rag.Document{ID: id, Name: id, IndexStatus: "ready", ChunkCount: 1}
}
func (f selectedProfileSource) Search(context.Context, string, rag.SearchOptions) (*rag.RAGContext, error) {
	return nil, errors.New("legacy search must not be used for explicit sources")
}

func TestSelectedKnowledgeAgentAndChildUseEffectiveScope(t *testing.T) {
	for _, selected := range [][]string{{"doc-a", "doc-b"}, {"doc-a"}, {}} {
		fixture := sourceScopeFixture()
		config := agents.DefaultAgentConfig()
		config.Profile = &agents.AgentProfile{Settings: agents.ProfileSettings{Knowledge: []string{"doc-a", "doc-b", "private"}}}
		config.KnowledgeSelection = &selected
		request := httptest.NewRequest("GET", "/", nil)
		request = request.WithContext(withKnowledgeScope(request.Context(), []string{"doc-a", "doc-b"}, nil))
		w := httptest.NewRecorder()
		if !attachProfileKnowledge(w, request, "question", &config, selectedProfileSource{fixture}) {
			t.Fatal(w.Body.String())
		}
		if len(selected) == 0 {
			if fixture.selected != nil || config.KnowledgeContext != "" {
				t.Fatal("empty child scope inherited parent sources")
			}
		} else if !reflect.DeepEqual(fixture.selected, selected) || len(config.KnowledgeSources) != 1 || config.KnowledgeSources[0].DocumentID != "doc-a" {
			t.Fatal("effective selection or evidence lost")
		}
	}
}

func (f *selectedKnowledgeFixture) EnhancePrompt(context.Context, string) (string, *rag.RAGContext, error) {
	f.broadCalls++
	return "", f.result, f.err
}
func (f *selectedKnowledgeFixture) EnhancePromptForDocuments(_ context.Context, _ string, ids []string) (string, *rag.RAGContext, error) {
	f.selected = append([]string(nil), ids...)
	return "", f.result, f.err
}
func sourceScopeFixture() *selectedKnowledgeFixture {
	result := &rag.RAGContext{Results: []rag.SearchResult{{DocumentID: "doc-a", DocName: "Notes", Chunk: &rag.Chunk{ID: "chunk-a", Content: "café 日本語 — evidence"}}}}
	result.FormatContext()
	return &selectedKnowledgeFixture{knowledgeFixture: knowledgeFixture{enabled: true, result: result}}
}

func TestKnowledgeSelectionValidationAndAgentRestrictionIntersection(t *testing.T) {
	for _, tc := range []struct {
		body  string
		valid bool
	}{
		{`{}`, true},
		{`{"use_knowledge_base":true,"knowledge_documents":["a"]}`, true},
		{`{"knowledge_documents":["a"]}`, false},
		{`{"use_knowledge_base":true,"knowledge_documents":[]}`, false},
		{`{"use_knowledge_base":true,"knowledge_documents":["a","a"]}`, false},
		{`{"use_knowledge_base":true,"knowledge_documents":[" a"]}`, false},
		{`{"use_knowledge_base":true,"knowledge_documents":["a"],"agent":{"id":"profile-a","revision":1},"restrictions":{"knowledge":["a","b"]}}`, true},
		{`{"use_knowledge_base":true,"knowledge_documents":["a"],"agent":{"id":"profile-a","revision":1},"restrictions":{"knowledge":["b"]}}`, false},
	} {
		var req sessionGenerateRequest
		if err := json.Unmarshal([]byte(tc.body), &req); err != nil {
			t.Fatal(err)
		}
		if err := validateKnowledgeSelection(&req); (err == nil) != tc.valid {
			t.Errorf("%s: %v", tc.body, err)
		}
		if tc.valid && req.Agent != nil && !reflect.DeepEqual(*req.Restrictions.Knowledge, req.KnowledgeDocuments) {
			t.Fatal("restriction was not narrowed")
		}
	}
}

func TestSelectedKnowledgeNeverFallsBackAndCapturesExactEvidence(t *testing.T) {
	for _, failure := range []string{"", "missing source", "wrong source", "unsupported retriever", "no matches"} {
		t.Run(failure, func(t *testing.T) {
			fixture := sourceScopeFixture()
			var source knowledgeRetriever = fixture
			switch failure {
			case "missing source":
				fixture.err = errors.New("private source unavailable")
			case "wrong source":
				fixture.result.Results[0].DocumentID = "private-other-document"
			case "unsupported retriever":
				source = fixture.knowledgeFixture
			case "no matches":
				fixture.result = &rag.RAGContext{}
			}
			var passages []sessions.KnowledgePassage
			req := &api.ChatCompletionRequest{UseKnowledgeBase: boolPointer(true), Messages: []api.ChatMessage{{Role: "user", Content: "question"}}}
			err := injectKnowledge(withKnowledgeScope(context.Background(), []string{"doc-a"}, &passages), req, source)
			if fixture.broadCalls != 0 {
				t.Fatal("selected retrieval broadened")
			}
			if failure != "" {
				if err == nil || len(passages) != 0 || len(req.Messages) != 1 {
					t.Fatalf("unsafe failure: %v %+v", err, req)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(fixture.selected, []string{"doc-a"}) || len(passages) != 1 {
				t.Fatalf("scope/evidence lost: %+v %+v", fixture.selected, passages)
			}
			if passages[0].Content != "café 日本語 — evidence" || passages[0].ContentHash != rag.GenerateContentHash(passages[0].Content) {
				t.Fatal("evidence changed")
			}
		})
	}
}

func TestSelectedKnowledgeSurvivesAllSessionTransports(t *testing.T) {
	for _, transport := range []string{"legacy", "stream", "durable"} {
		t.Run(transport, func(t *testing.T) {
			h := newStreamingSessions(t)
			defer h.closeTurns()
			fixture := sourceScopeFixture()
			calls := 0
			h.streamer = func(ctx context.Context, req *api.ChatCompletionRequest, _ string, _ func(ChatStreamEvent) error) (ChatStreamResult, error) {
				calls++
				if err := injectKnowledge(ctx, req, fixture); err != nil {
					return ChatStreamResult{}, err
				}
				return ChatStreamResult{Answer: "Answer [1]", FinishReason: "stop"}, nil
			}
			h.completer = func(ctx context.Context, _ string, messages []api.ChatMessage, knowledge bool) (string, error) {
				calls++
				err := injectKnowledge(ctx, &api.ChatCompletionRequest{Messages: messages, UseKnowledgeBase: &knowledge}, fixture)
				return "Answer [1]", err
			}
			body := `{"content":"question","use_knowledge_base":true,"knowledge_documents":["doc-a"]`
			if transport != "legacy" {
				body += `,"stream":true`
			}
			if transport == "durable" {
				body += `,"durable":true,"request_id":"source-turn"`
			}
			body += `}`
			request := func(body string) *httptest.ResponseRecorder {
				w := httptest.NewRecorder()
				h.HandleSessions(w, httptest.NewRequest("POST", "/v1/sessions/chat/generate", strings.NewReader(body)))
				return w
			}
			response := request(body)
			if response.Code != 200 || !strings.Contains(response.Body.String(), `"knowledge_sources"`) {
				t.Fatalf("%d %s", response.Code, response.Body.String())
			}
			saved, err := h.manager.Load("chat")
			if err != nil || len(saved.Messages) != 2 || len(saved.Messages[1].KnowledgeSources) != 1 {
				t.Fatalf("evidence not persisted: %+v %v", saved, err)
			}
			if saved.Messages[1].KnowledgeSources[0].Content != "café 日本語 — evidence" {
				t.Fatal("stored evidence changed")
			}
			if transport == "durable" {
				if !reflect.DeepEqual(saved.Turn.KnowledgeDocuments, []string{"doc-a"}) {
					t.Fatal("durable scope lost")
				}
				request(body)
				if calls != 1 {
					t.Fatal("replayed generation")
				}
				if response := request(strings.Replace(body, "doc-a", "doc-b", 1)); response.Code != 409 {
					t.Fatalf("changed source reused identity: %d", response.Code)
				}
			}
		})
	}
}

func TestSelectedKnowledgeDoesNotGrantRetrievalPermission(t *testing.T) {
	h := newStreamingSessions(t)
	h.requireAuth = true
	store := users.NewUserStore("")
	_, key, err := store.CreateUser("viewer", "test-password", users.RoleViewer)
	if err != nil {
		t.Fatal(err)
	}
	auth := users.NewMiddleware(store)
	auth.SetRequireAuth(true)
	for _, suffix := range []string{"", `,"stream":true`, `,"stream":true,"durable":true,"request_id":"scoped-test"`} {
		request := httptest.NewRequest("POST", "/v1/sessions/chat/generate", strings.NewReader(`{"content":"question","use_knowledge_base":true,"knowledge_documents":["doc-a"]`+suffix+`}`))
		request.Header.Set("Authorization", "Bearer "+key)
		w := httptest.NewRecorder()
		auth.Wrap(http.HandlerFunc(h.HandleSessions)).ServeHTTP(w, request)
		if w.Code != http.StatusForbidden {
			t.Fatalf("source reference granted authority: %d %s", w.Code, w.Body.String())
		}
	}
}
