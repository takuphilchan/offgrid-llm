package serviceclient

import (
	"context"
	"net/http"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func (c *Client) UserPreferences(ctx context.Context) (*api.UserPreferences, error) {
	var preferences api.UserPreferences
	err := c.JSON(ctx, http.MethodGet, "/api/v2/user-preferences", nil, &preferences)
	return &preferences, err
}

func (c *Client) ConfigureUserPreferences(ctx context.Context, command api.UserPreferencesCommand) (*api.ConversationCommandReceipt, error) {
	var receipt api.ConversationCommandReceipt
	err := c.JSON(ctx, http.MethodPost, "/api/v2/user-preferences/commands", command, &receipt)
	return &receipt, err
}
