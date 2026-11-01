package server

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/automation"
)

// This read-only projection uses the scheduler's calendar and clock. It neither
// saves a policy nor claims an occurrence, and cannot approve future execution.
func (s *Server) previewScheduleCalendar(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	for _, key := range []string{"zone", "hour", "minute", "weekdays"} {
		if len(query[key]) != 1 {
			writeJobError(w, 400, "invalid_calendar", automation.ErrCalendar.Error(), false)
			return
		}
	}
	hour, hourErr := strconv.Atoi(query.Get("hour"))
	minute, minuteErr := strconv.Atoi(query.Get("minute"))
	calendar := automation.Calendar{Zone: query.Get("zone"), Hour: hour, Minute: minute}
	if hourErr != nil || minuteErr != nil || len(query.Get("weekdays")) > 13 {
		writeJobError(w, 400, "invalid_calendar", automation.ErrCalendar.Error(), false)
		return
	}
	for _, raw := range strings.Split(query.Get("weekdays"), ",") {
		day, err := strconv.Atoi(raw)
		if err != nil {
			writeJobError(w, 400, "invalid_calendar", automation.ErrCalendar.Error(), false)
			return
		}
		calendar.Weekdays = append(calendar.Weekdays, day)
	}
	calendar, err := calendar.Canonical()
	if err != nil {
		writeJobError(w, 400, "invalid_calendar", automation.ErrCalendar.Error(), false)
		return
	}
	now := s.schedules.clock().UTC()
	after := now
	occurrences := make([]time.Time, 0, 3)
	for range 3 {
		next, err := calendar.Next(after)
		if err != nil {
			writeJobError(w, 400, "invalid_calendar", automation.ErrCalendar.Error(), false)
			return
		}
		occurrences = append(occurrences, next)
		after = next
	}
	writeJSON(w, http.StatusOK, map[string]any{"calendar": calendar, "as_of": now, "occurrences": occurrences})
}
