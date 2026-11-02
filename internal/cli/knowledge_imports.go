package cli

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

func runKnowledgeImportCommand(ctx context.Context, client *serviceclient.Client, args []string) (any, error) {
	method, path := http.MethodGet, "/v1/documents/imports"
	if args[0] == "imports" {
		if len(args) != 1 {
			return nil, Usage("Usage: offgrid kb imports")
		}
	} else {
		if len(args) != 2 || strings.TrimSpace(args[1]) == "" {
			return nil, Usage("Usage: offgrid kb %s IMPORT_ID", args[0])
		}
		path += "?id=" + url.QueryEscape(args[1])
		switch args[0] {
		case "review-import":
			path += "&view=extraction"
		case "retry-import":
			method = http.MethodPost
			path += "&action=retry"
		case "cancel-import":
			method = http.MethodPost
			path += "&action=cancel"
		case "remove-import":
			method = http.MethodDelete
		}
	}
	var result map[string]any
	if err := client.JSON(ctx, method, path, nil, &result); err != nil {
		return nil, err
	}
	valid := false
	if args[0] == "imports" {
		_, valid = result["imports"].([]any)
	} else if args[0] == "remove-import" {
		valid = result["success"] == true
	} else if args[0] == "review-import" {
		_, unitsOK := result["source_units"].([]any)
		valid = result["version"] == float64(1) && result["state"] == "partial" && unitsOK
	} else {
		id, idOK := result["id"].(string)
		state, stateOK := result["state"].(string)
		valid = idOK && stateOK && id == args[1] && state != ""
	}
	if !valid {
		return nil, &serviceclient.Error{Code: "invalid_response", Message: "Import response is incomplete. Inspect Knowledge before retrying."}
	}
	return result, nil
}
