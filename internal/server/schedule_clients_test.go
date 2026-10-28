package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/cli"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

func TestScheduleCLIThroughHTTPPreservesPolicyAndNeverRunsInspection(t *testing.T) {
	s, policy, _ := scheduledFixture(t, nil)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/system", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 200, map[string]any{"product": "offgrid", "capabilities": []string{"agent-schedules-v1"}})
	})
	mux.HandleFunc("/api/v2/agent-schedules", s.handleAgentSchedules)
	mux.HandleFunc("/api/v2/agent-schedules/", s.handleAgentSchedules)
	service := httptest.NewServer(mux)
	defer service.Close()
	client, err := serviceclient.New(service.URL, "", service.Client())
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	env := cli.Environment{Stdout: &out, Stderr: io.Discard, Stdin: strings.NewReader(""), Service: cli.ServiceDependencies{Client: func() (*serviceclient.Client, error) { return client, nil }}}
	run := func(args ...string) {
		t.Helper()
		out.Reset()
		if err := cli.RunAgentCommand(context.Background(), args, env); err != nil {
			t.Fatal(args, err)
		}
	}
	run("schedules", "list")
	if !strings.Contains(out.String(), policy.ID) {
		t.Fatal(out.String())
	}
	run("schedules", "show", policy.ID)
	if strings.Contains(out.String(), "review_epoch") || !strings.Contains(out.String(), policy.Config.Profile.Settings.Model) {
		t.Fatal(out.String())
	}
	run("schedules", "history", policy.ID)
	if !strings.Contains(out.String(), `"occurrences":[]`) {
		t.Fatal(out.String())
	}
	settings := policy.Settings
	settings.Name = "Reviewed café 日本語"
	settings.Enabled = false
	data, _ := json.Marshal(map[string]any{"settings": settings, "profile_id": policy.Config.Profile.ID, "profile_revision": 1})
	env.Stdin = strings.NewReader(string(data))
	run("schedules", "create", "-")
	var view struct {
		ID       string `json:"id"`
		Revision int    `json:"revision"`
	}
	if json.Unmarshal(out.Bytes(), &view) != nil || view.ID == policy.ID || view.Revision != 1 {
		t.Fatal(out.String())
	}
	settings.Enabled = true
	data, _ = json.Marshal(map[string]any{"settings": settings, "profile_id": policy.Config.Profile.ID, "profile_revision": 1})
	env.Stdin = strings.NewReader(string(data))
	run("schedules", "update", view.ID, "1", "-")
	run("schedules", "disable", view.ID, "2")
	if !strings.Contains(out.String(), `"enabled":false`) {
		t.Fatal(out.String())
	}
	if err = cli.RunAgentCommand(context.Background(), []string{"schedules", "disable", view.ID, "2"}, env); err == nil {
		t.Fatal("stale writer accepted")
	}
	for _, bad := range []string{`{"settings":{},"actor":"other"}`, `{"settings":{},"settings":{}}`, `{"profile_id":"x","profile_revision":1,"expected_revision":5}`} {
		env.Stdin = strings.NewReader(bad)
		if err = cli.RunAgentCommand(context.Background(), []string{"schedules", "create", "-"}, env); err == nil {
			t.Fatal("invalid input", bad)
		}
	}
	if len(s.agentManager.ListTasks()) != 0 {
		t.Fatal("management started a job")
	}
}

func TestScheduleAPIMethodAndAuthorityFieldsRejected(t *testing.T) {
	s, policy, _ := scheduledFixture(t, nil)
	for _, method := range []string{"PATCH", "DELETE"} {
		if w := scheduleHTTP(s, method, "/api/v2/agent-schedules/"+policy.ID, nil); w.Code != 405 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	for _, extra := range []string{"actor", "config", "computer_session", "due_at", "run_now"} {
		w := scheduleHTTP(s, "POST", "/api/v2/agent-schedules", map[string]any{extra: "forged"})
		if w.Code != 400 {
			t.Fatal(extra, w.Code, w.Body.String())
		}
	}
	if w := scheduleHTTP(s, "POST", "/api/v2/agent-schedules/"+policy.ID+"/run", nil); w.Code != 405 {
		t.Fatal(w.Code, w.Body.String())
	}
	if len(s.agentManager.ListTasks()) != 0 {
		t.Fatal("rejected request submitted work")
	}
}
