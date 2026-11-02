package serviceclient

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type ConversationRequest struct {
	Content     string                         `json:"content"`
	Model       string                         `json:"model_id,omitempty"`
	RequestID   string                         `json:"request_id"`
	Attachments []sessions.AttachmentReference `json:"attachments,omitempty"`
	Images      []api.SessionImage             `json:"images,omitempty"`
	Knowledge   bool                           `json:"use_knowledge_base,omitempty"`
	Stream      bool                           `json:"stream"`
	Durable     bool                           `json:"durable"`
}
type ConversationResponse struct {
	Session *sessions.Session `json:"session"`
	Message *sessions.Message `json:"message"`
}

// Conversation uses the durable service path. Receiving deltas is not completion:
// only the terminal, persisted session response makes the turn successful.
func (c *Client) Conversation(ctx context.Context, name string, input ConversationRequest, delta func(string) error) (ConversationResponse, error) {
	var result ConversationResponse
	if input.RequestID == "" || name == "" {
		return result, fmt.Errorf("conversation and request identity are required")
	}
	if len(input.Attachments) > 0 {
		if err := c.RequireCapability(ctx, "session-files-v1"); err != nil {
			return result, err
		}
	}
	if len(input.Images) > 0 {
		if err := c.RequireCapability(ctx, "session-images-v1"); err != nil {
			return result, err
		}
	}
	input.Stream = true
	input.Durable = true
	data, err := json.Marshal(input)
	if err != nil {
		return result, err
	}
	bounded, cancel := context.WithTimeout(ctx, 31*time.Minute)
	defer cancel()
	stream := *c
	transport := *c.http
	transport.Timeout = 0
	stream.http = &transport
	timer := time.AfterFunc(30*time.Second, cancel)
	defer func() {
		if ctx.Err() != nil {
			stop, done := context.WithTimeout(context.Background(), 5*time.Second)
			defer done()
			_ = c.JSON(stop, http.MethodPost, "/v1/sessions/"+url.PathEscape(name)+"/turn/cancel", map[string]string{"id": input.RequestID}, nil)
		}
	}()
	response, err := stream.Do(bounded, http.MethodPost, "/v1/sessions/"+url.PathEscape(name)+"/generate", "application/json", bytes.NewReader(data))
	timer.Stop()
	if err != nil {
		return result, err
	}
	defer response.Body.Close()
	if !strings.HasPrefix(response.Header.Get("Content-Type"), "text/event-stream") {
		return result, &Error{Code: "invalid_response", Message: "The service did not provide a durable conversation stream. Inspect saved state before retrying."}
	}
	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 4096), 32<<20)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		var event struct {
			Type      string `json:"type"`
			Delta     string `json:"delta"`
			Error     string `json:"error"`
			ErrorCode string `json:"error_code"`
			ConversationResponse
		}
		if json.Unmarshal([]byte(strings.TrimSpace(strings.TrimPrefix(line, "data:"))), &event) != nil {
			return result, &Error{Code: "invalid_response", Message: "Invalid conversation event. Pending files remain saved."}
		}
		switch event.Type {
		case "phase":
		case "delta":
			if delta != nil {
				if err := delta(event.Delta); err != nil {
					return result, err
				}
			}
		case "error":
			message := "The conversation failed. Inspect its saved turn; attached files remain available."
			if safe, ok := modelErrorMessages[event.ErrorCode]; ok {
				message = safe
			}
			return result, &Error{Code: event.ErrorCode, Message: message}
		case "done":
			result = event.ConversationResponse
			if result.Session == nil || result.Session.Name != name || result.Session.Turn == nil || result.Session.Turn.ID != input.RequestID || result.Session.Turn.Status != "completed" || result.Message == nil || result.Message.Role != "assistant" {
				return ConversationResponse{}, &Error{Code: "invalid_response", Message: "The service did not confirm this conversation turn was saved."}
			}
			return result, nil
		default:
			return result, &Error{Code: "invalid_response", Message: "Unknown conversation event; inspect saved state before retrying."}
		}
	}
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	if err := scanner.Err(); err != nil {
		return result, fmt.Errorf("conversation stream interrupted: %w", err)
	}
	return result, &Error{Code: "incomplete_response", Message: "The conversation stream ended without saved completion. Inspect the saved turn before retrying; files remain available."}
}
