package agents

import (
	"fmt"
	"net/url"
)

// Only a rejection before constructing/sending the request proves no network
// effects. Transport errors after dispatch must keep the normal uncertainty.
func validateHTTPGetURL(value string) error {
	u, err := url.Parse(value)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return invalidHTTPGet(fmt.Errorf("expected an absolute HTTP or HTTPS URL"))
	}
	return nil
}

func invalidHTTPGet(cause error) *ToolFailure {
	return &ToolFailure{Code: "tool_http_url_rejected", Message: "No request was sent. http_get requires an absolute HTTP or HTTPS URL and cannot access local files.", Cause: cause, BeforeEffects: true,
		Recovery: "For local files or commands, use the offered workspace tools. If no workspace is attached, call request_workspace_access when offered, with the minimum required access. Do not invent URLs or substitute service files. For an actual web request, correct its HTTP or HTTPS URL."}
}
