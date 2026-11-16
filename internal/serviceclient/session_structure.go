package serviceclient

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

func (c *Client) SessionAttachmentStructure(ctx context.Context, name, id, revision, unit string, offset, limit, relationshipOffset int, navigationOffset ...int) (document.StructurePage, error) {
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
	if name == "" || !validAttachmentID(id) || revision == "" || len(revision) > 128 || len(unit) > 128 || offset < 0 || offset > 200000 || limit < 1 || limit > 200 || relationshipOffset < 0 || relationshipOffset > 400000 {
		return page, fmt.Errorf("invalid attachment structure request")
	}
	q := url.Values{"revision": {revision}, "unit": {unit}, "offset": {strconv.Itoa(offset)}, "limit": {strconv.Itoa(limit)}, "relationship_offset": {strconv.Itoa(relationshipOffset)}, "navigation_offset": {strconv.Itoa(navigation)}}
	if err := c.JSON(ctx, http.MethodGet, sessionFilesPath(name)+"/"+id+"/structure?"+q.Encode(), nil, &page); err != nil {
		return page, err
	}
	if page.NavigationOffset != navigation || navigation > 0 && page.Navigation == nil || page.DocumentID != id || page.Revision == "" || page.Offset != offset || page.RelationshipOffset != relationshipOffset || len(page.Blocks) > limit || page.CanCorrect {
		return document.StructurePage{}, &Error{Code: "invalid_response", Message: "Private structure response does not match the requested source."}
	}
	for _, b := range page.Blocks {
		if b.Editable {
			return document.StructurePage{}, &Error{Code: "invalid_response", Message: "Private structure response unexpectedly allows corrections."}
		}
	}
	return page, nil
}
