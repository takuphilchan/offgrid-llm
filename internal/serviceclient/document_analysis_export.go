package serviceclient

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"unicode/utf8"
)

type DocumentAnalysisExport struct {
	AnalysisID string `json:"analysis_id"`
	Format     string `json:"format"`
	Content    string `json:"content"`
	SHA256     string `json:"sha256"`
}

func (c *Client) ExportDocumentAnalysis(ctx context.Context, id, format string) (*DocumentAnalysisExport, error) {
	return c.exportDocumentAnalysis(ctx, "/v1/document-analyses/export", id, format)
}

func (c *Client) ExportDocumentSynthesis(ctx context.Context, id, format string) (*DocumentAnalysisExport, error) {
	return c.exportDocumentAnalysis(ctx, "/v1/document-analyses/synthesis/export", id, format)
}

func (c *Client) exportDocumentAnalysis(ctx context.Context, endpoint, id, format string) (*DocumentAnalysisExport, error) {
	if id == "" || len(id) > 128 || (format != "json" && format != "csv" && format != "markdown") {
		return nil, &Error{Code: "invalid_request", Message: "Provide an analysis ID and json, csv or markdown format."}
	}
	resp, err := c.Do(ctx, http.MethodGet, endpoint+"?"+url.Values{"id": {id}, "format": {format}}.Encode(), "", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	// JSON escaping can expand an 8 MiB UTF-8 artifact to six times its size.
	data, err := io.ReadAll(io.LimitReader(resp.Body, (49<<20)+1))
	if err != nil {
		return nil, err
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	var result DocumentAnalysisExport
	if len(data) > 49<<20 || json.Unmarshal(data, &result) != nil || result.AnalysisID != id || result.Format != format || len(result.Content) == 0 || len(result.Content) > 8<<20 || !utf8.ValidString(result.Content) {
		return nil, &Error{Code: "invalid_response", Message: "Export identity or size was not confirmed."}
	}
	hash := sha256.Sum256([]byte(result.Content))
	if result.SHA256 != hex.EncodeToString(hash[:]) {
		return nil, &Error{Code: "invalid_response", Message: "Export digest was not confirmed."}
	}
	return &result, nil
}
