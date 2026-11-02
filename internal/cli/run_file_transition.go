package cli

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type FileConversationStarter func(context.Context, string, []ChatMessage) (*ServiceFileConversation, error)

// A stable identity makes an uncertain create recoverable without duplicating
// history. Only the completed conversation is copied; no generation is replayed.
func newFileConversationStarter(env Environment) FileConversationStarter {
	name := "files-" + rand.Text()
	return func(ctx context.Context, model string, messages []ChatMessage) (*ServiceFileConversation, error) {
		history, err := seedChatMessages(messages)
		if err != nil {
			return nil, err
		}
		client, err := serviceClient(env)
		if err != nil {
			return nil, err
		}
		for _, capability := range []string{"session-files-v1", "session-history-seed-v1"} {
			if err = client.RequireCapability(ctx, capability); err != nil {
				return nil, err
			}
		}
		var saved sessions.Session
		err = client.JSON(ctx, http.MethodPost, "/v1/sessions", map[string]any{"name": name, "model_id": model, "history": history}, &saved)
		if err != nil {
			// A lost response may follow a durable create. Observe, never append or
			// overwrite; a conflicting conversation must not acquire these files.
			if readErr := client.JSON(ctx, http.MethodGet, "/v1/sessions/"+url.PathEscape(name), nil, &saved); readErr != nil {
				return nil, fmt.Errorf("could not prepare file chat %s: %w; current history is unchanged", name, err)
			}
		}
		if saved.Name != name || saved.ModelID != model || saved.Turn != nil || len(saved.Jobs) != 0 || !sameSeedHistory(saved.Messages, history) {
			return nil, fmt.Errorf("saved conversation %s does not match this history; no file was uploaded", name)
		}
		return &ServiceFileConversation{Context: ctx, Client: client, Name: name, Saved: &saved}, nil
	}
}

func sameSeedHistory(a, b []sessions.Message) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Role != b[i].Role || a[i].Content != b[i].Content || !reflect.DeepEqual(a[i].Images, b[i].Images) ||
			a[i].JobID != "" || a[i].TurnID != "" || len(a[i].Attachments)+len(a[i].KnowledgeSources)+len(a[i].KnowledgeDocuments) != 0 {
			return false
		}
	}
	return true
}

func seedChatMessages(messages []ChatMessage) ([]sessions.Message, error) {
	history := make([]sessions.Message, 0, len(messages))
	for _, message := range messages {
		m := sessions.Message{Role: message.Role}
		if text, ok := message.Content.(string); ok {
			m.Content = text
		} else {
			data, err := json.Marshal(message.Content)
			if err != nil {
				return nil, err
			}
			var parts []api.ChatContentPart
			if err := json.Unmarshal(data, &parts); err != nil {
				return nil, err
			}
			for _, part := range parts {
				switch part.Type {
				case "text":
					m.Content += part.Text
				case "image_url":
					if part.ImageURL == nil {
						return nil, sessions.ErrInvalidSeedHistory
					}
					m.Images = append(m.Images, api.SessionImage{Name: fmt.Sprintf("image-%d", len(m.Images)+1), DataURL: part.ImageURL.URL})
				default:
					return nil, sessions.ErrInvalidSeedHistory
				}
			}
		}
		history = append(history, m)
	}
	return history, sessions.ValidateSeedHistory(history)
}

// Listing/removing an empty draft must not create a saved conversation. Check
// an explicit local path before any service mutation, preserving its spelling.
func FileCommandNeedsSession(input string) (bool, string, error) {
	command, arg, _ := strings.Cut(strings.TrimSpace(strings.TrimPrefix(input, "/")), " ")
	if command == "attach" {
		if _, _, err := ReadChatFile(conversationFilePath(arg)); err != nil {
			return false, "", err
		}
		return true, "", nil
	}
	return false, "No attached files. Use /attach PATH to add a document.", nil
}
