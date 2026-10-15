package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

func TestProfileCommandsUseAuthenticatedRevisionBoundService(t *testing.T) {
	var mutations []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Error("missing actor credentials")
		}
		if r.URL.Path == "/api/v2/system" {
			w.Write([]byte(`{"product":"offgrid","capabilities":["agent-profiles-v1"]}`))
			return
		}
		if r.Method != "GET" {
			var body map[string]any
			if json.NewDecoder(r.Body).Decode(&body) != nil {
				t.Error("bad body")
			}
			body["path"] = r.URL.Path
			mutations = append(mutations, body)
		}
		if strings.HasSuffix(r.URL.Path, "/archive") {
			w.Write([]byte(`{"id":"writer","archived":true}`))
			return
		}
		if r.Method == "PUT" && len(mutations) > 2 {
			w.WriteHeader(409)
			return
		}
		if r.Method == "GET" && r.URL.Path == "/api/v2/agent-profiles" {
			w.Write([]byte(`{"profiles":[]}`))
			return
		}
		w.Write([]byte(`{"id":"writer","revision":2,"settings":{"name":"Writer\u001b[2J","model":"installed","max_iterations":10}}`))
	}))
	defer server.Close()
	client, _ := serviceclient.New(server.URL, "secret", server.Client())
	var stdout bytes.Buffer
	settings := `{"name":"Writer","model":"installed","max_iterations":10,"instructions":"Use evidence."}`
	for _, args := range [][]string{{"list"}, {"create", "-"}, {"update", "writer", "1", "-"}, {"archive", "writer", "2"}} {
		if err := profileCommand(context.Background(), client, args, strings.NewReader(settings), &stdout, false); err != nil {
			t.Fatal(args, err)
		}
	}
	if len(mutations) != 3 || mutations[1]["expected_revision"] != float64(1) || mutations[2]["path"] != "/api/v2/agent-profiles/writer/archive" {
		t.Fatal(mutations)
	}
	if strings.ContainsRune(stdout.String(), '\x1b') {
		t.Fatal("terminal injection")
	}
	if err := profileCommand(context.Background(), client, []string{"update", "writer", "1", "-"}, strings.NewReader(settings), &stdout, false); err == nil {
		t.Fatal("conflict hidden")
	}
	if len(mutations) != 4 {
		t.Fatal("conflicting mutation retried")
	}
	for _, args := range [][]string{{"archive", "writer", "0"}, {"show", "../escape"}, {"update", "writer", "2"}} {
		if profileCommand(context.Background(), client, args, strings.NewReader(settings), &stdout, false) == nil {
			t.Fatal("invalid args accepted", args)
		}
	}
	for _, invalid := range []string{settings + ` {}`, `{"name":"x","model":"m","max_iterations":2,"token":"secret"}`, strings.Repeat(" ", 65537)} {
		if _, err := readProfileSettings("-", strings.NewReader(invalid)); err == nil {
			t.Fatal("invalid settings accepted")
		}
	}
}

func TestProfileSubmissionRequiresExplicitRevisionAndCompatibleService(t *testing.T) {
	req, _, err := parseTaskSubmission([]string{"hello", "--profile", "writer", "--profile-revision", "3", "--request-id", "stable-request"})
	if err != nil || req.ProfileRevision != 3 || req.Model != "" {
		t.Fatal(req, err)
	}
	for _, args := range [][]string{{"hello", "--profile", "writer"}, {"hello", "--model", "m", "--profile-revision", "3"}} {
		if _, _, err := parseTaskSubmission(args); err == nil {
			t.Fatal("unpinned profile accepted")
		}
	}
	posts := 0
	supported := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/system" {
			caps := []string{}
			if supported {
				caps = []string{"agent-profiles-v1"}
			}
			json.NewEncoder(w).Encode(map[string]any{"product": "offgrid", "capabilities": caps})
			return
		}
		posts++
		var body taskSubmission
		json.NewDecoder(r.Body).Decode(&body)
		if body != req {
			t.Error("profile identity changed")
		}
		w.Write([]byte(`{"run_id":"run-one","status":"waiting_for_input","pending_input":{"id":"access","mode":"desktop"}}`))
	}))
	defer server.Close()
	client, _ := serviceclient.New(server.URL, "", server.Client())
	if _, err := submitTask(context.Background(), client, req); err == nil || posts != 0 {
		t.Fatal("old server received profile mutation")
	}
	supported = true
	result, err := submitTask(context.Background(), client, req)
	if err != nil || posts != 1 || result.PendingInput == nil {
		t.Fatal(result, err)
	}
}

