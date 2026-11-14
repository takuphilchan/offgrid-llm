package server

import (
	"fmt"
	"net/http"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Inbox history is independent of work identity. A foreground answer must not
// disappear just because it created no work item. Delivery uses a strict input
// boundary so later queued messages cannot steer an earlier turn accidentally.
// Pagination bounds each store read; the runner budgets the assembled context.
func (s *Server) assembleConversationInboxContext(r *http.Request, conversation, beforeInput string, config *agents.AgentConfig) error {
	scope := s.controlScope(r)
	before := int64(0)
	if beforeInput != "" {
		entry, err := s.agentManager.ConversationInput(scope, conversation, beforeInput)
		if err != nil {
			return err
		}
		before = entry.Sequence
	}
	after := int64(0)
	for {
		if err := r.Context().Err(); err != nil {
			return err
		}
		page, err := s.agentManager.ConversationInbox(scope, conversation, after, 100)
		if err != nil {
			return err
		}
		for _, input := range page.Items {
			if before > 0 && input.Sequence >= before {
				return nil
			}
			if err = s.appendConversationInputContext(scope, conversation, input, config); err != nil {
				return err
			}
		}
		if !page.HasMore {
			return nil
		}
		after = page.NextCursor
	}
}

func (s *Server) appendConversationInputContext(scope agents.ControlScope, conversation string, input api.ConversationInboxEntry, config *agents.AgentConfig) error {
	h := &conversationHistory{}
	if err := s.collectConversationInputHistory(scope, conversation, input, h); err != nil {
		return err
	}
	h.apply(config)
	return nil
}

func (s *Server) collectConversationInputHistory(scope agents.ControlScope, conversation string, input api.ConversationInboxEntry, h *conversationHistory) error {
	at := input.AcceptedAt
	if input.StoppedAt != nil {
		h.add(*input.StoppedAt, api.ChatMessage{Role: "assistant", Content: fmt.Sprintf("Recorded user control: this input was withdrawn. Retained historical text, not a current instruction: %q. Do not apply its decision or infer approval from it.", input.Input.Content)})
		return nil
	}
	if s.conversationQuestionSourceAllowed(scope, input.Question) {
		h.add(at, api.ChatMessage{Role: "assistant", Content: input.Question.Content})
	}
	h.add(at, api.ChatMessage{Role: "user", Content: input.Input.Content})
	if input.WorkID != "" && input.AppliedAt == nil {
		h.add(at, api.ChatMessage{Role: "assistant", Content: "Recorded conversation state: this input was accepted for focused work but has not been incorporated. Do not represent its requested changes as applied."})
	}
	if input.RunID == "" {
		if input.AppliedAt != nil && input.Incorporation == "worker_answer" {
			h.add(*input.AppliedAt, api.ChatMessage{Role: "assistant", Content: fmt.Sprintf("Recorded conversation state: this answer was accepted for saved question %s in work %s. It did not create a new execution attempt or grant access. Worker delivery/application is recorded separately.", input.Input.QuestionID, input.WorkID)})
			return nil
		}
		h.add(at, api.ChatMessage{Role: "assistant", Content: "Recorded conversation state: this input is queued and has no execution attempt."})
		return nil
	}
	task, ok := s.agentManager.GetTask(input.RunID)
	if !ok || task.DeletedAt != nil || task.Actor != scope.Actor || task.Origin == nil || task.Origin.Workspace != scope.Workspace || task.Origin.Conversation != conversation || task.ConversationTurn == nil || task.ConversationTurn.InputID != input.ID {
		return agents.ErrControlNotFound
	}
	if input.SettledAt != nil {
		at = *input.SettledAt
	}
	if !s.taskSourceAllowed(task) {
		h.add(at, api.ChatMessage{Role: "assistant", Content: "A prior conversation result and its derived context are unavailable because source access changed. Retain the original user input; do not reconstruct the withheld result."})
	} else {
		if task.Result != "" {
			h.add(at, api.ChatMessage{Role: "assistant", Content: task.Result})
		}
		if task.Status != agents.TaskCompleted {
			h.add(at, api.ChatMessage{Role: "assistant", Content: fmt.Sprintf("Recorded conversation attempt state: %s. Limitation: %s. Do not represent this attempt as completed.", task.Status, task.Error)})
		}
		h.sources(task.Config)
	}
	if input.AppliedAt != nil {
		at = *input.AppliedAt
	}
	if input.Incorporation == "correction" && input.AppliedAt != nil {
		h.add(at, api.ChatMessage{Role: "assistant", Content: fmt.Sprintf("Recorded control receipt: user input %s was applied to work instruction version %d. This supersedes an earlier reply saying it was pending; it does not prove execution or task completion.", input.ID, input.AppliedInstructionVersion)})
	} else if input.Decision != nil && input.AppliedAt == nil && input.Decision.Kind == "correction" {
		h.add(at, api.ChatMessage{Role: "assistant", Content: "Recorded control receipt: this correction decision remains pending, not applied. A completed foreground reply is not a work mutation receipt."})
	} else if input.Incorporation == "new_work" && input.AppliedAt != nil {
		h.add(at, api.ChatMessage{Role: "assistant", Content: "Recorded admission receipt: this input created work " + input.CreatedWorkID + ". Admission is not proof of execution or successful completion; use the recorded work outcome."})
	} else if input.Decision != nil && input.Decision.Kind == "new_work" {
		h.add(at, api.ChatMessage{Role: "assistant", Content: "Recorded control receipt: the new-work decision is pending admission. The foreground reply is not proof that ongoing work started."})
	}
	return nil
}
