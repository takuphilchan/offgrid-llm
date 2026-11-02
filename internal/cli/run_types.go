package cli

import (
	"encoding/json"
	"net/http"

	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// ChatMessage is the legacy terminal-run wire message. The terminal TUI uses
// this same type through a package-main alias during the boundary migration.
type ChatMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

func (m ChatMessage) StringContent() string {
	if m.Content == nil {
		return ""
	}
	if str, ok := m.Content.(string); ok {
		return str
	}
	if parts, ok := m.Content.([]api.ChatContentPart); ok {
		var text string
		for _, part := range parts {
			if part.Type == "text" {
				text += part.Text
			}
		}
		return text
	}
	if parts, ok := m.Content.([]interface{}); ok {
		var text string
		for _, part := range parts {
			if p, ok := part.(map[string]interface{}); ok {
				if kind, ok := p["type"].(string); ok && kind == "text" {
					if value, ok := p["text"].(string); ok {
						text += value
					}
				}
			}
		}
		return text
	}
	return ""
}

func (m ChatMessage) ImageCount() int {
	parts, ok := m.Content.([]api.ChatContentPart)
	if !ok {
		return 0
	}
	count := 0
	for _, part := range parts {
		if part.Type == "image_url" {
			count++
		}
	}
	return count
}

func SavedChatMessages(session *sessions.Session) []ChatMessage {
	var messages []ChatMessage
	for _, message := range session.Messages {
		messages = append(messages, ChatMessage{Role: message.Role, Content: api.SessionContent(message.Content, message.Images)})
	}
	return messages
}

type ChatCompletionRequest struct {
	Model            string        `json:"model"`
	Messages         []ChatMessage `json:"messages"`
	Stream           bool          `json:"stream"`
	UseKnowledgeBase bool          `json:"use_knowledge_base,omitempty"`
}

type ChatCompletionChunk struct {
	Error   json.RawMessage `json:"error,omitempty"`
	Choices []struct {
		Delta struct {
			Content interface{} `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

// InteractiveTUIState transfers only current chat state to the OS-owned TUI.
// The adapter copies mutations back after TUI exit; it does not create another
// session or service-side executor.
type InteractiveTUIState struct {
	StartFiles       FileConversationStarter
	Files            *ServiceFileConversation
	ModelName        string
	ResolvedModel    string
	ServerPort       int
	Client           *http.Client
	UseKnowledgeBase bool
	Images           []api.SessionImage
	Messages         []ChatMessage
	SessionMgr       *sessions.SessionManager
	CurrentSession   *sessions.Session
}
