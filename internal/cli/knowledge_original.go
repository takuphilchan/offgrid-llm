package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

// Download through the selected authenticated service, never a source URL or
// model-supplied path. Verify the full bounded response before creating output.
func downloadKnowledgeOriginal(ctx context.Context, client *serviceclient.Client, id, filename string) (any, error) {
	response, err := client.Do(ctx, http.MethodGet, "/v1/documents/source?id="+url.QueryEscape(id)+"&format=original", "", nil)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	const limit = 50 << 20
	data, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	digest := sha256.Sum256(data)
	hash := hex.EncodeToString(digest[:])
	if len(data) == 0 || len(data) > limit || response.Header.Get("Content-Type") != "application/octet-stream" || response.Header.Get("X-OffGrid-Source-SHA256") != hash {
		return nil, fmt.Errorf("original response failed size, type or digest validation; no file was written")
	}
	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, err
	}
	owned, statErr := file.Stat()
	_, err = file.Write(data)
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		if current, checkErr := os.Lstat(filename); statErr == nil && checkErr == nil && os.SameFile(owned, current) {
			_ = os.Remove(filename)
		}
		return nil, err
	}
	return map[string]any{"success": true, "document_id": id, "output": filename, "bytes": len(data), "sha256": hash}, nil
}
