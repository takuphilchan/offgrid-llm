package serviceclient

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func (c *Client) WorkChanges(ctx context.Context, id, after string, limit int) (*api.WorkChangeList, error) {
	var page api.WorkChangeList
	query := url.Values{"after": {after}, "limit": {strconv.Itoa(limit)}}
	err := c.JSON(ctx, http.MethodGet, "/api/v2/work-items/"+url.PathEscape(id)+"/changes?"+query.Encode(), nil, &page)
	return &page, err
}

func (c *Client) WorkChange(ctx context.Context, id, change, view, hash string, offset int64, limit int, artifact ...string) (*api.WorkChangeDetail, error) {
	var page api.WorkChangeDetail
	query := url.Values{"view": {view}, "hash": {hash}, "offset": {strconv.FormatInt(offset, 10)}, "limit": {strconv.Itoa(limit)}}
	if len(artifact) > 0 {
		query.Set("artifact", artifact[0])
	}
	err := c.JSON(ctx, http.MethodGet, "/api/v2/work-items/"+url.PathEscape(id)+"/changes/"+url.PathEscape(change)+"?"+query.Encode(), nil, &page)
	return &page, err
}
