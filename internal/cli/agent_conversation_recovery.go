package cli

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

// A known stopped turn need not destroy the interactive conversation or its
// locally approved workspace. Unknown stream outcomes retain explicit stopping;
// no response text, network error or mismatched turn can establish completion.
func stoppedInteractiveTurn(ctx context.Context, client *serviceclient.Client, name, requestID string) *AgentRunSnapshot {
	check, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var saved LinkedTurnResponse
	if client.JSON(check, http.MethodGet, "/v1/sessions/"+url.PathEscape(name)+"/turn", nil, &saved) != nil || saved.Turn == nil || saved.Turn.ID != requestID || saved.Job == nil || saved.Turn.JobID != saved.Job.RunID {
		return nil
	}
	switch saved.Job.Status {
	case "completed", "failed", "cancelled", "interrupted", "uncertain", "waiting_for_input", "waiting_for_approval":
		return saved.Job
	}
	return nil
}
