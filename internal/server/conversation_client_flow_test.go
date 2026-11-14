package server

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/cli"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestConversationClientFlowDefaultCLIAndRetryWithoutInference(t *testing.T) {
	f := newConversationFixture(t)
	f.s.config.EnableConversationPreview = true
	f.s.bindCanonicalConversationGuard()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v2/system":
			f.s.handleSystemIdentity(w, r)
		case strings.HasPrefix(r.URL.Path, "/api/v2/conversations"):
			f.s.handleConversations(w, r)
		case r.URL.Path == "/v1/sessions" && r.Method == "POST":
			f.s.sessionHandlers.HandleSessionCreate(w, r)
		case strings.HasPrefix(r.URL.Path, "/v1/sessions/"):
			f.s.sessionHandlers.HandleSessionGet(w, r, strings.TrimPrefix(r.URL.Path, "/v1/sessions/"))
		default:
			t.Error("unexpected client path", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	client, err := serviceclient.New(server.URL, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	env := cli.Environment{Stdin: strings.NewReader("First question\nSecond question while work waits\n/exit\n"), Stdout: &output, Stderr: &output, Service: cli.ServiceDependencies{Client: func() (*serviceclient.Client, error) { return client, nil }}}
	if err = cli.RunSessionCommand(context.Background(), []string{"chat"}, env); err != nil {
		t.Fatal(err)
	}
	if strings.Count(output.String(), "Accepted input ") != 2 || f.calls.Load() != 0 {
		t.Fatal("CLI waited for inference or lost input", output.String(), f.calls.Load())
	}
	// The import manifest supplies the canonical identity; no client-local run
	// or profile is required, and a different process can inspect the same work.
	list, err := f.s.sessionHandlers.manager.WithAccess(sessions.Access{All: true}).List()
	if err != nil || len(list) != 2 {
		t.Fatal("default conversation missing", list, err)
	}
	for _, saved := range list {
		if saved.Name == "chat" {
			continue
		}
		scope := f.s.controlScope(httptest.NewRequest("GET", "/", nil))
		manifest, e := f.s.agentManager.ImportedConversation(scope, saved.Name)
		if e != nil || manifest == nil {
			t.Fatal(manifest, e)
		}
		page, e := f.s.agentManager.ConversationInbox(scope, manifest.ConversationID, 0, 100)
		if e != nil || len(page.Items) != 2 {
			t.Fatal(page, e)
		}
	}
}

func TestConversationClientRetryObservesAcceptedInputAndRejectsChangedReuse(t *testing.T) {
	s, scope, c := contextInboxFixture(t)
	s.config.EnableConversationPreview = true
	server := httptest.NewServer(http.HandlerFunc(s.handleConversations))
	defer server.Close()
	client, _ := serviceclient.New(server.URL, "", nil)
	input := api.ConversationInboxInput{Content: "Retain this input", Origin: "typed"}
	first, err := client.SubmitSessionInput(context.Background(), c.ID, "client-retry", input)
	if err != nil {
		t.Fatal(err)
	}
	// Later state revisions and disabled admission cannot turn retry into new work.
	s.config.EnableConversationPreview = false
	again, err := client.SubmitSessionInput(context.Background(), c.ID, "client-retry", input)
	if err != nil || again.ID != first.ID {
		t.Fatal(again, err)
	}
	input.Content = "Different input"
	if _, err = client.SubmitSessionInput(context.Background(), c.ID, "client-retry", input); err == nil {
		t.Fatal("changed reuse accepted")
	}
	page, err := s.agentManager.ConversationInbox(scope, c.ID, 0, 100)
	if err != nil || len(page.Items) != 1 || page.Items[0].RunID != "" {
		t.Fatal("client replayed mutation", page, err)
	}
}
