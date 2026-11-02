package cli

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

func runKnowledgeRetention(ctx context.Context, client *serviceclient.Client, args []string) (any, error) {
	const usage = "offgrid kb retention ID [--revision N --delete-after UTC_UNIX_SECONDS --legal-hold true|false]"
	if len(args) != 2 && len(args) != 8 || len(args) > 1 && strings.TrimSpace(args[1]) == "" {
		return nil, Usage("Usage: %s", usage)
	}
	method := http.MethodGet
	var body any
	if len(args) == 8 {
		values := map[string]string{}
		for i := 2; i < len(args); i += 2 {
			if _, ok := values[args[i]]; ok {
				return nil, Usage("Do not repeat retention options")
			}
			values[args[i]] = args[i+1]
		}
		revision, err1 := strconv.Atoi(values["--revision"])
		deleteAfter, err2 := strconv.ParseInt(values["--delete-after"], 10, 64)
		hold, ok := values["--legal-hold"]
		if len(values) != 3 || err1 != nil || revision < 1 || err2 != nil || deleteAfter < 0 || deleteAfter > 253402300799 || !ok || (hold != "true" && hold != "false") {
			return nil, Usage("Usage: %s", usage)
		}
		body = rag.DocumentRetention{Version: 1, Revision: revision, DeleteAfter: deleteAfter, LegalHold: hold == "true"}
		method = http.MethodPut
	}
	var wire struct {
		Version     int    `json:"version"`
		Revision    int    `json:"revision"`
		DeleteAfter *int64 `json:"delete_after"`
		LegalHold   *bool  `json:"legal_hold"`
	}
	if err := client.JSON(ctx, method, "/v1/documents/retention?id="+url.QueryEscape(args[1]), body, &wire); err != nil {
		return nil, err
	}
	if wire.Version != 1 || wire.Revision < 1 || wire.DeleteAfter == nil || wire.LegalHold == nil || *wire.DeleteAfter < 0 || *wire.DeleteAfter > 253402300799 {
		return nil, &serviceclient.Error{Code: "invalid_response", Message: "Retention response is incomplete. Inspect the document before retrying."}
	}
	result := rag.DocumentRetention{Version: wire.Version, Revision: wire.Revision, DeleteAfter: *wire.DeleteAfter, LegalHold: *wire.LegalHold}
	if sent, ok := body.(rag.DocumentRetention); ok && (result.Revision != sent.Revision+1 || result.DeleteAfter != sent.DeleteAfter || result.LegalHold != sent.LegalHold) {
		return nil, &serviceclient.Error{Code: "invalid_response", Message: "Retention update was not confirmed. Inspect the document before retrying."}
	}
	return result, nil
}
