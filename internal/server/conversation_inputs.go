package server

import (
	"net/http"
	"strconv"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Observation of accepted/delivered/applied state is independent of preparation
// and execution. No GET can create a run, reset backoff or release a work hold.
func (s *Server) handleConversationInputs(w http.ResponseWriter, r *http.Request, conversation string, tail []string) {
	if len(tail) == 1 {
		entry, err := s.agentManager.ConversationInput(s.controlScope(r), conversation, tail[0])
		if err != nil {
			writeControlError(w, err)
			return
		}
		writeJSON(w, 200, s.publicConversationInput(r, *entry))
		return
	}
	after, limit := int64(0), 100
	var err error
	if raw := r.URL.Query().Get("after"); raw != "" {
		after, err = strconv.ParseInt(raw, 10, 64)
	}
	if raw := r.URL.Query().Get("limit"); err == nil && raw != "" {
		limit, err = strconv.Atoi(raw)
	}
	if err != nil {
		writeControlError(w, agents.ErrControlCommand)
		return
	}
	page, err := s.agentManager.ConversationInbox(s.controlScope(r), conversation, after, limit)
	if err != nil {
		writeControlError(w, err)
		return
	}
	for i := range page.Items {
		page.Items[i] = s.publicConversationInput(r, page.Items[i])
	}
	writeJSON(w, 200, page)
}

// Model-authored acceptance text is derived context too. Keep the user's input
// and control state inspectable when access changes, but withhold derived data.
func (s *Server) publicConversationInput(r *http.Request, entry api.ConversationInboxEntry) api.ConversationInboxEntry {
	if entry.Question != nil && !s.conversationQuestionSourceAllowed(s.controlScope(r), entry.Question) {
		entry.Question, entry.ContextUnavailable = nil, true
	}
	if entry.RunID == "" {
		return entry
	}
	task, found := s.agentManager.GetTask(entry.RunID)
	if !found || task.DeletedAt != nil || task.Actor != s.agentActor(r) || task.ConversationTurn == nil || task.ConversationTurn.InputID != entry.ID || task.Origin == nil || task.Origin.Workspace != s.workspaceID || task.Origin.Conversation != entry.ConversationID || !s.taskSourceAllowed(task) {
		entry.Decision, entry.ResultReference, entry.ContextUnavailable = nil, "", true
	}
	return entry
}

func (s *Server) publicInputReceipt(r *http.Request, receipt *agents.ConversationCommandReceipt) *agents.ConversationCommandReceipt {
	copy := *receipt
	if receipt.Input != nil {
		entry := s.publicConversationInput(r, *receipt.Input)
		copy.Input = &entry
	}
	return &copy
}
