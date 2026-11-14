package serviceclient

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func (c *Client) ProjectCommand(ctx context.Context, command api.ProjectCommand) (*api.ConversationCommandReceipt, error) {
	path := "/api/v2/projects"
	if command.TargetID != "" {
		path += "/" + url.PathEscape(command.TargetID) + "/commands"
	}
	var receipt api.ConversationCommandReceipt
	err := c.JSON(ctx, http.MethodPost, path, command, &receipt)
	return &receipt, err
}

func (c *Client) Project(ctx context.Context, id string, revision int64) (*api.Project, error) {
	path := "/api/v2/projects/" + url.PathEscape(id)
	if revision != 0 {
		path += "?revision=" + strconv.FormatInt(revision, 10)
	}
	var project api.Project
	err := c.JSON(ctx, http.MethodGet, path, nil, &project)
	return &project, err
}
