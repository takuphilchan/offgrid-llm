package serviceclient

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/takuphilchan/offgrid-llm/internal/document"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
)

func (c *Client) DocumentStructure(ctx context.Context, id, revision, unit string, offset, limit, relationshipOffset int, navigationOffset ...int) (document.StructurePage, error) {
	var page document.StructurePage
	navigation := 0
	if len(navigationOffset) > 1 {
		return page, fmt.Errorf("invalid navigation request")
	}
	if len(navigationOffset) == 1 {
		navigation = navigationOffset[0]
	}
	if navigation < 0 || navigation > 200000 {
		return page, fmt.Errorf("invalid navigation request")
	}
	if id == "" || offset < 0 || limit < 1 || limit > 200 || relationshipOffset < 0 {
		return page, fmt.Errorf("invalid structure request")
	}
	query := url.Values{"id": {id}, "revision": {revision}, "unit": {unit}, "offset": {strconv.Itoa(offset)}, "limit": {strconv.Itoa(limit)}, "relationship_offset": {strconv.Itoa(relationshipOffset)}, "navigation_offset": {strconv.Itoa(navigation)}}
	if err := c.JSON(ctx, http.MethodGet, "/v1/documents/structure?"+query.Encode(), nil, &page); err != nil {
		return page, err
	}
	if page.NavigationOffset != navigation || navigation > 0 && page.Navigation == nil || page.DocumentID != id || page.Revision == "" || revision != "" && page.Revision != revision || page.Offset != offset || page.RelationshipOffset != relationshipOffset || len(page.Blocks) > limit {
		return document.StructurePage{}, &Error{Code: "invalid_response", Message: "Document structure response does not match the requested revision."}
	}
	return page, nil
}

func (c *Client) CorrectDocument(ctx context.Context, request document.CorrectionRequest) (*rag.Document, error) {
	if request.DocumentID == "" || request.ExpectedRevision == "" || len(request.Edits) == 0 || len(request.Edits) > 256 {
		return nil, fmt.Errorf("invalid correction request")
	}
	var result struct {
		Success  bool          `json:"success"`
		Document *rag.Document `json:"document"`
	}
	if err := c.JSON(ctx, http.MethodPost, "/v1/documents/corrections", request, &result); err != nil {
		return nil, err
	}
	if !result.Success || result.Document == nil || result.Document.ID != request.DocumentID || result.Document.ExtractionRevision == "" || result.Document.ExtractionRevision == request.ExpectedRevision {
		return nil, &Error{Code: "invalid_response", Message: "Correction publication was not confirmed. Inspect the active revision before retrying."}
	}
	return result.Document, nil
}
