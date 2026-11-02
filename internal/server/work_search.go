package server

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/takuphilchan/offgrid-llm/internal/users"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func (s *Server) handleWorkSearch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		writeError(w, "Use GET to search saved work", 405)
		return
	}
	user := users.GetUser(r)
	local := s.config == nil || !s.config.RequireAuth
	canSessions := local || user != nil && user.HasPermission(users.PermissionSessions)
	canTasks := local || user != nil && user.HasPermission(users.PermissionChat)
	if !canSessions && !canTasks {
		writeError(w, "Forbidden", 403)
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("query"))
	if !utf8.ValidString(query) || utf8.RuneCountInString(query) < 2 || len(query) > 256 || strings.ContainsAny(query, "\x00\r\n") {
		writeError(w, "Use 2 or more characters, up to 256 UTF-8 bytes", 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	response := api.WorkSearchResponse{Results: []api.WorkSearchResult{}, Scopes: []string{}}
	response.Partial = canSessions && s.sessionHandlers == nil || canTasks && s.agentManager == nil
	if canSessions && s.sessionHandlers != nil {
		response.Scopes = append(response.Scopes, "conversation_names")
		results, partial, err := s.sessionHandlers.scoped(r).SearchNames(ctx, query, 20)
		response.Results = append(response.Results, results...)
		response.Partial = response.Partial || partial || err != nil
	}
	if canTasks && s.agentManager != nil {
		response.Scopes = append(response.Scopes, "task_requests")
		results, partial, err := s.agentManager.SearchRequests(ctx, s.agentActor(r), s.profileAdmin(r), query, 20)
		for _, item := range results {
			task, ok := s.agentManager.GetTask(item.ID)
			if ok && s.taskKnowledgeAllowed(s.agentActor(r), task) {
				response.Results = append(response.Results, item)
			}
		}
		response.Partial = response.Partial || partial || err != nil
	}
	if r.Context().Err() != nil {
		return
	}
	sort.Slice(response.Results, func(i, j int) bool {
		a, b := response.Results[i], response.Results[j]
		if a.UpdatedAt.Equal(b.UpdatedAt) {
			return a.Kind+a.ID < b.Kind+b.ID
		}
		return a.UpdatedAt.After(b.UpdatedAt)
	})
	if len(response.Results) > 20 {
		response.Results = response.Results[:20]
		response.Partial = true
	}
	writeJSON(w, 200, response)
}
