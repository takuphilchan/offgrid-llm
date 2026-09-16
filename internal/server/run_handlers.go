package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
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
		log.Printf("Run event encoding failed for %s: %v", runID, err)
		return
	}
	// A disconnected HTTP client must not prevent the terminal event from
	// reaching durable storage.
	if ctx == nil || ctx.Err() != nil {
		ctx = context.Background()
	}
	if err := s.runLog.Publish(ctx, event); err != nil {
		log.Printf("Run event projection failed for %s: %v", runID, err)
	}
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
	path := strings.TrimPrefix(r.URL.Path, "/v1/runs")
	path = strings.Trim(path, "/")
	if path != "" {
		parts := strings.Split(path, "/")
		if len(parts) != 2 || parts[1] != "events" || parts[0] == "" {
			http.NotFound(w, r)
			return
		}
		if s.runLog == nil {
			writeError(w, "Run event history unavailable", http.StatusServiceUnavailable)
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

	events := []runs.Event{}
	projectionAvailable := s.runLog != nil
	if s.runLog != nil {
		var err error
		events, err = s.runLog.Replay("", 0)
		if err != nil {
			log.Printf("Run event history unavailable: %v", err)
			projectionAvailable = false
		}
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
		case runs.RunStateChanged:
			var data map[string]any
			if json.Unmarshal(event.Data, &data) == nil {
				if status, ok := data["status"].(string); ok {
					summary.Status = status
				}
				summary.Data = data
			}
		case runs.ApprovalRequired:
			summary.Status = "approval_required"
		case runs.RunCompleted:
			summary.Status = "completed"
		case runs.RunFailed:
			summary.Status = "failed"
		}
	}
	// Checkpoint snapshots override event projections, including recovery states
	// written at startup and state changes whose event projection could not save.
	if s.agentManager != nil {
		if err := s.agentManager.StorageError(); err != nil {
			writeAgentError(w, err)
			return
		}
		for _, task := range s.agentManager.ListTasks() {
			summary := byID[task.ID]
			if summary == nil {
				summary = &runSummary{ID: task.ID, StartedAt: task.CreatedAt, UpdatedAt: task.CreatedAt}
				byID[task.ID] = summary
			}
			summary.Status = string(task.Status)
			summary.Data = map[string]any{"prompt": task.Prompt, "model": task.Model, "actor": task.Actor}
			if task.CompletedAt != nil {
				summary.UpdatedAt = *task.CompletedAt
			}
		}
	}
	summaries := make([]runSummary, 0, len(byID))
	for _, summary := range byID {
		summaries = append(summaries, *summary)
	}
	sort.Slice(summaries, func(i, j int) bool { return summaries[i].UpdatedAt.After(summaries[j].UpdatedAt) })
	json.NewEncoder(w).Encode(map[string]any{"runs": summaries, "event_history_available": projectionAvailable})
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
