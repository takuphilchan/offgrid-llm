package serviceclient

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"

	"github.com/takuphilchan/offgrid-llm/internal/rag"
)

func collectionReceipt(p *rag.CollectionPolicy, id string) error {
	if p == nil || p.ID == "" || id != "" && p.ID != id || p.Version != 1 || p.Revision < 1 || p.Name == "" || p.Readers == nil || p.Editors == nil || len(p.Readers) > 128 || len(p.Editors) > 128 || p.ID != rag.WorkspaceCollection && p.Owner == "" {
		return &Error{Code: "invalid_response", Message: "Collection state was not confirmed. Inspect the policy before retrying."}
	}
	return nil
}

func (c *Client) Collections(ctx context.Context) ([]*rag.CollectionPolicy, error) {
	var result struct {
		Collections []*rag.CollectionPolicy `json:"collections"`
	}
	if err := c.JSON(ctx, http.MethodGet, "/v1/collections", nil, &result); err != nil {
		return nil, err
	}
	if result.Collections == nil || len(result.Collections) > 1024 {
		return nil, &Error{Code: "invalid_response", Message: "Collection list is incomplete."}
	}
	seen := map[string]bool{}
	for _, p := range result.Collections {
		if err := collectionReceipt(p, ""); err != nil {
			return nil, err
		}
		if seen[p.ID] {
			return nil, &Error{Code: "invalid_response", Message: "Duplicate collection identity."}
		}
		seen[p.ID] = true
	}
	return result.Collections, nil
}

func (c *Client) CollectionAudit(ctx context.Context, id string, after int64) ([]rag.CollectionAudit, error) {
	if id == "" || after < 0 {
		return nil, fmt.Errorf("provide a collection identity and nonnegative audit cursor")
	}
	var result struct {
		Events []rag.CollectionAudit `json:"events"`
	}
	query := url.Values{"id": {id}, "view": {"audit"}, "after": {strconv.FormatInt(after, 10)}}
	if err := c.JSON(ctx, http.MethodGet, "/v1/collections?"+query.Encode(), nil, &result); err != nil {
		return nil, err
	}
	if result.Events == nil || len(result.Events) > 100 {
		return nil, &Error{Code: "invalid_response", Message: "Collection audit page is incomplete."}
	}
	for _, event := range result.Events {
		if event.Sequence <= after || event.Actor == "" || event.Action == "" || event.Subject == "" || event.Revision < 0 || event.CreatedAt.IsZero() {
			return nil, &Error{Code: "invalid_response", Message: "Collection audit sequence is invalid."}
		}
		after = event.Sequence
	}
	return result.Events, nil
}

func (c *Client) Collection(ctx context.Context, id string) (*rag.CollectionPolicy, error) {
	if id == "" {
		return nil, fmt.Errorf("collection identity is required")
	}
	var p rag.CollectionPolicy
	if err := c.JSON(ctx, http.MethodGet, "/v1/collections?id="+url.QueryEscape(id), nil, &p); err != nil {
		return nil, err
	}
	return &p, collectionReceipt(&p, id)
}

func (c *Client) CreateCollection(ctx context.Context, name string) (*rag.CollectionPolicy, error) {
	var p rag.CollectionPolicy
	if err := c.JSON(ctx, http.MethodPost, "/v1/collections", map[string]string{"name": name}, &p); err != nil {
		return nil, err
	}
	if err := collectionReceipt(&p, ""); err != nil {
		return nil, err
	}
	if p.ID == rag.WorkspaceCollection || p.Revision != 1 || p.Name != name || len(p.Readers) != 0 || len(p.Editors) != 0 {
		return nil, &Error{Code: "invalid_response", Message: "Private collection creation was not confirmed."}
	}
	return &p, nil
}

func (c *Client) UpdateCollection(ctx context.Context, id string, request rag.CollectionUpdate) (*rag.CollectionPolicy, error) {
	if id == "" || request.ExpectedRevision < 1 || request.Readers == nil || request.Editors == nil {
		return nil, fmt.Errorf("provide explicit membership and current revision")
	}
	var p rag.CollectionPolicy
	if err := c.JSON(ctx, http.MethodPut, "/v1/collections?id="+url.QueryEscape(id), request, &p); err != nil {
		return nil, err
	}
	if err := collectionReceipt(&p, id); err != nil {
		return nil, err
	}
	readers, editors := slices.Clone(request.Readers), slices.Clone(request.Editors)
	slices.Sort(readers)
	slices.Sort(editors)
	if p.Revision != request.ExpectedRevision+1 || p.Name != request.Name || !slices.Equal(p.Readers, readers) || !slices.Equal(p.Editors, editors) {
		return nil, &Error{Code: "invalid_response", Message: "Sharing change was not confirmed. Inspect the current audience before retrying."}
	}
	return &p, nil
}

func (c *Client) DocumentCollection(ctx context.Context, id string) (*rag.DocumentCollection, error) {
	var result rag.DocumentCollection
	if err := c.JSON(ctx, http.MethodGet, "/v1/documents/collection?id="+url.QueryEscape(id), nil, &result); err != nil {
		return nil, err
	}
	if result.DocumentID != id || result.Revision < 1 {
		return nil, &Error{Code: "invalid_response", Message: "Document collection was not confirmed."}
	}
	return &result, collectionReceipt(result.Collection, "")
}

func (c *Client) MoveDocument(ctx context.Context, request rag.CollectionMove) (*rag.DocumentCollection, error) {
	if request.DocumentID == "" || request.DestinationID == "" || request.ExpectedRevision < 1 || request.ExpectedSourceRevision < 1 || request.ExpectedDestinationRevision < 1 {
		return nil, fmt.Errorf("review both collection audiences before moving a document")
	}
	var result rag.DocumentCollection
	if err := c.JSON(ctx, http.MethodPut, "/v1/documents/collection", request, &result); err != nil {
		return nil, err
	}
	if err := collectionReceipt(result.Collection, request.DestinationID); err != nil {
		return nil, err
	}
	if result.DocumentID != request.DocumentID || result.Revision < request.ExpectedRevision || result.Revision > request.ExpectedRevision+1 || result.Collection.Revision != request.ExpectedDestinationRevision {
		return nil, &Error{Code: "invalid_response", Message: "Document movement was not confirmed. Inspect its audience before retrying."}
	}
	return &result, nil
}
