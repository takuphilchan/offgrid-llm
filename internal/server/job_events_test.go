package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
)

type blockedJobViewer struct {
	header  http.Header
	started chan struct{}
	release chan struct{}
}

func (w *blockedJobViewer) Header() http.Header { return w.header }
func (w *blockedJobViewer) WriteHeader(int)     {}
func (w *blockedJobViewer) Flush()              {}
func (w *blockedJobViewer) Write([]byte) (int, error) {
	select {
	case w.started <- struct{}{}:
	default:
	}
	<-w.release
	return 0, errors.New("viewer disconnected")
}

func TestSlowJobViewerCannotBlockTaskCancellation(t *testing.T) {
	m := agents.NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	runner := agents.NewRunner(m, nil, nil)
	task, err := runner.Create("test", "model", "local-admin", agents.DefaultAgentConfig())
	if err != nil {
		t.Fatal(err)
	}
	w := &blockedJobViewer{header: make(http.Header), started: make(chan struct{}, 1), release: make(chan struct{})}
	done := make(chan struct{})
	go func() {
		defer close(done)
		(&Server{agentManager: m}).handleJobRead(w, httptest.NewRequest(http.MethodGet, "/api/v2/jobs/"+task.ID+"/events", nil))
	}()
	defer func() { close(w.release); <-done }()
	select {
	case <-w.started:
	case <-time.After(5 * time.Second):
		t.Fatal("stream did not start")
	}
	cancelled := make(chan error, 1)
	go func() { _, err := runner.Stop(task.ID, "local-admin", "cancel", ""); cancelled <- err }()
	select {
	case err := <-cancelled:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("viewer held execution/storage lock")
	}
}

func TestJobEventsUseDurableOwnerScopedAgentHistory(t *testing.T) {
	m := agents.NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	runner := agents.NewRunner(m, nil, nil)
	task, err := runner.Create("private prompt", "model", "local-admin", agents.DefaultAgentConfig())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = runner.Stop(task.ID, "local-admin", "cancel", ""); err != nil {
		t.Fatal(err)
	}
	s := &Server{agentManager: m}
	for _, cursor := range []string{"", "1", "999999"} {
		r := httptest.NewRequest(http.MethodGet, "/api/v2/jobs/"+task.ID+"/events", nil)
		r.Header.Set("Last-Event-ID", cursor)
		w := httptest.NewRecorder()
		s.handleJobRead(w, r)
		body := w.Body.String()
		if w.Code != 200 || !strings.Contains(w.Header().Get("Content-Type"), "text/event-stream") || !strings.Contains(body, `"status":"cancelled"`) || !strings.Contains(body, "id: ") {
			t.Fatalf("missing terminal replay: %d %s", w.Code, body)
		}
		if cursor == "999999" && !strings.Contains(body, "event: snapshot_recovery") {
			t.Fatal("no explicit cursor recovery")
		}
		if strings.Contains(body, "private prompt") {
			t.Fatal("event leaked prompt")
		}
	}
	r := httptest.NewRequest(http.MethodGet, "/api/v2/jobs/"+task.ID+"/events", nil)
	r.Header.Set("Last-Event-ID", "garbage")
	w := httptest.NewRecorder()
	s.handleJobRead(w, r)
	if w.Code != 400 || !strings.Contains(w.Body.String(), "invalid_event_cursor") {
		t.Fatal("invalid cursor accepted")
	}
	other, err := runner.Create("other private prompt", "model", "bob", agents.DefaultAgentConfig())
	if err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	s.handleJobRead(w, httptest.NewRequest(http.MethodGet, "/api/v2/jobs/"+other.ID+"/events", nil))
	if w.Code != 404 {
		t.Fatal("cross-user job visible")
	}
}
