package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
)

type LinkedTurnResponse struct {
	Turn             *sessions.Turn    `json:"turn"`
	Job              *AgentRunSnapshot `json:"job,omitempty"`
	RecoveryRequired bool              `json:"recovery_required,omitempty"`
}

// Existing conversations and jobs remain service-owned. Observing a turn does
// not call recover, release, approve, or resume.
func RunConversationService(ctx context.Context, client *serviceclient.Client, args []string, stdout io.Writer, jsonMode bool, renderStream func(io.Writer, io.Writer, io.Reader) error) error {
	return runConversationService(ctx, client, args, stdout, jsonMode, renderStream, nil)
}

func runConversationService(ctx context.Context, client *serviceclient.Client, args []string, stdout io.Writer, jsonMode bool, renderStream func(io.Writer, io.Writer, io.Reader) error, pinned *execution.Reference) error {
	return runConversationServiceWithAccess(ctx, client, args, stdout, jsonMode, renderStream, pinned, nil)
}

type conversationWorkspace struct {
	request TaskSubmission
	env     Environment
	lease   ExecutionLease
}

func runConversationServiceWithAccess(ctx context.Context, client *serviceclient.Client, args []string, stdout io.Writer, jsonMode bool, renderStream func(io.Writer, io.Writer, io.Reader) error, pinned *execution.Reference, workspace *conversationWorkspace) error {
	if len(args) < 2 || !ValidProfileID(args[1]) {
		return Usage("session send|turn|recover NAME [options]")
	}
	path := "/v1/sessions/" + url.PathEscape(args[1]) + "/turn"
	method := http.MethodGet
	var body any
	requestID := ""
	wait := false
	switch args[0] {
	case "turn":
		if len(args) != 2 {
			return Usage("session turn NAME")
		}
	case "recover":
		if len(args) != 3 || len(args[2]) < 8 || len(args[2]) > 128 || !ValidProfileID(args[2]) {
			return Usage("session recover NAME REQUEST_ID")
		}
		requestID = args[2]
		method = http.MethodPost
		path += "/recover"
		body = map[string]any{"id": requestID}
	case "send":
		req, follow, err := ParseTaskSubmission(args[2:])
		if err != nil {
			return err
		}
		if req.ProfileID == "" || req.Style != "" || req.MaxIterations != 0 {
			return Usage("session send NAME --profile ID --profile-revision N [--request-id ID] [--wait] <message>")
		}
		requestID = req.RequestID
		wait = follow
		method = http.MethodPost
		path = "/v1/sessions/" + url.PathEscape(args[1]) + "/generate"
		body = map[string]any{"content": req.Prompt, "request_id": requestID, "agent": sessions.AgentSelection{ID: req.ProfileID, Revision: req.ProfileRevision}, "durable": true, "profile": "interactive"}
		if req.ResponseProfile != "" {
			body.(map[string]any)["profile"] = req.ResponseProfile
		}
		if req.Workspace != "" {
			ref, _, err := resolveExecutionWorkspace(ctx, client, req.Workspace)
			if err != nil {
				return err
			}
			req.Execution = &ref
		}
		if pinned != nil {
			req.Execution = pinned
		}
		if workspace != nil {
			body.(map[string]any)["execution_request_version"] = 1
			if workspace.lease != nil {
				ref := workspace.lease.Resource().Reference
				req.Execution = &ref
			}
		}
		if req.Execution != nil {
			body.(map[string]any)["execution"] = req.Execution
		}
		if req.Model != "" {
			body.(map[string]any)["model_id"] = req.Model
		}
	default:
		return Usage("Unknown conversation operation")
	}
	if err := client.RequireCapability(ctx, "agent-conversations-v1"); err != nil {
		return err
	}
	var result LinkedTurnResponse
	if err := client.JSON(ctx, method, path, body, &result); err != nil {
		if requestID != "" {
			return fmt.Errorf("%w (request %s; inspect session turn before retrying the same request)", err, TerminalSafe(requestID))
		}
		return err
	}
	if result.Turn != nil && requestID != "" && result.Turn.ID != requestID {
		return fmt.Errorf("The service returned a different turn; inspect the conversation before retrying")
	}
	if result.Job != nil && (result.Turn == nil || result.Turn.JobID != result.Job.RunID || !ValidAgentStatus(result.Job.Status)) {
		return fmt.Errorf("The service returned inconsistent conversation/job identities")
	}
	if args[0] == "send" && (result.Turn == nil || result.Job == nil) {
		return fmt.Errorf("Missing canonical job after submission; inspect session turn before retrying")
	}
	if jsonMode {
		return json.NewEncoder(stdout).Encode(result)
	}
	if result.Turn == nil {
		_, err := fmt.Fprintln(stdout, "No saved turn.")
		return err
	}
	if !AgentSessionOutput(stdout) {
		fmt.Fprintf(stdout, "Turn: %s\n", TerminalSafe(result.Turn.ID))
	}
	if result.Job == nil {
		fmt.Fprintf(stdout, "Status: %s\n%s\n", TerminalSafe(result.Turn.Status), TerminalSafe(result.Turn.Output))
		return nil
	}
	if (!wait || !foregroundRunLive(result.Job.Status)) && !AgentSessionOutput(stdout) {
		fmt.Fprintf(stdout, "Run: %s\n", TerminalSafe(result.Job.RunID))
	}
	if wait && workspace != nil {
		return runWithWorkspaceInput(ctx, client, workspace.request, result.Job, workspace.env, &workspace.lease)
	}
	following := wait && (result.Job.Status == "pending" || result.Job.Status == "running" || result.Job.Status == "waiting_for_children")
	if !following {
		if AgentSessionOutput(stdout) && result.Job.Status == "completed" && result.Job.Error == "" && result.Job.PendingApproval == nil && result.Job.PendingInput == nil {
			fmt.Fprintf(stdout, "\nOffGrid\n\n%s\n\nCompleted\n", TerminalSafe(result.Job.Output))
		} else {
			RenderAgentSnapshot(stdout, *result.Job)
		}
	}
	if result.RecoveryRequired {
		fmt.Fprintf(stdout, "Saved work needs conversation bookkeeping. No action was repeated.\n  offgrid session recover %s %s\n", TerminalSafe(args[1]), TerminalSafe(result.Turn.ID))
	}
	if following {
		if renderStream == nil {
			return fmt.Errorf("agent stream renderer unavailable")
		}
		response, err := client.Events(ctx, "/api/v2/jobs/"+url.PathEscape(result.Job.RunID)+"/events")
		if err != nil {
			return err
		}
		defer response.Body.Close()
		return renderStream(stdout, stdout, response.Body)
	}
	return nil
}
