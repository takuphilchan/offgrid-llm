package serviceclient

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func ConversationStageID(request, stage string) string {
	digest := sha256.Sum256([]byte(request + "\x00" + stage))
	return "conversation-" + hex.EncodeToString(digest[:])
}

// Import is an explicit service command. This adapter is used only after the
// service advertises new input admission or a session is already canonical.
func (c *Client) ImportSessionConversation(ctx context.Context, name, requestID string) (*api.Conversation, error) {
	receipt, err := c.ConversationCommand(ctx, api.ConversationCommand{RequestID: ConversationStageID(requestID, "import"), Kind: "import_conversation", Payload: api.ConversationCommandPayload{LegacyName: name}})
	if err != nil {
		return nil, err
	}
	if receipt.Conversation == nil {
		return nil, &Error{Code: "invalid_response", Message: "The service returned no canonical conversation."}
	}
	return receipt.Conversation, nil
}

// Command-line retries can arrive in a new process. Observe the prior accepted
// input by identity before preparing a fresh revision; never manufacture a new
// request ID to recover an unknown outcome. Changed reuse is still a conflict.
func (c *Client) SubmitSessionInput(ctx context.Context, id, requestID string, input api.ConversationInboxInput) (*api.ConversationInboxEntry, error) {
	for after := int64(0); ; {
		page, err := c.ConversationInputs(ctx, id, after, 100)
		if err != nil {
			return nil, err
		}
		for _, entry := range page.Items {
			if entry.RequestID != requestID {
				continue
			}
			want, _ := json.Marshal(input)
			got, _ := json.Marshal(entry.Input)
			if string(want) != string(got) {
				return nil, &Error{Code: "request_conflict", Message: "This request ID already belongs to different conversation input."}
			}
			return &entry, nil
		}
		if !page.HasMore {
			break
		}
		after = page.NextCursor
	}
	// Receipt recovery above remains available after capability withdrawal. A
	// fresh submission must never let an older service ignore source/options
	// fields, including when this call is an explicit retry after a lost reply.
	if len(input.Attachments)+len(input.Images)+len(input.KnowledgeDocuments)+len(input.KnowledgeSelections) > 0 || input.UseKnowledge {
		if err := c.RequireCapability(ctx, "conversation-source-inputs-v1"); err != nil {
			return nil, &conversationInputRejected{err}
		}
	}
	if len(input.KnowledgeSelections) > 0 {
		if err := c.RequireCapability(ctx, "conversation-source-scopes-v1"); err != nil {
			return nil, &conversationInputRejected{err}
		}
	}
	if input.ResponseProfile != "" || input.MaxTokens != 0 || strings.TrimSpace(input.Content) == "" {
		if err := c.RequireCapability(ctx, "conversation-input-options-v1"); err != nil {
			return nil, &conversationInputRejected{err}
		}
	}
	conversation, err := c.CanonicalConversation(ctx, id)
	if err != nil {
		return nil, err
	}
	receipt, err := c.SubmitConversationInput(ctx, api.ConversationInputCommand{RequestID: requestID, TargetID: id, ExpectedRevision: conversation.Revision, Kind: "submit_input", Payload: input})
	if err != nil {
		var remote *Error
		// These input/source validation responses precede AcceptConversationInput
		// or its transaction commit. Other statuses (including identity conflicts)
		// and failures while inspecting an earlier receipt remain conservative.
		if errors.As(err, &remote) && (remote.Status == 400 || remote.Status == 413 || remote.Status == 422) {
			return nil, &conversationInputRejected{err}
		}
		return nil, err
	}
	if receipt.Input == nil {
		return nil, &Error{Code: "invalid_response", Message: "The service returned no accepted input."}
	}
	return receipt.Input, nil
}

type conversationInputRejected struct{ error }

func (e *conversationInputRejected) Unwrap() error { return e.error }

// ConversationInputRejected identifies this invocation's preflight failure
// before POST or explicit validation rejection. It cannot resolve uncertainty
// retained from an earlier invocation.
func ConversationInputRejected(err error) bool {
	var rejected *conversationInputRejected
	return errors.As(err, &rejected)
}

func (c *Client) WorkItem(ctx context.Context, id string) (*api.WorkItem, error) {
	var response struct {
		Work *api.WorkItem `json:"work"`
		Run  *struct {
			ID string `json:"run_id"`
		} `json:"run"`
	}
	if err := c.JSON(ctx, http.MethodGet, "/api/v2/work-items/"+url.PathEscape(id), nil, &response); err != nil {
		return nil, err
	}
	if response.Work == nil || response.Work.ID != id || response.Work.Revision < 1 || response.Work.RunID == "" || response.Run == nil || response.Run.ID != response.Work.RunID {
		return nil, &Error{Code: "invalid_response", Message: "The service returned incomplete or mismatched work state."}
	}
	return response.Work, nil
}

func PlainSessionCanMigrate(session *sessions.Session) bool {
	if session.Turn != nil && session.Turn.Active() {
		return false
	}
	for _, message := range session.Messages {
		if len(message.Images)+len(message.Attachments) > 0 {
			return false
		}
	}
	return true
}
