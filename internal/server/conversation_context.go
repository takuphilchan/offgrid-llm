package server

import (
	"fmt"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Read linked canonical jobs as well as the legacy success-only transcript.
// This is observation: it neither repairs admissions nor resumes execution.
func (s *Server) assembleConversationContext(conversation *sessions.Session, actor string, config *agents.AgentConfig) error {
	return s.assembleConversationContextPolicy(conversation, actor, config, false)
}

func (s *Server) assembleConversationContextPolicy(conversation *sessions.Session, actor string, config *agents.AgentConfig, retainAuthorized bool) error {
	h := &conversationHistory{}
	if err := s.collectConversationHistory(conversation, actor, retainAuthorized, h); err != nil {
		return err
	}
	h.apply(config)
	return nil
}

func (s *Server) collectConversationHistory(conversation *sessions.Session, actor string, retainAuthorized bool, h *conversationHistory) error {
	linked := map[string]bool{}
	for _, link := range conversation.Jobs {
		if linked[link.JobID] {
			continue
		}
		task, exists := s.agentManager.GetTask(link.JobID)
		if !exists || task.DeletedAt != nil {
			continue
		}
		if !s.conversationContainsTask(actor, conversation.Name, task) || task.Origin.TurnID != link.TurnID {
			return fmt.Errorf("a source used by previous conversation work is no longer available")
		}
		authorized := s.taskKnowledgeAllowed(actor, task)
		if !authorized && !retainAuthorized {
			return fmt.Errorf("a source used by previous conversation work is no longer available")
		}
		linked[link.JobID] = true
		input, fromInput := h.inputs[task.Origin.TurnID]
		fromInput = fromInput && input.CreatedWorkID == task.WorkID && task.Origin.Conversation == conversation.Name
		if fromInput && h.before > 0 && input.Sequence >= h.before {
			continue
		}
		if !fromInput {
			h.add(task.CreatedAt, api.ChatMessage{Role: "user", Content: task.Prompt})
		}
		futureInstruction := false
		for _, instruction := range task.Instructions {
			input, fromInput := h.inputs[instruction.RequestID]
			if fromInput && input.WorkID == task.WorkID && input.Incorporation == "correction" && input.AppliedAt != nil {
				futureInstruction = futureInstruction || h.before > 0 && input.Sequence >= h.before
				continue
			}
			h.add(instruction.At, api.ChatMessage{Role: "user", Content: instruction.Text})
		}
		// Accepted but unapplied corrections must not masquerade as applied.
		for _, instruction := range task.PendingInstructions {
			h.add(instruction.At, api.ChatMessage{Role: "user", Content: instruction.Text}, api.ChatMessage{Role: "assistant", Content: "Recorded state: this correction was accepted but was not applied to the prior attempt."})
		}
		at := task.CreatedAt
		if task.CompletedAt != nil {
			at = *task.CompletedAt
		} else {
			at = time.Now().UTC()
		}
		if futureInstruction {
			h.add(at, api.ChatMessage{Role: "assistant", Content: "A work result depends on newer conversation input and is excluded from this earlier turn."})
			continue
		}
		if !authorized {
			h.add(at, api.ChatMessage{Role: "assistant", Content: "A prior result and its derived context are unavailable because source access changed. The user's original request and corrections remain applicable; do not reconstruct the withheld result."})
			continue
		}
		if task.Result != "" {
			h.add(at, api.ChatMessage{Role: "assistant", Content: task.Result})
		}
		if task.Status != agents.TaskCompleted {
			h.add(at, api.ChatMessage{Role: "assistant", Content: fmt.Sprintf("Recorded prior attempt state: %s. Limitation: %s. Its effects and results must not be represented as successful completion.", task.Status, task.Error)})
		}
		h.sources(task.Config)
	}
	for _, message := range conversation.Messages {
		if retainAuthorized && (linked[message.JobID] || (message.Role != "user" && message.Role != "assistant")) {
			continue
		}
		if retainAuthorized {
			ids := append([]string{}, message.KnowledgeDocuments...)
			for _, source := range message.KnowledgeSources {
				ids = append(ids, source.DocumentID)
			}
			if !s.savedKnowledgeAllowed(actor, ids) {
				if message.Role == "user" {
					h.add(message.Timestamp, api.ChatMessage{Role: "user", Content: message.Content})
				}
				h.add(message.Timestamp, api.ChatMessage{Role: "assistant", Content: "Historical source-dependent content is unavailable because access changed."})
				continue
			}
		}
		h.dependencies = append(h.dependencies, message.KnowledgeDocuments...)
		for _, source := range message.KnowledgeSources {
			h.dependencies = append(h.dependencies, source.DocumentID)
		}
		if linked[message.JobID] || (message.Role != "user" && message.Role != "assistant") {
			continue
		}
		content := message.Content
		if len(message.Images) > 0 {
			content += unavailableHistoricalImages
		}
		h.add(message.Timestamp, api.ChatMessage{Role: message.Role, Content: content})
	}
	return nil
}
