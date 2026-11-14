package serviceclient

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"

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
	conversation, err := c.CanonicalConversation(ctx, id)
	if err != nil {
		return nil, err
	}
	receipt, err := c.SubmitConversationInput(ctx, api.ConversationInputCommand{RequestID: requestID, TargetID: id, ExpectedRevision: conversation.Revision, Kind: "submit_input", Payload: input})
	if err != nil {
		return nil, err
	}
	if receipt.Input == nil {
		return nil, &Error{Code: "invalid_response", Message: "The service returned no accepted input."}
	}
	return receipt.Input, nil
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
