package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Compatibility reads derive their transcript from the canonical inbox. They
// never write the retained import file or admit, settle or resume execution.
func (s *Server) projectConversationInputs(r *http.Request, conversation string, session *sessions.Session) (map[string]api.ConversationInboxEntry, error) {
	inputs := map[string]api.ConversationInboxEntry{}
	after := int64(0)
	for {
		if err := r.Context().Err(); err != nil {
			return nil, err
		}
		page, err := s.agentManager.ConversationInbox(s.controlScope(r), conversation, after, 100)
		if err != nil {
			return nil, err
		}
		for _, input := range page.Items {
			inputs[input.ID] = input
			if err := s.projectConversationInput(r, input, session); err != nil {
				return nil, err
			}
		}
		if !page.HasMore {
			return inputs, nil
		}
		after = page.NextCursor
	}
}

func (s *Server) projectConversationInput(r *http.Request, input api.ConversationInboxEntry, session *sessions.Session) error {
	if s.conversationQuestionSourceAllowed(s.controlScope(r), input.Question) {
		session.Messages = append(session.Messages, sessions.Message{Role: "assistant", Content: input.Question.Content, Timestamp: input.AcceptedAt, TurnID: input.ID})
	}
	message := sessions.Message{Role: "user", Content: input.Input.Content, Timestamp: input.AcceptedAt, TurnID: input.ID, JobID: input.RunID}
	session.Messages = append(session.Messages, message)
	updateProjectedTime(session, input.AcceptedAt)
	turn := &sessions.Turn{ID: input.ID, JobID: input.RunID, Prompt: input.Input.Content, Status: input.State, Phase: input.State}
	session.Turn = turn
	if input.RunID != "" {
		task, found := s.agentManager.GetTask(input.RunID)
		if !found || task.DeletedAt != nil || task.Actor != s.agentActor(r) || task.Origin == nil || task.Origin.Workspace != s.workspaceID || task.Origin.Conversation != input.ConversationID || task.ConversationTurn == nil || task.ConversationTurn.InputID != input.ID {
			return agents.ErrControlNotFound
		}
		session.Jobs = append(session.Jobs, sessions.ConversationJobLink{TurnID: input.ID, JobID: task.ID})
		turn.JobAdmission, turn.Model = "linked", task.Model
		if input.StoppedAt == nil {
			turn.Status, turn.Phase = string(task.Status), string(task.Status)
		}
		session.ModelID = task.Model
		message.Role, message.Content, message.Timestamp = "assistant", task.Result, task.CreatedAt
		if task.CompletedAt != nil {
			message.Timestamp = *task.CompletedAt
		}
		if !s.taskSourceAllowed(task) {
			message.Content = "This reply is unavailable because source access changed."
		} else {
			turn.Output, turn.Error = task.Result, task.Error
			message.KnowledgeSources = task.Config.KnowledgeSources
			message.KnowledgeDocuments = append([]string{}, task.Config.KnowledgeDependencies...)
			for _, doc := range task.Config.KnowledgeDocuments {
				message.KnowledgeDocuments = append(message.KnowledgeDocuments, doc.ID)
			}
		}
		if message.Content != "" {
			session.Messages = append(session.Messages, message)
			updateProjectedTime(session, message.Timestamp)
		}
	}
	// State receipts remain separate from generated prose. A pending reply must
	// not conceal a later correction, withdrawal or failed attempt on reload.
	status, at := "", input.AcceptedAt
	switch {
	case input.StoppedAt != nil:
		status, at = "This input was withdrawn; its proposed decision will not be applied.", *input.StoppedAt
	case input.AppliedAt != nil && input.Incorporation == "correction":
		status, at = fmt.Sprintf("Correction applied to work instruction version %d.", input.AppliedInstructionVersion), *input.AppliedAt
	case input.AppliedAt != nil && input.Incorporation == "new_work":
		status, at = "Ongoing work was accepted. Its result is recorded separately.", *input.AppliedAt
	case input.AppliedAt != nil && input.Incorporation == "worker_answer":
		status, at = "Your answer was accepted. Work can continue.", *input.AppliedAt
	case input.State == "failed":
		status = "This conversation attempt did not complete."
		if input.SettledAt != nil {
			at = *input.SettledAt
		}
	}
	if status != "" {
		session.Messages = append(session.Messages, sessions.Message{Role: "assistant", Content: status, Timestamp: at, TurnID: input.ID, JobID: input.RunID})
		updateProjectedTime(session, at)
	}
	return nil
}

func updateProjectedTime(session *sessions.Session, at time.Time) {
	if at.After(session.UpdatedAt) {
		session.UpdatedAt = at
	}
}
