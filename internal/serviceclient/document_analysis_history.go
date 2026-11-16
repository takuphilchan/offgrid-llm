package serviceclient

import (
	"context"
	"fmt"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"net/http"
)

func (c *Client) DocumentAnalysisHistory(ctx context.Context, offset, limit int) (rag.AnalysisHistory, error) {
	var page rag.AnalysisHistory
	if offset < 0 || offset > 256 || limit < 1 || limit > 20 {
		return page, fmt.Errorf("invalid analysis history pagination")
	}
	err := c.JSON(ctx, http.MethodGet, fmt.Sprintf("/v1/document-analyses?offset=%d&limit=%d", offset, limit), nil, &page)
	if err != nil {
		return page, err
	}
	if page.Analyses == nil || len(page.Analyses) > limit || (page.NextOffset != nil && (*page.NextOffset != offset+limit || *page.NextOffset > 256)) {
		return page, fmt.Errorf("invalid analysis history response")
	}
	seen := map[string]bool{}
	for _, item := range page.Analyses {
		validState := false
		for _, state := range []string{"queued", "running", "interrupted", "cancelled", "failed", "complete", "incomplete"} {
			validState = validState || item.State == state
		}
		if item.ID == "" || seen[item.ID] || !validState || len(item.Documents) != 2 || item.Documents[0] == "" || item.Documents[1] == "" || item.CreatedAt.IsZero() {
			return page, fmt.Errorf("invalid analysis history item")
		}
		seen[item.ID] = true
	}
	return page, nil
}
