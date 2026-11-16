package serviceclient

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strconv"

	"github.com/takuphilchan/offgrid-llm/internal/document"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
)

type DocumentSynthesisRequest struct {
	RequestKey string               `json:"request_key"`
	Workflow   string               `json:"workflow"`
	Sources    []document.SourceRef `json:"sources"`
	Model      string               `json:"model"`
	Language   string               `json:"language"`
}

func validSynthesis(r *rag.SynthesisRecord) bool {
	if r == nil || r.ID == "" || r.Plan == nil || r.Plan.Version != document.AnalysisVersion || (r.Plan.Workflow != "summary" && r.Plan.Workflow != "requirements") || len(r.Plan.Sources) < 1 || len(r.Plan.Sources) > 16 || len(r.Plan.Units) == 0 || r.Model.Validate() != nil || r.CompletedBatches < 0 || r.Attempt < 0 || r.UnitCursor < 0 || r.UnitCursor > len(r.Plan.Units) || r.CompletedUnits != r.UnitCursor || r.NextByte < 0 {
		return false
	}
	for i, u := range r.Plan.Units {
		if u.Ordinal != i || u.Source < 0 || u.Source >= len(r.Plan.Sources) || u.UnitID == "" || u.Span.Start < 0 || u.Span.End < u.Span.Start {
			return false
		}
	}
	for _, s := range r.Plan.Sources {
		if s.DocumentID == "" || s.Revision == "" {
			return false
		}
	}
	if r.UnitCursor < len(r.Plan.Units) {
		u := r.Plan.Units[r.UnitCursor]
		if r.NextByte < u.Span.Start || r.NextByte > u.Span.End {
			return false
		}
	} else if r.NextByte != 0 {
		return false
	}
	switch r.State {
	case "queued", "running", "interrupted", "cancelled", "failed":
		return r.UnitCursor < len(r.Plan.Units)
	case "awaiting_review":
		return r.UnitCursor == len(r.Plan.Units)
	default:
		return false
	}
}
func (c *Client) CreateDocumentSynthesis(ctx context.Context, request DocumentSynthesisRequest) (rag.SynthesisRecord, error) {
	var r rag.SynthesisRecord
	if request.RequestKey == "" || (request.Workflow != "summary" && request.Workflow != "requirements") || request.Model == "" || request.Language == "" || len(request.Sources) < 1 || len(request.Sources) > 16 {
		return r, fmt.Errorf("invalid synthesis request")
	}
	err := c.JSON(ctx, http.MethodPost, "/v1/document-analyses/synthesis", request, &r)
	if err == nil && (!validSynthesis(&r) || r.Plan.Workflow != request.Workflow || !reflect.DeepEqual(r.Plan.Sources, request.Sources) || r.Model.ModelID != request.Model || r.Model.Language != request.Language) {
		err = fmt.Errorf("unconfirmed document synthesis; inspect history before retrying")
	}
	return r, err
}
func (c *Client) DocumentSynthesis(ctx context.Context, id string) (rag.SynthesisRecord, error) {
	var r rag.SynthesisRecord
	if id == "" {
		return r, fmt.Errorf("analysis ID is required")
	}
	err := c.JSON(ctx, http.MethodGet, "/v1/document-analyses/synthesis?id="+url.QueryEscape(id), nil, &r)
	if err == nil && (!validSynthesis(&r) || r.ID != id) {
		err = fmt.Errorf("invalid synthesis response")
	}
	return r, err
}
func (c *Client) DocumentSynthesisHistory(ctx context.Context, offset int) (rag.AnalysisHistory, error) {
	var p rag.AnalysisHistory
	if offset < 0 || offset > 256 {
		return p, fmt.Errorf("invalid synthesis history offset")
	}
	err := c.JSON(ctx, http.MethodGet, "/v1/document-analyses/synthesis?offset="+strconv.Itoa(offset), nil, &p)
	if err == nil && (p.Analyses == nil || len(p.Analyses) > 20 || (p.NextOffset != nil && *p.NextOffset != offset+20)) {
		err = fmt.Errorf("invalid synthesis history")
	}
	return p, err
}
func (c *Client) ControlDocumentSynthesis(ctx context.Context, id, action string) error {
	if action == "cancel" {
		return c.ControlDocumentAnalysis(ctx, id, action)
	}
	if action != "retry" || id == "" {
		return fmt.Errorf("invalid synthesis action")
	}
	var response struct {
		Accepted bool `json:"accepted"`
	}
	err := c.JSON(ctx, http.MethodPost, "/v1/document-analyses/synthesis/retry", map[string]string{"id": id}, &response)
	if err == nil && !response.Accepted {
		err = fmt.Errorf("unconfirmed synthesis retry")
	}
	return err
}
func (c *Client) DocumentSynthesisResults(ctx context.Context, id string, offset int) (rag.SynthesisResultPage, error) {
	var page rag.SynthesisResultPage
	if offset < 0 || offset > 100000 {
		return page, fmt.Errorf("invalid synthesis offset")
	}
	job, err := c.DocumentSynthesis(ctx, id)
	if err != nil {
		return page, err
	}
	err = c.JSON(ctx, http.MethodGet, "/v1/document-analyses/synthesis/results?"+url.Values{"id": {id}, "offset": {strconv.Itoa(offset)}}.Encode(), nil, &page)
	if err != nil {
		return page, err
	}
	if page.AnalysisID != id || page.Offset != offset || page.TotalBatches > 100000 || page.TotalBatches < 0 || page.Receipts == nil || len(page.Receipts) > 2 {
		return page, fmt.Errorf("invalid synthesis result page")
	}
	end := offset + len(page.Receipts)
	if end > page.TotalBatches || (end < page.TotalBatches && (end == offset || page.NextOffset == nil || *page.NextOffset != end)) || (end == page.TotalBatches && page.NextOffset != nil) {
		return page, fmt.Errorf("invalid synthesis pagination")
	}
	identity, _ := job.Model.ID()
	for _, r := range page.Receipts {
		if r == nil || (r.Validation != "" && r.Validation != "source-values-v2") || r.BatchID == "" || r.ModelIdentity != identity || r.UnitOrdinal < 0 || r.UnitOrdinal >= len(job.Plan.Units) || r.Result == nil || r.Result.BatchID != r.BatchID || len(r.Result.Candidates) > 32 || r.Method == "model" && !r.Result.SemanticReviewRequired {
			return page, fmt.Errorf("invalid synthesis receipt")
		}
		unit := job.Plan.Units[r.UnitOrdinal]
		if r.Primary.Start < unit.Span.Start || r.Primary.End > unit.Span.End || r.Primary.End < r.Primary.Start || r.EndOfUnit != (r.Primary.End == unit.Span.End) || !validSynthesisDigest(r.ResponseSHA256) {
			return page, fmt.Errorf("invalid synthesis receipt range")
		}
		switch r.Method {
		case "model":
			if !document.MeasuredSynthesisFit(r.PromptTokens, job.Model.ContextTokens, job.Model.ResponseTokens) {
				return page, fmt.Errorf("invalid synthesis prompt measurement")
			}
		case "empty_source":
			if len(r.Result.Candidates) != 0 || r.PromptTokens != 0 || r.Result.NoFindingsReported {
				return page, fmt.Errorf("invalid empty source receipt")
			}
		default:
			return page, fmt.Errorf("unknown synthesis method")
		}
		for _, item := range r.Result.Candidates {
			if item.Text == "" || item.Verification != "source_identity_only" || len(item.Evidence) == 0 {
				return page, fmt.Errorf("invalid synthesis candidate")
			}
			for _, group := range [][]document.SynthesisCitedQuote{item.Evidence, item.Conditions, item.Exceptions} {
				if len(group) > 8 {
					return page, fmt.Errorf("oversized synthesis evidence")
				}
				for _, quote := range group {
					if !validSynthesisQuote(quote, job) {
						return page, fmt.Errorf("invalid synthesis quote")
					}
				}
			}
		}
	}
	return page, nil
}

func validSynthesisDigest(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 32
}
func validSynthesisQuote(q document.SynthesisCitedQuote, r rag.SynthesisRecord) bool {
	c := q.Citation
	if c.Source < 0 || c.Source >= len(r.Plan.Sources) || c.Revision != r.Plan.Sources[c.Source].Revision || len(q.Text) == 0 || len(q.Text) > 8192 || len(q.Text) != c.Span.End-c.Span.Start {
		return false
	}
	hash := sha256.Sum256([]byte(q.Text))
	if hex.EncodeToString(hash[:]) != c.TextSHA256 {
		return false
	}
	for _, u := range r.Plan.Units {
		if u.Source == c.Source && u.UnitID == c.UnitID && c.Span.Start >= u.Span.Start && c.Span.End <= u.Span.End {
			return true
		}
	}
	return false
}
