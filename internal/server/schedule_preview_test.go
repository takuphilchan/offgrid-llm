package server

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/users"
)

func TestSchedulePreviewUsesCanonicalDSTAndDoesNotChangeWork(t *testing.T) {
	s, original, clock := scheduledFixture(t, nil)
	for _, tc := range []struct{ name, now, query, first, second string }{
		{"gap", "2026-03-08T06:00:00Z", "zone=America/New_York&hour=2&minute=30&weekdays=0,1,2,3,4,5,6", "2026-03-09T06:30:00Z", "2026-03-10T06:30:00Z"},
		{"fold", "2026-11-01T04:00:00Z", "zone=America/New_York&hour=1&minute=30&weekdays=0,1,2,3,4,5,6", "2026-11-01T05:30:00Z", "2026-11-02T06:30:00Z"},
		{"fold already passed", "2026-11-01T05:45:00Z", "zone=America/New_York&hour=1&minute=30&weekdays=0,1,2,3,4,5,6", "2026-11-02T06:30:00Z", "2026-11-03T06:30:00Z"},
		{"weekdays", "2026-10-30T12:00:00Z", "zone=Africa/Harare&hour=9&minute=0&weekdays=1,2,3,4,5", "2026-11-02T07:00:00Z", "2026-11-03T07:00:00Z"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			*clock, _ = time.Parse(time.RFC3339, tc.now)
			w := scheduleHTTP(s, "GET", "/api/v2/agent-schedules/preview?"+tc.query, nil)
			var result struct {
				AsOf        string   `json:"as_of"`
				Occurrences []string `json:"occurrences"`
			}
			if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &result) != nil || len(result.Occurrences) != 3 || result.AsOf != tc.now || result.Occurrences[0] != tc.first || result.Occurrences[1] != tc.second {
				t.Fatalf("%d %s", w.Code, w.Body.String())
			}
			if w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("preview must not be cached")
			}
		})
	}
	items, err := s.agentManager.ListSchedules(s.workspaceID, "local-admin")
	if err != nil || len(items) != 1 || items[0].Digest != original.Digest || !items[0].NextAt.Equal(original.NextAt) || len(s.agentManager.ListTasks()) != 0 {
		t.Fatal("preview mutated policy or created work", err)
	}
	history, err := s.agentManager.ListScheduleOccurrences(s.workspaceID, "local-admin", original.ID)
	if err != nil || len(history) != 0 {
		t.Fatal("preview claimed an occurrence", err)
	}
}

func TestSchedulePreviewRejectsInvalidCalendarAndRequiresAuthentication(t *testing.T) {
	s, _, _ := scheduledFixture(t, nil)
	for _, query := range []string{
		"", "zone=Local&hour=9&minute=0&weekdays=1", "zone=UTC&hour=24&minute=0&weekdays=1",
		"zone=UTC&hour=9&minute=60&weekdays=1", "zone=UTC&hour=9&minute=0&weekdays=1,1",
		"zone=UTC&hour=9&minute=0&weekdays=", "zone=UTC&zone=Africa/Harare&hour=9&minute=0&weekdays=1",
	} {
		if w := scheduleHTTP(s, "GET", "/api/v2/agent-schedules/preview?"+query, nil); w.Code != 400 {
			t.Fatal(query, w.Code)
		}
	}
	s.config.RequireAuth = true
	s.userStore = users.NewUserStore("")
	middleware := users.NewMiddleware(s.userStore)
	middleware.SetRequireAuth(true)
	w := httptest.NewRecorder()
	middleware.Wrap(s.requirePermissionWhenAuthEnabled(users.PermissionChat, s.handleAgentSchedules)).ServeHTTP(w, httptest.NewRequest("GET", "/api/v2/agent-schedules/preview?zone=UTC&hour=9&minute=0&weekdays=1", nil))
	if w.Code != 401 {
		t.Fatal("unauthenticated preview", w.Code)
	}
}
