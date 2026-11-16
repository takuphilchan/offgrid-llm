package serviceclient

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/takuphilchan/offgrid-llm/internal/document"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
)

const compositionEndpoint = "/v1/document-analyses/synthesis/compositions"

type DocumentCompositionRequest struct {
	RequestKey string `json:"request_key"`
	AnalysisID string `json:"analysis_id"`
	Model      string `json:"model"`
	Language   string `json:"language"`
}

func validComposition(r rag.CompositionStatus) bool {
	if r.ID == "" || r.AnalysisID == "" || r.PlanID == "" || r.SourcePlanID == "" || r.Model.Validate() != nil || r.Attempt < 0 || r.CompletedSteps < 0 || r.TotalSteps < r.CompletedSteps || r.TotalSteps > 65536 || r.CreatedAt.IsZero() || r.UpdatedAt.IsZero() {
		return false
	}
	switch r.State {
	case "queued", "running", "interrupted", "cancelled", "failed":
		return true
	case "awaiting_review":
		return r.TotalSteps == r.CompletedSteps
	}
	return false
}
func (c *Client) CreateDocumentComposition(ctx context.Context, body DocumentCompositionRequest) (rag.CompositionStatus, error) {
	var r rag.CompositionStatus
	if body.RequestKey == "" || len(body.RequestKey) > 128 || body.AnalysisID == "" || len(body.AnalysisID) > 128 || body.Model == "" || body.Language == "" {
		return r, fmt.Errorf("invalid composition request")
	}
	err := c.JSON(ctx, http.MethodPost, compositionEndpoint, body, &r)
	if err == nil && (!validComposition(r) || r.AnalysisID != body.AnalysisID || r.Model.ModelID != body.Model || r.Model.Language != body.Language) {
		err = fmt.Errorf("unconfirmed composition; inspect history before retrying")
	}
	return r, err
}
func (c *Client) DocumentComposition(ctx context.Context, id string) (rag.CompositionStatus, error) {
	var r rag.CompositionStatus
	if id == "" || len(id) > 128 {
		return r, fmt.Errorf("invalid composition ID")
	}
	err := c.JSON(ctx, http.MethodGet, compositionEndpoint+"?id="+url.QueryEscape(id), nil, &r)
	if err == nil && (!validComposition(r) || r.ID != id) {
		err = fmt.Errorf("invalid composition response")
	}
	return r, err
}
func (c *Client) DocumentCompositionHistory(ctx context.Context, parent string, offset int) (rag.CompositionHistory, error) {
	var r rag.CompositionHistory
	if parent == "" || offset < 0 || offset > 256 {
		return r, fmt.Errorf("invalid composition history request")
	}
	err := c.JSON(ctx, http.MethodGet, compositionEndpoint+"?"+url.Values{"analysis_id": {parent}, "offset": {strconv.Itoa(offset)}}.Encode(), nil, &r)
	if err == nil {
		if r.AnalysisID != parent || r.Items == nil || len(r.Items) > 20 || (r.NextOffset != nil && *r.NextOffset != offset+20) {
			return r, fmt.Errorf("invalid composition history")
		}
		for _, item := range r.Items {
			if !validComposition(item) || item.AnalysisID != parent {
				return r, fmt.Errorf("invalid composition history identity")
			}
		}
	}
	return r, err
}
func (c *Client) ControlDocumentComposition(ctx context.Context, id, action string) error {
	if id == "" || len(id) > 128 || (action != "cancel" && action != "retry") {
		return fmt.Errorf("invalid composition control")
	}
	var r struct {
		Accepted bool `json:"accepted"`
	}
	err := c.JSON(ctx, http.MethodPost, compositionEndpoint+"/"+action, map[string]string{"id": id}, &r)
	if err == nil && !r.Accepted {
		return fmt.Errorf("unconfirmed composition action")
	}
	return err
}
func (c *Client) DocumentCompositionReport(ctx context.Context, id string) (rag.CompositionReportView, error) {
	var r rag.CompositionReportView
	job, err := c.DocumentComposition(ctx, id)
	if err != nil {
		return r, err
	}
	parent, err := c.DocumentSynthesis(ctx, job.AnalysisID)
	if err != nil {
		return r, err
	}
	err = c.JSON(ctx, http.MethodGet, compositionEndpoint+"/report?id="+url.QueryEscape(id), nil, &r)
	if err != nil {
		return r, err
	}
	if !validComposition(r.Composition) || r.Composition.ID != id || r.Composition.PlanID != job.PlanID || r.Composition.AnalysisID != parent.ID || r.Composition.SourcePlanID != parent.Plan.ID || r.Composition.State != "awaiting_review" || !r.SemanticReviewRequired || r.SourceUnits != len(parent.Plan.Units) || r.Sections == nil || len(r.Sections) > 4 || r.UnrepresentedBatches < 0 || r.UnrepresentedBatches > parent.CompletedBatches {
		return r, fmt.Errorf("invalid composition report")
	}
	for _, section := range r.Sections {
		if section.Text == "" || len(section.Text) > 2048 || section.Contributors < 1 || section.Contributors > 65536 || section.Verification != "source_identity_only" {
			return r, fmt.Errorf("invalid composed section")
		}
	}
	for _, gaps := range [][]rag.CompositionGapCount{r.Gaps, r.ExtractionGaps, r.CandidateGaps} {
		if gaps == nil || len(gaps) > 256 {
			return r, fmt.Errorf("invalid report gaps")
		}
		for _, gap := range gaps {
			if gap.Code == "" || gap.Count < 1 {
				return r, fmt.Errorf("invalid report gap")
			}
		}
	}
	return r, nil
}
func (c *Client) DocumentCompositionEvidence(ctx context.Context, id string, section, offset int) (rag.CompositionEvidencePage, error) {
	var r rag.CompositionEvidencePage
	if section < 0 || section > 3 || offset < 0 || offset > 65536 {
		return r, fmt.Errorf("invalid composition evidence page")
	}
	report, err := c.DocumentCompositionReport(ctx, id)
	if err != nil {
		return r, err
	}
	if section >= len(report.Sections) {
		return r, fmt.Errorf("missing composition section")
	}
	parent, err := c.DocumentSynthesis(ctx, report.Composition.AnalysisID)
	if err != nil {
		return r, err
	}
	err = c.JSON(ctx, http.MethodGet, compositionEndpoint+"/evidence?"+url.Values{"id": {id}, "section": {strconv.Itoa(section)}, "offset": {strconv.Itoa(offset)}}.Encode(), nil, &r)
	if err != nil {
		return r, err
	}
	if r.CompositionID != id || r.PlanID != report.Composition.PlanID || r.Section != section || r.Offset != offset || r.Total != report.Sections[section].Contributors || r.Items == nil || len(r.Items) > 8 {
		return r, fmt.Errorf("invalid composition evidence identity")
	}
	end := offset + len(r.Items)
	if end > r.Total || (end < r.Total && (end == offset || r.NextOffset == nil || *r.NextOffset != end)) || (end == r.Total && r.NextOffset != nil) {
		return r, fmt.Errorf("invalid composition evidence pagination")
	}
	for _, item := range r.Items {
		if !validCompositionEvidence(item, parent) {
			return r, fmt.Errorf("invalid composition source")
		}
	}
	return r, nil
}
func validCompositionEvidence(item rag.CompositionEvidence, parent rag.SynthesisRecord) bool {
	i, c := item.Input, item.Candidate
	raw, _ := json.Marshal(c)
	sum := sha256.Sum256(raw)
	if i.ID == "" || i.Batch < 0 || i.Batch >= parent.CompletedBatches || i.BatchID == "" || i.CandidateIndex < 0 || i.CandidateIndex > 31 || i.CandidateSHA256 != hex.EncodeToString(sum[:]) || i.ReviewRevision < 0 || i.ReviewRevision > 256 || (i.Decision != "keep" && i.Decision != "unreviewed") || (i.SectionChecked && i.Decision == "unreviewed") || c.Text == "" || c.Verification != "source_identity_only" || len(c.Evidence) < 1 {
		return false
	}
	for _, group := range [][]document.SynthesisCitedQuote{c.Evidence, c.Conditions, c.Exceptions} {
		if group == nil || len(group) > 8 {
			return false
		}
		for _, q := range group {
			if !validSynthesisQuote(q, parent) {
				return false
			}
		}
	}
	return true
}
func (c *Client) ExportDocumentComposition(ctx context.Context, id, format string) (*DocumentAnalysisExport, error) {
	return c.exportDocumentAnalysis(ctx, compositionEndpoint+"/export", id, format)
}
