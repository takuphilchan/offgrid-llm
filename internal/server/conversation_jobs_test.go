package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/internal/users"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type conversationFixture struct {
	s              *Server
	request        sessionGenerateRequest
	calls          atomic.Int32
	directory      string
	agentDirectory string
}

func newConversationFixture(t *testing.T) *conversationFixture {
	t.Helper()
	s := profileFixture(t)
	f := &conversationFixture{s: s, directory: t.TempDir(), agentDirectory: t.TempDir()}
	s.agentManager = agents.NewManagerWithPersistence(nil, nil, nil, f.agentDirectory)
	s.sessionHandlers = NewSessionHandlers(f.directory)
	s.sessionHandlers.agentServer = s
	if _, err := s.sessionHandlers.manager.WithAccess(sessions.Access{All: true}).Create("chat", "model"); err != nil {
		t.Fatal(err)
	}
	p, err := s.agentManager.SaveProfile(s.workspaceID, "local-admin", "", 0, agents.ProfileSettings{Name: "Assistant", Model: "model", Instructions: "Be concise.", MaxIterations: 8})
	if err != nil {
		t.Fatal(err)
	}
	f.request = sessionGenerateRequest{Content: "hello", RequestID: "conversation-request-1", Agent: &sessions.AgentSelection{ID: p.ID, Revision: p.Revision}}
	f.installRunner(t)
	return f
}

func (f *conversationFixture) installRunner(t *testing.T) {
	f.s.agentRunner = agents.NewRunner(f.s.agentManager, f.s.toolRegistry, func(_ context.Context, task *agents.Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		f.calls.Add(1)
		if task.Origin == nil || task.AdmissionHeld {
			t.Error("unlinked dispatch")
		}
		conversation, err := f.s.sessionHandlers.manager.Load("chat")
		if err != nil || conversation.Turn == nil || conversation.Turn.JobID != task.ID {
			t.Error("dispatch before durable session association", err)
		}
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "stop", Message: api.ChatMessage{Role: "assistant", Content: "Hello from your agent."}}}}, nil
	})
	f.s.agentRunner.Observer = f.s.projectConversationTask
	runner := f.s.agentRunner
	t.Cleanup(func() { _ = runner.Shutdown(context.Background()) })
}

func (f *conversationFixture) submit(req sessionGenerateRequest) *httptest.ResponseRecorder {
	body, _ := json.Marshal(req)
	w := httptest.NewRecorder()
	f.s.sessionHandlers.HandleSessionGenerate(w, httptest.NewRequest("POST", "/v1/sessions/chat/generate", strings.NewReader(string(body))), "chat")
	return w
}

func (f *conversationFixture) waitComplete(t *testing.T) *sessions.Session {
	t.Helper()
	until := time.Now().Add(5 * time.Second)
	for time.Now().Before(until) {
		conversation, err := f.s.sessionHandlers.manager.Load("chat")
		if err == nil && conversation.Turn != nil && conversation.Turn.Status == "completed" {
			return conversation
		}
		time.Sleep(5 * time.Millisecond)
	}
	saved, loadErr := f.s.sessionHandlers.manager.Load("chat")
	for _, task := range f.s.agentManager.ListTasks() {
		t.Logf("unattached job: status=%s error_code=%s error=%q steps=%d", task.Status, task.ErrorCode, task.Error, len(task.Steps))
	}
	t.Fatalf("conversation result was not attached: session=%+v load=%v jobs=%+v", saved, loadErr, f.s.agentManager.ListTasks())
	return nil
}

