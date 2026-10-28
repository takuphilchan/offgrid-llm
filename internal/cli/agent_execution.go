package cli

import (
	"context"
	"fmt"
	"net/http"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

// Selecting a published host does not create a grant or resolve a local path
// on the service. Keep the returned exact revision pinned across chat turns.
func resolveExecutionWorkspace(ctx context.Context, client *serviceclient.Client, session string) (execution.Reference, execution.Resource, error) {
	var catalog struct {
		Workspaces []execution.Resource `json:"workspaces"`
		Protocol   int                  `json:"protocol"`
	}
	if err := client.JSON(ctx, http.MethodGet, "/api/v2/execution/workspaces", nil, &catalog); err != nil {
		return execution.Reference{}, execution.Resource{}, fmt.Errorf("execution workspace discovery failed; use a matching service and locally approved host: %w", err)
	}
	if catalog.Protocol != execution.ProtocolVersion {
		return execution.Reference{}, execution.Resource{}, fmt.Errorf("execution protocol mismatch; no fallback was used")
	}
	var selected *execution.Resource
	for _, resource := range catalog.Workspaces {
		if resource.Reference.Session == session {
			if selected != nil || resource.Reference.Validate() != nil {
				return execution.Reference{}, execution.Resource{}, fmt.Errorf("ambiguous or invalid execution identity")
			}
			copy := resource
			selected = &copy
		}
	}
	if selected == nil {
		return execution.Reference{}, execution.Resource{}, fmt.Errorf("execution workspace %s is unavailable; connect and approve it locally before starting work", TerminalSafe(session))
	}
	return selected.Reference, *selected, nil
}
