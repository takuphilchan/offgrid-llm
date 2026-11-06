package server

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
)

type workCursor struct {
	CreatedAt time.Time `json:"time"`
	ID        string    `json:"id"`
}
type workPage struct {
	Items          []agents.WorkSummary `json:"items"`
	Counts         map[string]int       `json:"counts"`
	RemovableCount int                  `json:"removable_count"`
	NextCursor     string               `json:"next_cursor,omitempty"`
	HasMore        bool                 `json:"has_more"`
	Scope          string               `json:"scope"`
}

func workBucket(status agents.TaskStatus) string {
	switch status {
	case agents.TaskCompleted, agents.TaskFailed, agents.TaskCancelled:
		return "history"
	case agents.TaskRunning, agents.TaskPending, "queued", "waiting_for_children":
		return "running"
	default:
		return "attention"
	}
}

// handleWorkSummary is an additive, read-only view. Authorization is evaluated
// before counting; history metadata never grants permission to cancel/delete.
func (s *Server) handleWorkSummary(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "Method not allowed", 405)
		return
	}
	if s.agentManager == nil {
		writeJobError(w, 503, "work_unavailable", "Work status is unavailable.", false)
		return
	}
	if err := s.agentManager.StorageError(); err != nil {
		writeAgentError(w, err)
		return
	}
	q := r.URL.Query()
	bucket := q.Get("bucket")
	if bucket == "" {
		bucket = "all"
	}
	limit := 20
	if q.Has("limit") {
		n, err := strconv.Atoi(q.Get("limit"))
		if err != nil || n < 1 || n > 100 {
			writeJobError(w, 400, "invalid_work_query", "Page size must be between 1 and 100.", false)
			return
		}
		limit = n
	}
	if bucket != "all" && bucket != "attention" && bucket != "running" && bucket != "history" {
		writeJobError(w, 400, "invalid_work_query", "Choose attention, running, history or all.", false)
		return
	}
	search := strings.ToLower(strings.TrimSpace(q.Get("query")))
	if len(search) > 512 {
		writeJobError(w, 400, "invalid_work_query", "Search is too long.", false)
		return
	}
	var cursor *workCursor
	if raw := q.Get("cursor"); raw != "" {
		if len(raw) > 512 {
			writeJobError(w, 400, "invalid_work_cursor", "Refresh the work list before continuing.", false)
			return
		}
		var c workCursor
		data, err := base64.RawURLEncoding.DecodeString(raw)
		if len(raw) > 512 || err != nil || json.Unmarshal(data, &c) != nil || c.ID == "" || len(c.ID) > 128 || c.CreatedAt.IsZero() {
			writeJobError(w, 400, "invalid_work_cursor", "Refresh the work list before continuing.", false)
			return
		}
		cursor = &c
	}
	actor, admin := s.agentActor(r), s.profileAdmin(r)
	page := workPage{Items: []agents.WorkSummary{}, Counts: map[string]int{"attention": 0, "running": 0, "history": 0, "all": 0}, Scope: "actor_jobs"}
	rows := []agents.WorkSummary{}
	for _, row := range s.agentManager.ListWorkSummaries() {
		if row.Actor != "" && row.Actor != actor || !admin && (!row.ProfilePresent || row.Actor != actor) || row.UnscopedKnowledge || !s.savedKnowledgeAllowed(actor, row.SourceIDs) {
			continue
		}
		if row.Actor != actor && !(row.Actor == "" && actor == "local-admin") {
			row.Deletable = false
			row.Protection = "ownership"
		}
		kind := workBucket(row.Status)
		page.Counts[kind]++
		page.Counts["all"]++
		if q.Get("id") != "" && q.Get("id") != row.ID || (bucket != "all" && kind != bucket) || search != "" && !strings.Contains(strings.ToLower(row.Title), search) {
			continue
		}
		if row.Deletable {
			page.RemovableCount++
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].CreatedAt.Equal(rows[j].CreatedAt) {
			return rows[i].ID < rows[j].ID
		}
		return rows[i].CreatedAt.After(rows[j].CreatedAt)
	})
	for _, row := range rows {
		if cursor != nil && (row.CreatedAt.After(cursor.CreatedAt) || row.CreatedAt.Equal(cursor.CreatedAt) && row.ID <= cursor.ID) {
			continue
		}
		if len(page.Items) == limit {
			page.HasMore = true
			break
		}
		page.Items = append(page.Items, row)
	}
	if page.HasMore {
		last := page.Items[len(page.Items)-1]
		data, _ := json.Marshal(workCursor{last.CreatedAt, last.ID})
		page.NextCursor = base64.RawURLEncoding.EncodeToString(data)
	}
	writeJSON(w, 200, page)
}