func TestConversationAdmissionFaultBoundariesAndConcurrentRetry(t *testing.T) {
	for _, stage := range []string{"intent_saved", "turn_saved", "job_saved", "intent_bound", "turn_linked", "job_released"} {
		t.Run(stage, func(t *testing.T) {
			f := newConversationFixture(t)
			var injected atomic.Bool
			f.s.sessionHandlers.admissionFault = func(current string) error {
				if current == stage && injected.CompareAndSwap(false, true) {
					return errors.New("injected persistence boundary")
				}
				return nil
			}
			w := f.submit(f.request)
			if w.Code < 400 || f.calls.Load() != 0 {
				t.Fatal("dispatched through failed admission", w.Code, f.calls.Load())
			}
			var wg sync.WaitGroup
			ids := make(chan string, 4)
			for i := 0; i < 4; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					w := f.submit(f.request)
					if w.Code != 202 {
						t.Error(w.Code, w.Body.String())
						return
					}
					ids <- w.Header().Get("Location")
				}()
			}
			wg.Wait()
			close(ids)
			id := ""
			for current := range ids {
				if id != "" && id != current {
					t.Fatal("duplicate canonical job")
				}
				id = current
			}
			conversation := f.waitComplete(t)
			if f.calls.Load() != 1 || len(conversation.Messages) != 2 || len(conversation.Jobs) != 1 {
				t.Fatal("duplicate work or exchange", f.calls.Load(), conversation)
			}
			conflict := f.request
			conflict.Content = "different prompt"
			if w := f.submit(conflict); w.Code != 409 {
				t.Fatal("different payload reused identity", w.Code)
			}
		})
	}
}

func TestConversationFinalWriteRecoveryNeverReruns(t *testing.T) {
	for _, stage := range []string{"before_final_write", "after_final_write"} {
		t.Run(stage, func(t *testing.T) {
			f := newConversationFixture(t)
			var fail atomic.Bool
			fail.Store(true)
			f.s.sessionHandlers.admissionFault = func(current string) error {
				if current == stage && fail.Load() {
					return errors.New("lost final write acknowledgment")
				}
				return nil
			}
			if w := f.submit(f.request); w.Code != 202 {
				t.Fatal(w.Code, w.Body.String())
			}
			until := time.Now().Add(5 * time.Second)
			for time.Now().Before(until) {
				tasks := f.s.agentManager.ListTasks()
				if len(tasks) == 1 && tasks[0].Status == agents.TaskCompleted && !f.s.agentRunner.WorkerActive(tasks[0].ID) {
					break
				}
				time.Sleep(5 * time.Millisecond)
			}
			w := httptest.NewRecorder()
			f.s.sessionHandlers.handleTurn(w, httptest.NewRequest("GET", "/v1/sessions/chat/turn", nil), "chat", "turn")
			if w.Code != 200 || f.calls.Load() != 1 {
				t.Fatal("observation replayed", w.Code, f.calls.Load())
			}
			fail.Store(false)
			for i := 0; i < 2; i++ {
				w = httptest.NewRecorder()
				f.s.sessionHandlers.handleTurn(w, httptest.NewRequest("POST", "/v1/sessions/chat/turn/recover", strings.NewReader(`{"id":"conversation-request-1"}`)), "chat", "turn/recover")
				if w.Code != 200 {
					t.Fatal(w.Code, w.Body.String())
				}
			}
			conversation := f.waitComplete(t)
			if len(conversation.Messages) != 2 || f.calls.Load() != 1 || conversation.Messages[1].JobID == "" {
				t.Fatal("recovery duplicated/lost output")
			}
		})
	}
}

