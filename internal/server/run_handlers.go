package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/artifacts"
	"github.com/takuphilchan/offgrid-llm/internal/runs"
)

type runSummary struct {
	ID         string         `json:"id"`
	Status     string         `json:"status"`
	StartedAt  time.Time      `json:"started_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	EventCount int            `json:"event_count"`
	Data       map[string]any `json:"data,omitempty"`
}

func (s *Server) publishRunEvent(ctx context.Context, runID string, eventType runs.EventType, data any) {
	if s.runLog == nil {
		return
	}
	event, err := runs.NewEvent(runID, eventType, data)
	if err != nil {
		return
	}
	// A disconnected HTTP client must not prevent the terminal event from
	// reaching durable storage.
	if ctx == nil || ctx.Err() != nil {
		ctx = context.Background()
	}
	_ = s.runLog.Publish(ctx, event)
}

func (s *Server) persistRunOutput(ctx context.Context, runID, output string) (*artifacts.Metadata, error) {
	if s.artifactStore == nil || output == "" {
		return nil, nil
	}
	metadata, err := s.artifactStore.Put(ctx, strings.NewReader(output), artifacts.Metadata{
		Name:      runID + "-output.txt",
		MediaType: "text/plain; charset=utf-8",
		Labels:    map[string]string{"run_id": runID, "kind": "agent-output"},
	})
	if err != nil {
		return nil, err
	}
	s.publishRunEvent(ctx, runID, runs.ArtifactCreated, metadata)
	return &metadata, nil
}

func (s *Server) handleRuns(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if s.runLog == nil {
		json.NewEncoder(w).Encode(map[string]any{"runs": []runSummary{}})
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/v1/runs")
	path = strings.Trim(path, "/")
	if path != "" {
		parts := strings.Split(path, "/")
		if len(parts) != 2 || parts[1] != "events" || parts[0] == "" {
			http.NotFound(w, r)
			return
		}
		after, err := strconv.ParseUint(r.URL.Query().Get("after"), 10, 64)
		if err != nil && r.URL.Query().Get("after") != "" {
			http.Error(w, `{"error":"after must be an unsigned integer"}`, http.StatusBadRequest)
			return
		}
		events, err := s.runLog.Replay(parts[0], after)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"events": events})
		return
	}

	events, err := s.runLog.Replay("", 0)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusInternalServerError)
		return
	}
	byID := make(map[string]*runSummary)
	for _, event := range events {
		summary := byID[event.RunID]
		if summary == nil {
			summary = &runSummary{ID: event.RunID, Status: "running", StartedAt: event.Time, Data: make(map[string]any)}
			byID[event.RunID] = summary
		}
		summary.EventCount++
		summary.UpdatedAt = event.Time
		if event.Type == runs.RunStarted && len(event.Data) > 0 {
			_ = json.Unmarshal(event.Data, &summary.Data)
		}
		switch event.Type {
		case runs.ApprovalRequired:
			summary.Status = "approval_required"
		case runs.RunCompleted:
			summary.Status = "completed"
		case runs.RunFailed:
			summary.Status = "failed"
		}
	}
	summaries := make([]runSummary, 0, len(byID))
	for _, summary := range byID {
		summaries = append(summaries, *summary)
	}
	sort.Slice(summaries, func(i, j int) bool { return summaries[i].UpdatedAt.After(summaries[j].UpdatedAt) })
	json.NewEncoder(w).Encode(map[string]any{"runs": summaries})
}

func (s *Server) handleArtifacts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.artifactStore == nil {
		http.Error(w, `{"error":"artifact store unavailable"}`, http.StatusServiceUnavailable)
		return
	}
	digest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/v1/artifacts/"), "/")
	reader, metadata, err := s.artifactStore.Open(digest)
	if err != nil {
		http.Error(w, `{"error":"artifact not found"}`, http.StatusNotFound)
		return
	}
	defer reader.Close()
	if metadata.MediaType != "" {
		w.Header().Set("Content-Type", metadata.MediaType)
	}
	w.Header().Set("ETag", `"sha256:`+metadata.Digest+`"`)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Length", strconv.FormatInt(metadata.Size, 10))
	_, _ = io.Copy(w, reader)
}
