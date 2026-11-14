package server

import (
	"encoding/json"
	"net/http"
	"sort"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type conversationHistoryEntry struct {
	at       time.Time
	messages []api.ChatMessage
}

type conversationHistory struct {
	entries      []conversationHistoryEntry
	dependencies []string
	inputs       map[string]api.ConversationInboxEntry
	before       int64
}

func (h *conversationHistory) add(at time.Time, messages ...api.ChatMessage) {
	h.entries = append(h.entries, conversationHistoryEntry{at, messages})
}

func (h *conversationHistory) sources(config agents.AgentConfig) {
	h.dependencies = append(h.dependencies, config.KnowledgeDependencies...)
	for _, doc := range config.KnowledgeDocuments {
		h.dependencies = append(h.dependencies, doc.ID)
	}
	for _, source := range config.KnowledgeSources {
		h.dependencies = append(h.dependencies, source.DocumentID)
	}
}

func (h *conversationHistory) apply(config *agents.AgentConfig) {
	sort.SliceStable(h.entries, func(i, j int) bool { return h.entries[i].at.Before(h.entries[j].at) })
	for _, entry := range h.entries {
		config.ConversationContext = append(config.ConversationContext, entry.messages...)
	}
	seen := map[string]bool{}
	for _, id := range config.KnowledgeDependencies {
		seen[id] = true
	}
	for _, id := range h.dependencies {
		if id != "" && !seen[id] {
			config.KnowledgeDependencies = append(config.KnowledgeDependencies, id)
			seen[id] = true
		}
	}
}

// One history across imported exchanges, work and foreground turns. Input
// identities, never matching text, suppress duplicated prompts/corrections.
// All retrieval is observation-only; runner context accounting remains final.
func (s *Server) assembleCanonicalHistory(r *http.Request, c *agents.Conversation, beforeInput string, config *agents.AgentConfig) error {
	h := &conversationHistory{inputs: map[string]api.ConversationInboxEntry{}}
	scope := s.controlScope(r)
	if beforeInput != "" {
		input, err := s.agentManager.ConversationInput(scope, c.ID, beforeInput)
		if err != nil {
			return err
		}
		h.before = input.Sequence
	}
	for after := int64(0); ; {
		if err := r.Context().Err(); err != nil {
			return err
		}
		page, err := s.agentManager.ConversationInbox(scope, c.ID, after, 100)
		if err != nil {
			return err
		}
		for _, input := range page.Items {
			h.inputs[input.ID] = input
		}
		if !page.HasMore {
			break
		}
		after = page.NextCursor
	}
	if source, err := s.agentManager.ImportedConversationSource(scope, c.ID); err != nil {
		return err
	} else if len(source) > 0 {
		var legacy sessions.Session
		if err = json.Unmarshal(source, &legacy); err != nil {
			return agents.ErrRunStorage
		}
		if err = s.collectConversationHistory(&legacy, scope.Actor, true, h); err != nil {
			return err
		}
	}
	runs, err := s.agentManager.ConversationRuns(scope, c.ID)
	if err != nil {
		return err
	}
	work := &sessions.Session{Name: c.ID}
	for _, id := range runs {
		if task, found := s.agentManager.GetTask(id); found && task.Origin != nil {
			work.Jobs = append(work.Jobs, sessions.ConversationJobLink{JobID: id, TurnID: task.Origin.TurnID})
		}
	}
	if err = s.collectConversationHistory(work, scope.Actor, true, h); err != nil {
		return err
	}
	inputs := make([]api.ConversationInboxEntry, 0, len(h.inputs))
	for _, input := range h.inputs {
		if h.before == 0 || input.Sequence < h.before {
			inputs = append(inputs, input)
		}
	}
	sort.Slice(inputs, func(i, j int) bool { return inputs[i].Sequence < inputs[j].Sequence })
	for _, input := range inputs {
		if err = s.collectConversationInputHistory(scope, c.ID, input, h); err != nil {
			return err
		}
	}
	current, err := s.agentManager.Conversation(scope, c.ID)
	if err != nil {
		return err
	}
	if current.Revision != c.Revision {
		return agents.ErrRunConflict
	}
	h.apply(config)
	return nil
}
