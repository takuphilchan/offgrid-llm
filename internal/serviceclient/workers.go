package serviceclient

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func (c *Client) Worker(ctx context.Context, id string) (*api.WorkerSession, error) {
	var worker api.WorkerSession
	err := c.JSON(ctx, http.MethodGet, "/api/v2/workers/"+url.PathEscape(id), nil, &worker)
	return &worker, err
}

func (c *Client) WorkerCommand(ctx context.Context, command api.WorkerCommand) (*api.ConversationCommandReceipt, error) {
	var receipt api.ConversationCommandReceipt
	err := c.JSON(ctx, http.MethodPost, "/api/v2/workers/"+url.PathEscape(command.TargetID)+"/commands", command, &receipt)
	return &receipt, err
}

func (c *Client) WorkerMessages(ctx context.Context, id string, after int64, limit int) (*api.WorkerMessagePage, error) {
	var page api.WorkerMessagePage
	path := "/api/v2/workers/" + url.PathEscape(id) + "/messages?after=" + strconv.FormatInt(after, 10) + "&limit=" + strconv.Itoa(limit)
	err := c.JSON(ctx, http.MethodGet, path, nil, &page)
	return &page, err
}