func TestConversationCLIUsesOneCanonicalJobAndExplicitRecovery(t *testing.T) {
	var mutations []string
	supported := true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/system" {
			caps := []string{}
			if supported {
				caps = []string{"agent-conversations-v1"}
			}
			json.NewEncoder(w).Encode(map[string]any{"product": "offgrid", "capabilities": caps})
			return
		}
		if r.Method == "POST" {
			mutations = append(mutations, r.URL.Path)
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			if strings.HasSuffix(r.URL.Path, "/recover") {
				if body["id"] != "same-turn" {
					t.Error("wrong recovery identity")
				}
			} else if body["request_id"] != "same-turn" || body["agent"].(map[string]any)["revision"] != float64(3) {
				t.Error("wrong pinned submission", body)
			}
		}
		w.Write([]byte(`{"turn":{"id":"same-turn","job_id":"run-canonical","status":"uncertain"},"job":{"run_id":"run-canonical","status":"uncertain","uncertain_call_id":"call-sent"},"recovery_required":true}`))
	}))
	defer server.Close()
	client, _ := serviceclient.New(server.URL, "", server.Client())
	var out bytes.Buffer
	ctx := context.Background()
	for _, args := range [][]string{{"send", "saved-chat", "--profile", "writer", "--profile-revision", "3", "--request-id", "same-turn", "hello"}, {"turn", "saved-chat"}, {"recover", "saved-chat", "same-turn"}} {
		if err := conversationCommand(ctx, client, args, &out, true); err != nil {
			t.Fatal(args, err)
		}
	}
	if len(mutations) != 2 || mutations[0] != "/v1/sessions/saved-chat/generate" || mutations[1] != "/v1/sessions/saved-chat/turn/recover" {
		t.Fatal(mutations)
	}
	if strings.Count(out.String(), `"run_id":"run-canonical"`) != 3 || !strings.Contains(out.String(), `"uncertain_call_id":"call-sent"`) {
		t.Fatal(out.String())
	}
	supported = false
	if err := conversationCommand(ctx, client, []string{"send", "saved-chat", "--profile", "writer", "--profile-revision", "3", "hello"}, &out, true); err == nil || len(mutations) != 2 {
		t.Fatal("old server received mutation")
	}
}

func TestInteractiveProfileConversationSteersAndStopsOnlyTheCanonicalJob(t *testing.T) {
	for _, state := range []string{"running", "waiting_for_input", "uncertain"} {
		t.Run(state, func(t *testing.T) {
			var writes []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/v2/system":
					w.Write([]byte(`{"product":"offgrid","capabilities":["agent-conversations-v1"]}`))
					return
				case "/v1/sessions/saved-chat":
					w.Write([]byte(`{"name":"saved-chat","created_at":"2026-10-13T00:00:00Z","updated_at":"2026-10-13T00:00:00Z"}`))
					return
				case "/v1/sessions/saved-chat/turn":
					json.NewEncoder(w).Encode(map[string]any{"turn": map[string]string{"id": "saved-turn", "job_id": "run-one"}, "job": map[string]any{"run_id": "run-one", "status": state, "can_steer": state == "running"}})
					return
				}
				if r.Method != "POST" {
					t.Error("unexpected read", r.URL.Path)
				}
				writes = append(writes, r.URL.Path)
				if strings.HasSuffix(r.URL.Path, "/steer") {
					var body map[string]string
					json.NewDecoder(r.Body).Decode(&body)
					if body["instruction"] != "Use short paragraphs." || len(body["request_id"]) < 8 {
						t.Error(body)
					}
				}
				json.NewEncoder(w).Encode(map[string]string{"run_id": "run-one", "status": "cancelled"})
			}))
			defer server.Close()
			client, _ := serviceclient.New(server.URL, "", server.Client())
			var out bytes.Buffer
			if err := conversationChat(context.Background(), client, []string{"chat", "saved-chat", "--profile", "writer", "--profile-revision", "3"}, strings.NewReader("/status\nUse short paragraphs.\n/stop\n/exit\n"), &out, false); err != nil {
				t.Fatal(err)
			}
			want := 1
			if state == "running" {
				want = 2
			}
			if len(writes) != want || writes[len(writes)-1] != "/api/v2/jobs/run-one/cancel" {
				t.Fatal(writes)
			}
			if state != "running" && !strings.Contains(out.String(), "Message not submitted") {
				t.Fatal(out.String())
			}
		})
	}
}

func TestConversationDeleteRequiresExplicitWorkChoice(t *testing.T) {
	var choices []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		choices = append(choices, r.URL.Query().Get("work"))
		if r.URL.Query().Get("work") == "" {
			w.WriteHeader(409)
			return
		}
		w.Write([]byte(`{"success":true}`))
	}))
	defer server.Close()
	client, _ := serviceclient.New(server.URL, "", server.Client())
	var out bytes.Buffer
	if sessionCommand(context.Background(), client, []string{"delete", "chat"}, &out, true) == nil {
		t.Fatal("active work silently deleted")
	}
	for _, choice := range []string{"retain", "stop"} {
		if err := sessionCommand(context.Background(), client, []string{"delete", "chat", "--work", choice}, &out, true); err != nil {
			t.Fatal(err)
		}
	}
	if strings.Join(choices, ",") != ",retain,stop" {
		t.Fatal(choices)
	}
}
