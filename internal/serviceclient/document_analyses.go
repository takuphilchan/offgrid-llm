package serviceclient

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"unicode/utf8"

	"github.com/takuphilchan/offgrid-llm/internal/document"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
)

func validAnalysis(r *rag.AnalysisRecord) bool {
	if r == nil || r.ID == "" || r.Plan == nil || r.Plan.ID == "" || r.Plan.Version != document.AnalysisVersion || r.Plan.Workflow != "comparison" || len(r.Plan.Sources) != 2 || len(r.Plan.Units) < 2 || len(r.Pairs) == 0 || r.CompletedSteps < 0 || r.CompletedSteps > len(r.Pairs) || r.CompletedUnits < 0 || r.CompletedUnits > len(r.Plan.Units) {
		return false
	}
	for _, source := range r.Plan.Sources {
		if source.DocumentID == "" || source.Revision == "" {
			return false
		}
	}
	switch r.State {
	case "queued", "running", "interrupted", "cancelled", "failed":
		return true
	case "complete", "incomplete":
		return r.CompletedSteps == len(r.Pairs) && r.CompletedUnits == len(r.Plan.Units) && (r.State != "complete" || len(r.Plan.Gaps) == 0 && !r.ReviewRequired)
	default:
		return false
	}
}

func (c *Client) CreateDocumentComparison(ctx context.Context, key string, sources []document.SourceRef) (rag.AnalysisRecord, error) {
	var result rag.AnalysisRecord
	if key == "" || len(key) > 128 || len(sources) != 2 {
		return result, fmt.Errorf("invalid comparison request")
	}
	err := c.JSON(ctx, http.MethodPost, "/v1/document-analyses", map[string]any{"request_key": key, "workflow": "comparison", "sources": sources}, &result)
	if err == nil && (!validAnalysis(&result) || result.Plan.Sources[0] != sources[0] || result.Plan.Sources[1] != sources[1]) {
		err = fmt.Errorf("unconfirmed document comparison; inspect analyses before retrying")
	}
	return result, err
}

func (c *Client) DocumentAnalysis(ctx context.Context, id string) (rag.AnalysisRecord, error) {
	var result rag.AnalysisRecord
	if id == "" {
		return result, fmt.Errorf("analysis ID is required")
	}
	err := c.JSON(ctx, http.MethodGet, "/v1/document-analyses?id="+url.QueryEscape(id), nil, &result)
	if err == nil && (!validAnalysis(&result) || result.ID != id) {
		err = fmt.Errorf("invalid document analysis response")
	}
	return result, err
}

func (c *Client) ControlDocumentAnalysis(ctx context.Context, id, action string) error {
	if id == "" || (action != "cancel" && action != "retry") {
		return fmt.Errorf("invalid analysis action")
	}
	var result struct {
		Accepted bool `json:"accepted"`
	}
	err := c.JSON(ctx, http.MethodPost, "/v1/document-analyses/"+action, map[string]string{"id": id}, &result)
	if err == nil && !result.Accepted {
		err = fmt.Errorf("analysis action was not acknowledged; inspect its status")
	}
	return err
}

func (c *Client) DocumentAnalysisResults(ctx context.Context, id string, step, offset, limit int) (rag.AnalysisResultPage, error) {
	var page rag.AnalysisResultPage
	if id == "" || step < 0 || offset < 0 || offset > 400000 || limit < 1 || limit > 100 {
		return page, fmt.Errorf("invalid analysis pagination")
	}
	q := url.Values{"id": {id}, "step": {strconv.Itoa(step)}, "offset": {strconv.Itoa(offset)}, "limit": {strconv.Itoa(limit)}}
	err := c.JSON(ctx, http.MethodGet, "/v1/document-analyses/results?"+q.Encode(), nil, &page)
	if err == nil && (page.AnalysisID != id || page.Step != step || page.Offset != offset || page.Result == nil || page.TotalChanges < offset || len(page.Result.Changes) > limit || offset+len(page.Result.Changes) > page.TotalChanges) {
		err = fmt.Errorf("invalid analysis result page")
	}
	if err == nil {
		end := offset + len(page.Result.Changes)
		if (end < page.TotalChanges && (page.NextOffset == nil || *page.NextOffset != end)) || (end == page.TotalChanges && page.NextOffset != nil) {
			err = fmt.Errorf("invalid analysis result cursor")
		}
	}
	if err == nil && !validAnalysisPreviews(page) {
		err = fmt.Errorf("invalid source preview")
	}
	return page, err
}

func (c *Client) DocumentAnalysisSource(ctx context.Context, id string, step, change int, side string) (rag.AnalysisSourceResult, error) {
	var result rag.AnalysisSourceResult
	if id == "" || step < 0 || change < 0 || (side != "before" && side != "after") {
		return result, fmt.Errorf("invalid analysis source request")
	}
	q := url.Values{"id": {id}, "step": {strconv.Itoa(step)}, "change": {strconv.Itoa(change)}, "side": {side}}
	err := c.JSON(ctx, http.MethodGet, "/v1/document-analyses/source?"+q.Encode(), nil, &result)
	wantSource := 0
	if side == "after" {
		wantSource = 1
	}
	if err == nil && (result.AnalysisID != id || result.DocumentID == "" || result.Citation.Source != wantSource || result.Citation.Revision == "" || len(result.Text) > 256<<10) {
		err = fmt.Errorf("invalid analysis source response")
	}
	if err == nil {
		span := result.Citation.Span
		if span.Start < 0 || span.End < span.Start || !utf8.ValidString(result.Text) || len(result.Text) > span.End-span.Start || (!result.Truncated && len(result.Text) != span.End-span.Start) || (result.Truncated && len(result.Text) == span.End-span.Start) {
			err = fmt.Errorf("invalid analysis source span")
		}
		if !result.Truncated {
			hash := sha256.Sum256([]byte(result.Text))
			if hex.EncodeToString(hash[:]) != result.Citation.TextSHA256 {
				err = fmt.Errorf("invalid analysis source digest")
			}
		}
	}
	return result, err
}