func TestConversationIntentPinsConfigurationBeforeJobCreation(t *testing.T) {
	f := newConversationFixture(t)
	var failed atomic.Bool
	f.s.sessionHandlers.admissionFault = func(stage string) error {
		if stage == "intent_saved" && failed.CompareAndSwap(false, true) {
			return errors.New("crash before job creation")
		}
		return nil
	}
	if w := f.submit(f.request); w.Code < 400 {
		t.Fatal(w.Code)
	}
	profile, err := f.s.agentManager.GetProfile(f.s.workspaceID, "local-admin", f.request.Agent.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	profile.Settings.Instructions = "Changed after admission"
	if _, err := f.s.agentManager.SaveProfile(f.s.workspaceID, "local-admin", profile.ID, 1, profile.Settings); err != nil {
		t.Fatal(err)
	}
	if w := f.submit(f.request); w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	f.waitComplete(t)
	task := f.s.agentManager.ListTasks()[0]
	if task.Config.Profile.Revision != 1 || task.Config.Instructions != "Be concise." {
		t.Fatal("retry resolved mutable configuration")
	}
}

func TestConversationDeleteRetainsMinimalDeduplication(t *testing.T) {
	f := newConversationFixture(t)
	if w := f.submit(f.request); w.Code != 202 {
		t.Fatal(w.Code)
	}
	f.waitComplete(t)
	w := httptest.NewRecorder()
	f.s.sessionHandlers.HandleSessionDelete(w, httptest.NewRequest("DELETE", "/v1/sessions/chat?work=retain", nil), "chat")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	i, err := f.s.agentManager.ConversationIntent(f.s.workspaceID, "local-admin", conversationRequestID(f.s.workspaceID, f.request.RequestID))
	if err != nil || i == nil || !i.Detached || i.Prompt != "" || i.Config.Profile != nil || i.Origin.Conversation != "" {
		t.Fatal("deletion retained admission content", i, err)
	}
	if _, err := f.s.sessionHandlers.manager.WithAccess(sessions.Access{All: true}).Create("chat", "model"); err != nil {
		t.Fatal(err)
	}
	if w := f.submit(f.request); w.Code != 409 {
		t.Fatal("deleted history allowed replay", w.Code)
	}
	if len(f.s.agentManager.ListTasks()) != 1 || f.calls.Load() != 1 {
		t.Fatal("retained work orphaned or replayed")
	}
}

func TestConversationRestartRepairsBookkeepingWithoutDispatch(t *testing.T) {
	for _, stage := range []string{"intent_saved", "turn_saved", "job_saved", "intent_bound", "turn_linked", "job_released"} {
		t.Run(stage, func(t *testing.T) {
			f := newConversationFixture(t)
			f.s.sessionHandlers.admissionFault = func(current string) error {
				if current == stage {
					return errors.New("crash at admission boundary")
				}
				return nil
			}
			if w := f.submit(f.request); w.Code < 400 {
				t.Fatal(w.Code)
			}
			if err := f.s.agentRunner.Shutdown(context.Background()); err != nil {
				t.Fatal(err)
			}
			f.s.agentManager = agents.NewManagerWithPersistence(nil, nil, nil, f.agentDirectory)
			if err := f.s.agentManager.StorageError(); err != nil {
				t.Fatal(err)
			}
			f.s.sessionHandlers = NewSessionHandlers(f.directory)
			f.s.sessionHandlers.agentServer = f.s
			f.installRunner(t)
			for i := 0; i < 2; i++ {
				if err := f.s.recoverConversationAdmissions(); err != nil {
					t.Fatal(err)
				}
			}
			jobs := f.s.agentManager.ListTasks()
			if len(jobs) != 1 || jobs[0].AdmissionHeld || jobs[0].Status != agents.TaskInterrupted || f.calls.Load() != 0 {
				t.Fatalf("restart executed or duplicated work: %+v calls=%d", jobs, f.calls.Load())
			}
			conversation, err := f.s.sessionHandlers.manager.Load("chat")
			if err != nil || conversation.Turn.JobID != jobs[0].ID || len(conversation.Jobs) != 1 {
				t.Fatal(conversation, err)
			}
			// Even a retried original submission must not turn restart recovery
			// into implicit resume. Only the canonical resume command may do so.
			if w := f.submit(f.request); w.Code != 202 {
				t.Fatal(w.Code, w.Body.String())
			}
			if f.calls.Load() != 0 {
				t.Fatal("retry resumed interrupted work")
			}
		})
	}
}

func TestConversationConcurrentDifferentPayloadHasOneWinner(t *testing.T) {
	f := newConversationFixture(t)
	other := f.request
	other.Content = "A different request with the same identity"
	results := make(chan int, 2)
	for _, req := range []sessionGenerateRequest{f.request, other} {
		go func(req sessionGenerateRequest) { results <- f.submit(req).Code }(req)
	}
	a, b := <-results, <-results
	if !((a == 202 && b == 409) || (a == 409 && b == 202)) {
		t.Fatal(a, b)
	}
	f.waitComplete(t)
	if f.calls.Load() != 1 || len(f.s.agentManager.ListTasks()) != 1 {
		t.Fatal("conflict dispatched twice")
	}
}

func TestConversationActiveDeletionRequiresChoiceAndConfirmedStop(t *testing.T) {
	for _, choice := range []string{"retain", "stop"} {
		t.Run(choice, func(t *testing.T) {
			f := newConversationFixture(t)
			entered, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			defer once.Do(func() { close(release) })
			f.s.agentRunner = agents.NewRunner(f.s.agentManager, f.s.toolRegistry, func(ctx context.Context, _ *agents.Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
				f.calls.Add(1)
				close(entered)
				<-release // Deliberately slow cancellation acknowledgment.
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "stop", Message: api.ChatMessage{Role: "assistant", Content: "Saved in the job."}}}}, nil
			})
			f.s.agentRunner.Observer = f.s.projectConversationTask
			if w := f.submit(f.request); w.Code != 202 {
				t.Fatal(w.Code, w.Body.String())
			}
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				t.Fatal("worker did not start")
			}
			remove := func(query string) *httptest.ResponseRecorder {
				w := httptest.NewRecorder()
				f.s.sessionHandlers.HandleSessionDelete(w, httptest.NewRequest("DELETE", "/v1/sessions/chat"+query, nil), "chat")
				return w
			}
			if w := remove(""); w.Code != 409 {
				t.Fatal("active work deleted silently", w.Code)
			}
			job := f.s.agentManager.ListTasks()[0]
			w := remove("?work=" + choice)
			if choice == "stop" {
				if w.Code != 409 || !strings.Contains(w.Body.String(), "task_stop_pending") {
					t.Fatal("stop acknowledged before settlement", w.Code, w.Body.String())
				}
			} else if w.Code != 200 {
				t.Fatal(w.Code, w.Body.String())
			}
			once.Do(func() { close(release) })
			deadline := time.Now().Add(5 * time.Second)
			for f.s.agentRunner.WorkerActive(job.ID) && time.Now().Before(deadline) {
				time.Sleep(5 * time.Millisecond)
			}
			if f.s.agentRunner.WorkerActive(job.ID) {
				t.Fatal("worker did not settle")
			}
			if choice == "stop" {
				if w := remove("?work=stop"); w.Code != 200 {
					t.Fatal(w.Code, w.Body.String())
				}
			}
			if _, err := f.s.sessionHandlers.manager.Load("chat"); !errors.Is(err, sessions.ErrSessionNotFound) {
				t.Fatal("conversation retained", err)
			}
			saved, exists := f.s.agentManager.GetTask(job.ID)
			if !exists || (choice == "retain" && saved.Status != agents.TaskCompleted) || (choice == "stop" && saved.Status != agents.TaskCancelled) {
				t.Fatal("canonical job lost", saved)
			}
			if f.calls.Load() != 1 {
				t.Fatal("deletion replayed work")
			}
		})
	}
}

