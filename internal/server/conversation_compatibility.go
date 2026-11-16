package server

import (
	"encoding/json"
	"net/http"
	"sort"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
)

// The retained JSON is import evidence. Legacy reads project canonical state;
// a failed projection never changes a command receipt or dispatch eligibility.
func (s *Server) canonicalSessionProjection(r *http.Request, name string) (*sessions.Session, bool, error) {
	manifest, err := s.agentManager.ImportedConversation(s.controlScope(r), name)
	if err != nil {
		return nil, false, err
	}
	if manifest == nil {
		return nil, false, nil
	}
	deleted, err := s.agentManager.ConversationDeleted(s.controlScope(r), manifest.ConversationID)
	if err != nil {
		return nil, true, err
	}
	if deleted {
		return nil, true, sessions.ErrSessionNotFound
	}
	if err := s.admissionStage("canonical_projection"); err != nil {
		return nil, true, err
	}
	current, err := s.agentManager.Conversation(s.controlScope(r), manifest.ConversationID)
	if err != nil {
		return nil, true, err
	}
	source, err := s.agentManager.ImportedConversationSource(s.controlScope(r), current.ID)
	if err != nil {
		return nil, true, err
	}
	var session sessions.Session
	if err = json.Unmarshal(source, &session); err != nil {
		return nil, true, err
	}
	if err = s.sessionHandlers.scoped(r).CheckHistory(&session); err != nil {
		return nil, true, err
	}
	session.CanonicalID, session.ControlRevision = current.ID, current.Revision
	inputs, err := s.projectConversationInputs(r, current.ID, &session)
	if err != nil {
		return nil, true, err
	}
	ids, err := s.agentManager.ConversationRuns(s.controlScope(r), current.ID)
	if err != nil {
		return nil, true, err
	}
	var runs []*agents.Task
	for _, id := range ids {
		task, found := s.agentManager.GetTask(id)
		if !found || task.DeletedAt != nil {
			continue
		}
		if !s.conversationContainsTask(s.agentActor(r), current.ID, task) || !s.taskKnowledgeAllowed(task.Actor, task) {
			return nil, true, sessions.ErrKnowledgeUnavailable
		}
		runs = append(runs, task)
	}
	sort.SliceStable(runs, func(i, j int) bool { return runs[i].CreatedAt.Before(runs[j].CreatedAt) })
	for _, task := range runs {
		turn := task.Origin.TurnID
		session.Jobs = append(session.Jobs, sessions.ConversationJobLink{TurnID: turn, JobID: task.ID})
		input, fromInput := inputs[turn]
		if !fromInput || input.CreatedWorkID != task.WorkID || task.Origin.Conversation != current.ID {
			session.Messages = append(session.Messages, sessions.Message{Role: "user", Content: task.Prompt, Timestamp: task.CreatedAt, JobID: task.ID, TurnID: turn})
		}
		for _, instruction := range task.Instructions {
			input, fromInput := inputs[instruction.RequestID]
			if fromInput && input.WorkID == task.WorkID && input.Incorporation == "correction" && input.AppliedAt != nil {
				continue
			}
			session.Messages = append(session.Messages, sessions.Message{Role: "user", Content: instruction.Text, Timestamp: instruction.At, JobID: task.ID, TurnID: turn})
		}
		if task.Result != "" {
			message := sessions.Message{Role: "assistant", Content: task.Result, JobID: task.ID, TurnID: turn, Timestamp: task.CreatedAt, KnowledgeSources: task.Config.KnowledgeSources, KnowledgeDocuments: append([]string{}, task.Config.KnowledgeDependencies...)}
			for _, doc := range task.Config.KnowledgeDocuments {
				message.KnowledgeDocuments = append(message.KnowledgeDocuments, doc.ID)
			}
			if task.CompletedAt != nil {
				message.Timestamp = *task.CompletedAt
			}
			session.Messages = append(session.Messages, message)
		}
		session.Turn = &sessions.Turn{ID: turn, JobID: task.ID, JobAdmission: "linked", Prompt: task.Prompt, Model: task.Model, Status: string(task.Status), Phase: string(task.Status), Output: task.Result, Error: task.Error}
		if task.Config.Profile != nil {
			session.Turn.Agent = &sessions.AgentSelection{ID: task.Config.Profile.ID, Revision: task.Config.Profile.Revision}
		}
		session.ModelID = task.Model
		updateProjectedTime(&session, task.CreatedAt)
		if task.CompletedAt != nil {
			updateProjectedTime(&session, *task.CompletedAt)
		}
	}
	sort.SliceStable(session.Messages, func(i, j int) bool { return session.Messages[i].Timestamp.Before(session.Messages[j].Timestamp) })
	// Paging and task reads can overlap an input/application command. Do not
	// return a mixture that duplicates input or claims an older revision.
	latest, err := s.agentManager.Conversation(s.controlScope(r), current.ID)
	if err != nil {
		return nil, true, err
	}
	if latest.Revision != current.Revision {
		return nil, true, sessions.ErrTurnConflict
	}
	return &session, true, nil
}

func (h *SessionHandlers) projectedSession(r *http.Request, name string) (*sessions.Session, error) {
	if h.agentServer != nil && h.agentServer.agentManager != nil {
		projected, canonical, err := h.agentServer.canonicalSessionProjection(r, name)
		if err != nil {
			return nil, err
		}
		if canonical {
			return projected, nil
		}
	}
	return h.scoped(r).Load(name)
}

func (h *SessionHandlers) projectedControlState(r *http.Request, name string) (*sessions.Session, error) {
	if h.agentServer == nil || h.agentServer.agentManager == nil {
		return h.scoped(r).ControlState(name)
	}
	s := h.agentServer
	manifest, err := s.agentManager.ImportedConversation(s.controlScope(r), name)
	if err != nil {
		return nil, err
	}
	if manifest == nil {
		return h.scoped(r).ControlState(name)
	}
	conversation, err := s.agentManager.Conversation(s.controlScope(r), manifest.ConversationID)
	if err != nil {
		return nil, err
	}
	if conversation.FocusedWorkID == "" {
		return &sessions.Session{Name: name, CanonicalID: conversation.ID, ControlRevision: conversation.Revision}, nil
	}
	work, err := s.agentManager.WorkItem(s.controlScope(r), conversation.FocusedWorkID)
	if err != nil {
		return nil, err
	}
	task, found := s.agentManager.GetTask(work.RunID)
	if !found || task.Actor != s.agentActor(r) || task.Origin == nil {
		return nil, agents.ErrControlNotFound
	}
	return &sessions.Session{Name: name, CanonicalID: conversation.ID, ControlRevision: conversation.Revision, Turn: &sessions.Turn{ID: task.Origin.TurnID, JobID: task.ID, JobAdmission: "linked", Status: string(task.Status)}}, nil
}

func (h *SessionHandlers) rejectCanonicalLegacyDelete(w http.ResponseWriter, r *http.Request, name string) bool {
	if h.agentServer == nil || h.agentServer.agentManager == nil {
		return false
	}
	manifest, err := h.agentServer.agentManager.ImportedConversation(h.agentServer.controlScope(r), name)
	if err != nil {
		writeControlError(w, err)
		return true
	}
	if manifest != nil {
		if r.URL.Query().Get("canonical_id") != "" {
			h.agentServer.deleteCanonicalConversation(w, r, name, manifest)
			return true
		}
		writeSessionError(w, sessions.ErrCanonicalConversation)
		return true
	}
	if r.URL.Query().Get("canonical_id") != "" {
		writeControlError(w, agents.ErrControlNotFound)
		return true
	}
	return false
}
