package serviceclient

import (
	"context"
	"encoding/hex"
	"fmt"
	"github.com/takuphilchan/offgrid-llm/internal/document"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"net/http"
	"net/url"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/sessions"
)

type DocumentPreview struct {
	Evidence  *document.Coverage `json:"evidence,omitempty"`
	Name      string             `json:"name"`
	SHA256    string             `json:"sha256"`
	State     string             `json:"state"`
	ErrorCode string             `json:"error_code"`
	Units     []struct {
		Kind          string             `json:"kind"`
		Index         int                `json:"index"`
		Name          string             `json:"name"`
		Text          string             `json:"text"`
		State         string             `json:"state"`
		SHA256        string             `json:"source_sha256"`
		ImageGeometry *rag.ImageGeometry `json:"image_geometry,omitempty"`
	} `json:"units"`
}

func (c *Client) PreviewSessionFile(ctx context.Context, name string, source sessions.Attachment) (DocumentPreview, error) {
	var preview DocumentPreview
	digest, digestErr := hex.DecodeString(source.SHA256)
	if name == "" || !validAttachmentID(source.ID) || digestErr != nil || len(digest) != 32 {
		return preview, fmt.Errorf("invalid attachment identity")
	}
	bounded, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	client := *c
	transport := *c.http
	transport.Timeout = 2 * time.Minute
	client.http = &transport
	query := url.Values{"sha256": {source.SHA256}}
	if source.Revision != "" {
		query.Set("revision", source.Revision)
	}
	err := client.JSON(bounded, http.MethodGet, sessionFilesPath(name)+"/"+source.ID+"/preview?"+query.Encode(), nil, &preview)
	if err != nil {
		return DocumentPreview{}, err
	}
	if preview.SHA256 != source.SHA256 || preview.Name != source.Name {
		return DocumentPreview{}, fmt.Errorf("preview source identity changed")
	}
	for _, unit := range preview.Units {
		if unit.SHA256 != source.SHA256 || unit.Index < 1 {
			return DocumentPreview{}, fmt.Errorf("invalid preview source unit")
		}
		if g := unit.ImageGeometry; g != nil && (g.Width < 1 || g.Height < 1 || g.Width > 8192 || g.Height > 8192 || int64(g.Width)*int64(g.Height) > 16000000 || g.Orientation < 1 || g.Orientation > 8) {
			return DocumentPreview{}, fmt.Errorf("invalid preview image geometry")
		}
	}
	return preview, nil
}
