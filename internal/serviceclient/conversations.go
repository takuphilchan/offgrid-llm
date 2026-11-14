package serviceclient

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Canonical conversation transport follows the shared v2 OpenAPI envelope.
// The caller retains request IDs and revisions; the client never retries with
// a new identity, launches a local executor, or resumes work when reading.
func (c *Client) ResultPage(ctx context.Context, reference string, offset, limit int) (*api.ResultPage, error) {
	var page api.ResultPage
	err := c.JSON(ctx, http.MethodGet, "/api/v2/results/"+url.PathEscape(reference)+"?offset="+strconv.Itoa(offset)+"&limit="+strconv.Itoa(limit), nil, &page)
	return &page, err
}

func (c *Client) ConversationInputs(ctx context.Context, id string, after int64, limit int) (*api.ConversationInboxPage, error) {
	var page api.ConversationInboxPage
	path := "/api/v2/conversations/" + url.PathEscape(id) + "/inputs?after=" + strconv.FormatInt(after, 10) + "&limit=" + strconv.Itoa(limit)
	err := c.JSON(ctx, http.MethodGet, path, nil, &page)
	return &page, err
}

func (c *Client) ConversationInput(ctx context.Context, id, inputID string) (*api.ConversationInboxEntry, error) {
	var entry api.ConversationInboxEntry
	path := "/api/v2/conversations/" + url.PathEscape(id) + "/inputs/" + url.PathEscape(inputID)
	err := c.JSON(ctx, http.MethodGet, path, nil, &entry)
	return &entry, err
}

func (c *Client) SubmitConversationInput(ctx context.Context, command api.ConversationInputCommand) (*api.ConversationCommandReceipt, error) {
	var receipt api.ConversationCommandReceipt
	err := c.JSON(ctx, http.MethodPost, "/api/v2/conversations/"+url.PathEscape(command.TargetID)+"/inputs", command, &receipt)
	return &receipt, err
}

func (c *Client) ControlConversationInput(ctx context.Context, conversation string, command api.ConversationInputControlCommand) (*api.ConversationCommandReceipt, error) {
	var receipt api.ConversationCommandReceipt
	err := c.JSON(ctx, http.MethodPost, "/api/v2/conversations/"+url.PathEscape(conversation)+"/inputs/"+url.PathEscape(command.TargetID)+"/commands", command, &receipt)
	return &receipt, err
}

func (c *Client) ConversationCommand(ctx context.Context, command api.ConversationCommand) (*api.ConversationCommandReceipt, error) {
	path := "/api/v2/conversations"
	if command.TargetID != "" {
		path += "/" + url.PathEscape(command.TargetID) + "/commands"
	}
	var receipt api.ConversationCommandReceipt
	err := c.JSON(ctx, http.MethodPost, path, command, &receipt)
	return &receipt, err
}

func (c *Client) WorkCommand(ctx context.Context, command api.WorkCommand) (*api.ConversationCommandReceipt, error) {
	var receipt api.ConversationCommandReceipt
	err := c.JSON(ctx, http.MethodPost, "/api/v2/work-items/"+url.PathEscape(command.TargetID)+"/commands", command, &receipt)
	return &receipt, err
}

func (c *Client) CanonicalConversation(ctx context.Context, id string) (*api.Conversation, error) {
	var value api.Conversation
	err := c.JSON(ctx, http.MethodGet, "/api/v2/conversations/"+url.PathEscape(id), nil, &value)
	return &value, err
}

func (c *Client) ConversationConfiguration(ctx context.Context, id string) (*api.EffectiveConversationPreferences, error) {
	var value api.EffectiveConversationPreferences
	err := c.JSON(ctx, http.MethodGet, "/api/v2/conversations/"+url.PathEscape(id)+"/configuration", nil, &value)
	return &value, err
}

func (c *Client) ConversationEvents(ctx context.Context, id string, after int64, limit int) (*api.ControlEventPage, error) {
	var page api.ControlEventPage
	path := "/api/v2/conversations/" + url.PathEscape(id) + "/events?after=" + strconv.FormatInt(after, 10) + "&limit=" + strconv.Itoa(limit)
	err := c.JSON(ctx, http.MethodGet, path, nil, &page)
	return &page, err
}
