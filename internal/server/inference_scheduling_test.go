package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
)

func TestHealthReportsSharedAdmissionWithoutWorkIdentities(t *testing.T) {
	s := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	held, _, err := s.providerScheduler.AcquireGeneration(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer held()
	done := make(chan struct{})
	go func() {
		release, _, _ := s.providerScheduler.AcquireGeneration(inference.WithSchedulingScope(ctx, inference.SchedulingScope{Actor: "private-actor", Work: "private-work", Background: true}))
		if release != nil {
			release()
		}
		close(done)
	}()
	for s.providerScheduler.AdmissionStatus().BackgroundQueued != 1 {
		if ctx.Err() != nil {
			t.Fatal("background request did not queue")
		}
		time.Sleep(time.Millisecond)
	}
	w := httptest.NewRecorder()
	s.handleHealth(w, httptest.NewRequest("GET", "/health", nil))
	var response struct {
		Scheduling inference.AdmissionStatus `json:"scheduling"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Scheduling.Admitted != 1 || response.Scheduling.BackgroundQueued != 1 || response.Scheduling.ForegroundQueued != 0 || response.Scheduling.Policy == "" {
		t.Fatal("public queue status is inaccurate", response)
	}
	if strings.Contains(w.Body.String(), "private-actor") || strings.Contains(w.Body.String(), "private-work") {
		t.Fatal("health leaked ownership identities")
	}
	cancel()
	<-done
}
