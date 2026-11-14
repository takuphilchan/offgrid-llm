package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestConversationWorkerCommandsReuseSessionAndUnknownReceipt(t *testing.T) {
	worker := api.WorkerSession{ID: "worker", WorkID: "work", RootRunID: "root", CurrentRunID: "child", Revision: 3}
	commands := []api.WorkerCommand{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fixture" {
			t.Error("missing authentication")
		}
		switch r.URL.Path {
		case "/api/v2/conversations/conversation":
			json.NewEncoder(w).Encode(api.Conversation{ID: "conversation", FocusedWorkID: "work"})
		case "/api/v2/work-items/work":
			json.NewEncoder(w).Encode(map[string]any{"work": api.WorkItem{ID: "work", Revision: 1, RunID: "root"}, "run": map[string]string{"run_id": "root"}})
		case "/api/v2/jobs/root":
			json.NewEncoder(w).Encode(map[string]any{"children": []any{map[string]string{"worker_id": "worker", "id": "old"}, map[string]string{"worker_id": "worker", "id": "child"}}})
		case "/api/v2/workers/worker":
			json.NewEncoder(w).Encode(worker)
		case "/api/v2/workers/worker/messages":
			json.NewEncoder(w).Encode(api.WorkerMessagePage{Items: []api.WorkerMessage{{ID: "mail", State: "applied", Content: "Keep citations"}}})
		case "/api/v2/workers/worker/commands":
			var command api.WorkerCommand
			if err := json.NewDecoder(r.Body).Decode(&command); err != nil {
				t.Error(err)
			}
			commands = append(commands, command)
			if len(commands) == 1 {
				worker.Revision++
				http.Error(w, "lost acknowledgment", 503)
				return
			}
			json.NewEncoder(w).Encode(api.ConversationCommandReceipt{Worker: &worker})
		default:
			t.Error("unexpected request", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	client, _ := serviceclient.New(server.URL, "fixture", nil)
	ctx := context.Background()
	workers, err := focusedConversationWorkers(ctx, client, "conversation")
	if err != nil || len(workers) != 1 || workers[0].ID != worker.ID {
		t.Fatal(workers, err)
	}
	controls := &conversationWorkerControls{}
	var out bytes.Buffer
	if handled, err := controls.handle(ctx, client, "conversation", "/worker", "worker", &out); !handled || err != nil || !strings.Contains(out.String(), "applied") || len(commands) != 0 {
		t.Fatal(out.String(), handled, err)
	}
	if _, err := controls.handle(ctx, client, "conversation", "/message-worker", "worker Keep citations", &out); err == nil || controls.pending == nil {
		t.Fatal("unknown command was discarded")
	}
	if _, err := controls.handle(ctx, client, "conversation", "/retry-worker", "", &out); err != nil {
		t.Fatal(err)
	}
	if len(commands) != 2 || commands[0] != commands[1] || commands[1].ExpectedRevision != 3 || controls.pending != nil {
		t.Fatal("retry changed accepted envelope", commands)
	}
	for _, command := range []string{"/continue-worker", "/answer-worker", "/stop-worker", "/close-worker"} {
		arg := "worker"
		if command == "/continue-worker" {
			arg += " Expand findings"
		}
		if command == "/answer-worker" {
			arg += " question-one Include offline runtimes"
		}
		if _, err := controls.handle(ctx, client, "conversation", command, arg, &out); err != nil {
			t.Fatal(err)
		}
	}
	for _, command := range commands {
		if command.TargetID != "worker" {
			t.Fatal("recreated session", command)
		}
		if command.Kind == "answer" && (command.Payload.QuestionID != "question-one" || command.Payload.Content != "Include offline runtimes") {
			t.Fatal("answer lost question identity", command)
		}
	}
	// Separate CLI invocations preserve the caller's command identity and produce
	// a single structured JSON receipt without interactive status text.
	for i := 0; i < 2; i++ {
		out.Reset()
		env := Environment{Stdin: strings.NewReader(`{"request_id":"script-stop","target_id":"worker","expected_revision":4,"kind":"stop","payload":{}}`), Stdout: &out, Service: ServiceDependencies{Client: func() (*serviceclient.Client, error) { return client, nil }}}
		env.Options.JSON = true
		if err := RunAgentCommand(ctx, []string{"worker", "command"}, env); err != nil {
			t.Fatal(err)
		}
		if !json.Valid(out.Bytes()) {
			t.Fatal("non-JSON output", out.String())
		}
	}
	if commands[len(commands)-1] != commands[len(commands)-2] {
		t.Fatal("cross-process envelope changed")
	}
	for _, body := range []string{`{} {}`, `{"request_id":"x","target_id":"worker","expected_revision":4,"kind":"stop","authority":"admin"}`, strings.Repeat(" ", 65<<10)} {
		before := len(commands)
		env := Environment{Stdin: strings.NewReader(body), Stdout: &out, Service: ServiceDependencies{Client: func() (*serviceclient.Client, error) { return client, nil }}}
		if err := RunAgentCommand(ctx, []string{"worker", "command"}, env); err == nil || len(commands) != before {
			t.Fatal("invalid envelope dispatched", err)
		}
	}
	worker.WorkID = "another-work"
	before := len(commands)
	if _, err := controls.handle(ctx, client, "conversation", "/stop-worker", "worker", &out); err == nil || len(commands) != before {
		t.Fatal("foreign work command admitted")
	}
}