func TestConversationEventReconnectObservesSameCanonicalJob(t *testing.T) {
	f := newConversationFixture(t)
	if w := f.submit(f.request); w.Code != 202 {
		t.Fatal(w.Code)
	}
	conversation := f.waitComplete(t)
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/v1/sessions/chat/turn/events?id="+f.request.RequestID, nil)
		f.s.sessionHandlers.handleTurn(w, r, "chat", "turn/events")
		if w.Code != 200 || !strings.Contains(w.Body.String(), conversation.Turn.JobID) || !strings.Contains(w.Body.String(), "snapshot") {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if f.calls.Load() != 1 {
		t.Fatal("reconnection dispatched work")
	}
}

func TestConversationLinkedWorkOwnerIsolation(t *testing.T) {
	f := newConversationFixture(t)
	f.s.config.RequireAuth = true
	f.s.sessionHandlers.requireAuth = true
	store := users.NewUserStore("")
	keys := map[string]string{}
	for _, name := range []string{"alice", "bob"} {
		u, key, err := store.CreateUser(name, "fixture-password", users.RoleUser)
		if err != nil {
			t.Fatal(err)
		}
		keys[name] = key
		if name == "alice" {
			// Replace only this fixture's empty conversation with an owned one.
			if err := f.s.sessionHandlers.manager.WithAccess(sessions.Access{All: true}).Delete("chat"); err != nil {
				t.Fatal(err)
			}
			if _, err := f.s.sessionHandlers.manager.WithAccess(sessions.Access{UserID: u.ID}).Create("chat", "model"); err != nil {
				t.Fatal(err)
			}
			p, err := f.s.agentManager.SaveProfile(f.s.workspaceID, u.ID, "", 0, agents.ProfileSettings{Name: "Assistant", Model: "model", MaxIterations: 5})
			if err != nil {
				t.Fatal(err)
			}
			f.request.Agent = &sessions.AgentSelection{ID: p.ID, Revision: p.Revision}
		}
	}
	middleware := users.NewMiddleware(store)
	middleware.SetRequireAuth(true)
	handler := middleware.Wrap(http.HandlerFunc(f.s.sessionHandlers.HandleSessions))
	do := func(actor, method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if actor != "" {
			r.Header.Set("Authorization", "Bearer "+keys[actor])
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	body, _ := json.Marshal(f.request)
	if w := do("alice", "POST", "/v1/sessions/chat/generate", string(body)); w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	f.waitComplete(t)
	for _, endpoint := range []struct{ method, path, body string }{
		{"GET", "/v1/sessions/chat/turn", ""},
		{"GET", "/v1/sessions/chat/turn/events?id=" + f.request.RequestID, ""},
		{"POST", "/v1/sessions/chat/turn/recover", `{"id":"conversation-request-1"}`},
		{"POST", "/v1/sessions/chat/turn/cancel", `{"id":"conversation-request-1"}`},
		{"POST", "/v1/sessions/chat/generate", string(body)},
		{"DELETE", "/v1/sessions/chat?work=retain", ""},
	} {
		if w := do("bob", endpoint.method, endpoint.path, endpoint.body); w.Code != 404 {
			t.Fatal("cross-owner access", endpoint.path, w.Code, w.Body.String())
		}
		if w := do("", endpoint.method, endpoint.path, endpoint.body); w.Code != 401 {
			t.Fatal("anonymous access", endpoint.path, w.Code)
		}
	}
	if f.calls.Load() != 1 {
		t.Fatal("unauthorized request dispatched")
	}
}

func TestConversationHistoryBudgetsAndPlainChatCompatibility(t *testing.T) {
	for _, preset := range []string{"interactive", "extended"} {
		t.Run(preset, func(t *testing.T) {
			f := newConversationFixture(t)
			f.s.config.MaxContextSize = 16384
			f.s.config.ChatContextSize = 8192
			scope := f.s.sessionHandlers.manager.WithAccess(sessions.Access{All: true})
			for i := 0; i < 20; i++ {
				if _, err := scope.AppendExchange("chat", "model", strings.Repeat("u", 1000), strings.Repeat("a", 1000)); err != nil {
					t.Fatal(err)
				}
			}
			f.s.agentRunner = agents.NewRunner(f.s.agentManager, f.s.toolRegistry, func(_ context.Context, task *agents.Task, messages []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
				f.calls.Add(1)
				if len(task.Config.ConversationContext) != 24 || task.Config.ConversationOmitted != 16 || task.Model != "model" || task.Config.MaxTokens != 256 || task.Config.ContextWindow != f.s.chatContextWindow(preset) {
					t.Error("lost pinned context/budgets", task.Config)
				}
				if messages[len(messages)-1].StringContent() != "hello" {
					t.Error("current request lost")
				}
				found := false
				for _, m := range messages {
					if strings.Contains(m.StringContent(), "Older conversation messages are omitted") {
						found = true
					}
				}
				if !found {
					t.Error("history truncation concealed")
				}
				return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "stop", Message: api.ChatMessage{Role: "assistant", Content: "Agent answer"}}}}, nil
			})
			f.s.agentRunner.Observer = f.s.projectConversationTask
			f.request.Profile, f.request.MaxTokens = preset, 256
			if w := f.submit(f.request); w.Code != 202 {
				t.Fatal(w.Code, w.Body.String())
			}
			f.waitComplete(t)
			// Removing agent selection uses the original completer, preserving
			// historical exchanges and job links without another durable job.
			f.s.sessionHandlers.completer = func(_ context.Context, model string, messages []api.ChatMessage, knowledge bool) (string, error) {
				if model != "model" || knowledge || len(messages) != 43 {
					t.Error("plain Chat compatibility", model, len(messages), knowledge)
				}
				return "Plain answer", nil
			}
			if w := f.submit(sessionGenerateRequest{Content: "Now a plain question"}); w.Code != 200 {
				t.Fatal(w.Code, w.Body.String())
			}
			conversation, err := scope.Load("chat")
			if err != nil || len(conversation.Messages) != 44 || len(conversation.Jobs) != 1 || f.calls.Load() != 1 {
				t.Fatal("plain Chat replayed or lost history", conversation, err)
			}
		})
	}
}

