package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/takuphilchan/offgrid-llm/internal/agents"
)

// The same task IDs, ownership, snapshots and SQLite transactions as agents.
// Mutating job operations remain on their existing governed routes during cutover.
func (s *Server) handleJobRead(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if w.Header().Get("X-Request-ID") == "" {
		w.Header().Set("X-Request-ID", uuid.NewString())
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeJobError(w, 405, "method_not_allowed", "This job resource supports GET only.", false)
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v2/jobs/"), "/")
	if len(parts) < 1 || len(parts) > 2 || parts[0] == "" || (len(parts) == 2 && parts[1] != "events") {
		writeJobReadError(w, agents.ErrTaskNotFound)
		return
	}
	if s.agentManager == nil {
		writeJobReadError(w, agents.ErrRunStorage)
		return
	}
	var after int64
	if value := r.Header.Get("Last-Event-ID"); value != "" {
		var err error
		after, err = strconv.ParseInt(value, 10, 64)
		if err != nil || after < 0 || len(value) > 19 || strings.Trim(value, "0123456789") != "" {
			writeJobError(w, 400, "invalid_event_cursor", "Use the last received job event ID, or reconnect without a cursor.", false)
			return
		}
	}
	replay, err := s.agentManager.ReplayTask(r.Context(), parts[0], s.agentActor(r), after)
	if err != nil && !errors.Is(err, agents.ErrEventCursor) {
		writeJobReadError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	if len(parts) == 1 {
		response := taskResponse(replay.Snapshot)
		response["event_cursor"] = strconv.FormatInt(replay.Cursor, 10)
		writeJSON(w, 200, response)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJobError(w, 503, "job_stream_unavailable", "Streaming is unavailable. Read the saved job snapshot instead.", true)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("X-Accel-Buffering", "no")
	controller := http.NewResponseController(w)
	defer controller.SetWriteDeadline(time.Time{})
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	lastPayload := ""
	lastSent := time.Now()
	for {
		_ = controller.SetWriteDeadline(time.Now().Add(10 * time.Second))
		for _, event := range replay.Events {
			payload, _ := json.Marshal(map[string]any{"type": "activity", "activity": event})
			if _, err := fmt.Fprintf(w, "id: %d\nevent: activity\ndata: %s\n\n", event.Sequence, payload); err != nil {
				return
			}
		}
		response := taskResponse(replay.Snapshot)
		response["type"] = "snapshot"
		if replay.Recovery {
			response["type"] = "snapshot_recovery"
		}
		response["event_cursor"] = strconv.FormatInt(replay.Cursor, 10)
		payload, _ := json.Marshal(response)
		if string(payload) != lastPayload {
			if _, err := fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", replay.Cursor, response["type"], payload); err != nil {
				return
			}
			lastPayload, lastSent = string(payload), time.Now()
			flusher.Flush()
		} else if time.Since(lastSent) >= 5*time.Second {
			if _, err := fmt.Fprint(w, ": heartbeat\n\n"); err != nil {
				return
			}
			lastSent = time.Now()
			flusher.Flush()
		}
		if replay.Snapshot.Status != agents.TaskRunning && replay.Snapshot.Status != agents.TaskPending {
			return
		}
		after = replay.Cursor
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
		}
		replay, err = s.agentManager.ReplayTask(r.Context(), parts[0], s.agentActor(r), after)
		if err != nil && !errors.Is(err, agents.ErrEventCursor) {
			return
		}
	}
}

func writeJobError(w http.ResponseWriter, status int, code, message string, retryable bool) {
	writeJSON(w, status, map[string]any{"error": map[string]any{"code": code, "message": message, "retryable": retryable, "request_id": w.Header().Get("X-Request-ID")}})
}
func writeJobReadError(w http.ResponseWriter, err error) {
	if errors.Is(err, agents.ErrTaskNotFound) {
		writeJobError(w, 404, "job_not_found", "This job is not available in your workspace.", false)
		return
	}
	writeJobError(w, 503, "job_storage_unavailable", "Job history is unavailable. Do not resubmit work; reconnect to its saved state after the service recovers.", true)
}
