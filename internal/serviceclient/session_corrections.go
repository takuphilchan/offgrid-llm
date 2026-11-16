package serviceclient

import (
	"context"
	"fmt"
	"net/http"

	"github.com/takuphilchan/offgrid-llm/internal/document"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
)

type AttachmentCorrectionRequest struct {
	ExpectedRevision   string                `json:"expected_revision"`
	ExtractionRevision string                `json:"extraction_revision"`
	Edits              []document.Correction `json:"edits"`
}

func (c *Client) SessionAttachmentRevisions(ctx context.Context, name, id string) (sessions.AttachmentRevisionHistory, error) {
	var history sessions.AttachmentRevisionHistory
	if name == "" || !validAttachmentID(id) {
		return history, fmt.Errorf("invalid attachment identity")
	}
	if err := c.JSON(ctx, http.MethodGet, sessionFilesPath(name)+"/"+id+"/revisions", nil, &history); err != nil {
		return history, err
	}
	if history.ID != id || len(history.Revisions) < 1 || len(history.Revisions) > 65 || history.CurrentRevision == "" || history.Revisions[0].Revision != history.CurrentRevision {
		return sessions.AttachmentRevisionHistory{}, &Error{Code: "invalid_response", Message: "Attachment history does not match the requested source."}
	}
	seen := map[string]bool{}
	for _, r := range history.Revisions {
		if len(r.Revision) != 64 || r.ExtractionRevision == "" || seen[r.Revision] {
			return sessions.AttachmentRevisionHistory{}, &Error{Code: "invalid_response", Message: "Invalid attachment revision history."}
		}
		seen[r.Revision] = true
	}
	return history, nil
}

func (c *Client) CorrectSessionAttachment(ctx context.Context, name, id string, request AttachmentCorrectionRequest) (sessions.Attachment, error) {
	var result struct {
		Attachment sessions.Attachment `json:"attachment"`
	}
	if name == "" || !validAttachmentID(id) || len(request.ExpectedRevision) != 64 || request.ExtractionRevision == "" || len(request.Edits) < 1 || len(request.Edits) > 256 {
		return result.Attachment, fmt.Errorf("invalid attachment correction")
	}
	if err := c.RequireCapability(ctx, "session-document-corrections-v1"); err != nil {
		return result.Attachment, err
	}
	if err := c.JSON(ctx, http.MethodPost, sessionFilesPath(name)+"/"+id+"/corrections", request, &result); err != nil {
		return result.Attachment, err
	}
	a := result.Attachment
	if a.ID != id || a.Status != "ready" || a.ParentRevision != request.ExpectedRevision || len(a.Revision) != 64 || a.Revision == request.ExpectedRevision || a.ExtractionRevision == "" || a.ExtractionRevision == request.ExtractionRevision {
		return sessions.Attachment{}, &Error{Code: "invalid_response", Message: "Correction was not confirmed. Inspect retained revisions before retrying."}
	}
	return a, nil
}
