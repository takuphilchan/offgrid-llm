package cli

import (
	"context"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

// Accept the source only. As for a multipart acceptance, indexing is a separate
// explicit retry-import operation and never a side effect of inspection.
func importKnowledgeAttachment(ctx context.Context, client *serviceclient.Client, args []string) (any, error) {
	if len(args) != 4 || strings.TrimSpace(args[1]) == "" {
		return nil, Usage("Usage: offgrid kb import-attachment SESSION FILE_ID SHA256; then retry-import IMPORT_ID to index")
	}
	id, idErr := hex.DecodeString(strings.TrimPrefix(args[2], "file-"))
	digest, digestErr := hex.DecodeString(args[3])
	if idErr != nil || !strings.HasPrefix(args[2], "file-") || len(id) != 16 || digestErr != nil || len(digest) != 32 {
		return nil, Usage("Choose a saved attachment ID and its exact SHA-256 from the conversation file listing")
	}
	var result map[string]any
	body := map[string]any{"attachment": map[string]string{"session": args[1], "id": args[2], "sha256": args[3]}}
	if err := client.JSON(ctx, http.MethodPost, "/v1/documents/imports", body, &result); err != nil {
		return nil, err
	}
	idValue, _ := result["id"].(string)
	state, _ := result["state"].(string)
	if idValue == "" || state == "" || result["sha256"] != args[3] {
		return nil, &serviceclient.Error{Code: "invalid_response", Message: "Attachment import was not confirmed. Inspect saved Knowledge imports before continuing."}
	}
	return result, nil
}