type conversationUncertainTools struct {
	agents.RunTools
	mutations *atomic.Int32
}

func (t conversationUncertainTools) ExecuteWithPolicy(context.Context, string, json.RawMessage, agents.ToolExecution) (string, error) {
	t.mutations.Add(1)
	return "", errors.New("lost acknowledgment after synthetic effect")
}

func TestConversationUncertainDeletionRetainsRecoveryAndNeverRetries(t *testing.T) {
	f := newConversationFixture(t)
	var mutations atomic.Int32
	d, _ := f.s.toolRegistry.Capability("write_file")
	p, err := f.s.agentManager.SaveProfile(f.s.workspaceID, "local-admin", "", 0, agents.ProfileSettings{Name: "Fixture", Model: "model", MaxIterations: 5, Tools: []agents.ProfileToolReference{{Provider: d.Source, Tool: d.Name, SchemaRevision: d.Revision, Location: d.Location}}})
	if err != nil {
		t.Fatal(err)
	}
	f.request.Agent = &sessions.AgentSelection{ID: p.ID, Revision: p.Revision}
	f.s.agentRunner = agents.NewRunner(f.s.agentManager, conversationUncertainTools{f.s.toolRegistry, &mutations}, func(context.Context, *agents.Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		f.calls.Add(1)
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "tool_calls", Message: api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{{Type: "function", Function: api.FunctionCall{Name: "write_file", Arguments: `{"path":"unused-fixture.txt","content":"synthetic only"}`}}}}}}}, nil
	})
	f.s.agentRunner.Observer = f.s.projectConversationTask
	if w := f.submit(f.request); w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	deadline := time.Now().Add(5 * time.Second)
	var job *agents.Task
	for time.Now().Before(deadline) {
		jobs := f.s.agentManager.ListTasks()
		if len(jobs) == 1 {
			job = jobs[0]
			if job.Status == agents.TaskUncertain && !f.s.agentRunner.WorkerActive(job.ID) {
				break
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	if job == nil || job.Status != agents.TaskUncertain {
		t.Fatal("fixture did not retain unknown effects", job)
	}
	w := httptest.NewRecorder()
	f.s.sessionHandlers.HandleSessionDelete(w, httptest.NewRequest("DELETE", "/v1/sessions/chat?work=stop", nil), "chat")
	if w.Code != 409 {
		t.Fatal("stop erased uncertain work", w.Code, w.Body.String())
	}
	if _, err := f.s.sessionHandlers.manager.Load("chat"); err != nil {
		t.Fatal(err)
	}
	if w := f.submit(f.request); w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	f.s.sessionHandlers.HandleSessionDelete(w, httptest.NewRequest("DELETE", "/v1/sessions/chat?work=retain", nil), "chat")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	saved, exists := f.s.agentManager.GetTask(job.ID)
	if !exists || saved.Status != agents.TaskUncertain || saved.Checkpoint.ExecutingCall == "" || mutations.Load() != 1 || f.calls.Load() != 1 {
		t.Fatal("lost recovery or replayed uncertain work", saved, mutations.Load(), f.calls.Load())
	}
}
