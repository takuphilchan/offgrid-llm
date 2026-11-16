package serviceclient

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/takuphilchan/offgrid-llm/internal/document"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
)

func validSynthesisReview(r rag.SynthesisReview, id string, batch int, receipt *document.SynthesisBatchReceipt) bool {
	if r.AnalysisID != id || r.Batch != batch || receipt == nil || r.BatchID != receipt.BatchID || r.ResponseSHA256 != receipt.ResponseSHA256 || r.Revision < 0 || r.Revision > 256 || r.Decisions == nil || len(r.Decisions) != len(receipt.Result.Candidates) {
		return false
	}
	if r.Revision == 0 {
		if r.Author != "" || r.UpdatedAt != "" || r.SectionChecked {
			return false
		}
	} else {
		if _, err := time.Parse(time.RFC3339Nano, r.UpdatedAt); err != nil || r.Author == "" {
			return false
		}
	}
	for _, d := range r.Decisions {
		if d.Decision != "keep" && d.Decision != "exclude" && d.Decision != "unreviewed" || len(d.Note) > 2048 || !utf8.ValidString(d.Note) || r.SectionChecked && d.Decision == "unreviewed" || r.Revision == 0 && (d.Decision != "unreviewed" || d.Note != "") {
			return false
		}
	}
	return true
}

func (c *Client) DocumentSynthesisReview(ctx context.Context, id string, batch int, version ...int) (rag.SynthesisReview, error) {
	var review rag.SynthesisReview
	wanted := 0
	if len(version) > 0 {
		wanted = version[0]
	}
	if wanted < 0 || wanted > 256 {
		return review, fmt.Errorf("invalid review revision")
	}
	page, err := c.DocumentSynthesisResults(ctx, id, batch)
	if err != nil {
		return review, err
	}
	if len(page.Receipts) == 0 {
		return review, fmt.Errorf("missing synthesis section")
	}
	err = c.JSON(ctx, http.MethodGet, "/v1/document-analyses/synthesis/review?"+url.Values{"id": {id}, "batch": {strconv.Itoa(batch)}, "revision": {strconv.Itoa(wanted)}}.Encode(), nil, &review)
	if err == nil && (!validSynthesisReview(review, id, batch, page.Receipts[0]) || (wanted > 0 && review.Revision != wanted)) {
		err = fmt.Errorf("invalid synthesis review")
	}
	return review, err
}

func (c *Client) SaveDocumentSynthesisReview(ctx context.Context, req rag.SynthesisReviewRequest) (rag.SynthesisReview, error) {
	var review rag.SynthesisReview
	page, err := c.DocumentSynthesisResults(ctx, req.ID, req.Batch)
	if err != nil {
		return review, err
	}
	if len(page.Receipts) == 0 {
		return review, fmt.Errorf("missing synthesis section")
	}
	receipt := page.Receipts[0]
	if req.ExpectedRevision < 0 || req.ExpectedRevision >= 256 || req.BatchID != receipt.BatchID || req.ResponseSHA256 != receipt.ResponseSHA256 || len(req.Decisions) != len(receipt.Result.Candidates) {
		return review, fmt.Errorf("invalid review request")
	}
	err = c.JSON(ctx, http.MethodPost, "/v1/document-analyses/synthesis/review", req, &review)
	if err == nil && (!validSynthesisReview(review, req.ID, req.Batch, receipt) || review.Revision != req.ExpectedRevision+1 || review.SectionChecked != req.SectionChecked || !reflect.DeepEqual(review.Decisions, req.Decisions)) {
		err = fmt.Errorf("unconfirmed review; read saved review before retrying")
	}
	return review, err
}
