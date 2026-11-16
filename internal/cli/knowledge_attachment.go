package cli

import (
	"context"
	"encoding/hex"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

// Accept the source only. As for a multipart acceptance, indexing is a separate
// explicit retry-import operation and never a side effect of inspection.
func importKnowledgeAttachment(ctx context.Context, client *serviceclient.Client, args []string) (any, error) {
	if (len(args) != 6 && len(args) != 7) || strings.TrimSpace(args[1]) == "" || strings.TrimSpace(args[4]) == "" {
		return nil, Usage("Usage: offgrid kb import-attachment SESSION FILE_ID SHA256 COLLECTION_ID REVIEWED_REVISION [ATTACHMENT_REVISION]; then retry-import IMPORT_ID to index")
	}
	revision, err := strconv.Atoi(args[5])
	if err != nil || revision < 1 {
		return nil, Usage("Review the destination collection and provide its current revision")
	}
	id, idErr := hex.DecodeString(strings.TrimPrefix(args[2], "file-"))
	digest, digestErr := hex.DecodeString(args[3])
	if idErr != nil || !strings.HasPrefix(args[2], "file-") || len(id) != 16 || digestErr != nil || len(digest) != 32 {
		return nil, Usage("Choose a saved attachment ID and its exact SHA-256 from the conversation file listing")
	}
	var result map[string]any
	ref := map[string]string{"session": args[1], "id": args[2], "sha256": args[3]}
	if len(args) == 7 {
		decoded, err := hex.DecodeString(args[6])
		if err != nil || len(decoded) != 32 {
			return nil, Usage("Provide the exact attachment revision")
		}
		if err := client.RequireCapability(ctx, "session-document-corrections-v1"); err != nil {
			return nil, err
		}
		ref["revision"] = args[6]
	}
	body := map[string]any{"attachment": ref}
	query := url.Values{"collection_id": {args[4]}, "collection_revision": {strconv.Itoa(revision)}}
	if err := client.JSON(ctx, http.MethodPost, "/v1/collections/imports?"+query.Encode(), body, &result); err != nil {
		return nil, err
	}
	idValue, _ := result["id"].(string)
	state, _ := result["state"].(string)
	if idValue == "" || state == "" || result["sha256"] != args[3] || result["collection_id"] != args[4] || result["collection_revision"] != float64(revision) {
		return nil, &serviceclient.Error{Code: "invalid_response", Message: "Attachment import was not confirmed. Inspect saved Knowledge imports before continuing."}
	}
	return result, nil
}
