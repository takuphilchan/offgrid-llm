package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
)

func conversationChat(ctx context.Context, client *serviceclient.Client, args []string, stdin io.Reader, stdout io.Writer, jsonMode bool) error {
	if jsonMode {
		return usage("Interactive profile chat requires a terminal; use session send --json for automation")
	}
	if len(args) < 2 || !validProfileID(args[1]) {
		return usage("session chat NAME --profile ID --profile-revision N")
	}
	for i := 2; i < len(args); i += 2 {
		if i+1 >= len(args) || (args[i] != "--profile" && args[i] != "--profile-revision") {
			return usage("session chat accepts only --profile ID and --profile-revision N")
		}
	}
	req, _, err := parseTaskSubmission(append(append([]string{}, args[2:]...), "interactive"))
	if err != nil {
		return err
	}
	if req.ProfileID == "" || req.Style != "" || req.MaxIterations != 0 || req.Prompt != "interactive" {
		return usage("session chat NAME --profile ID --profile-revision N")
	}
	if err := client.RequireCapability(ctx, "agent-conversations-v1"); err != nil {
		return err
	}
	if _, err := loadServiceSession(ctx, client, args[1]); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Conversation: %s\nAgent: %s · revision %d\n/status /pause /stop /resume /recover /exit. Exiting does not cancel saved work.\n", terminalSafe(args[1]), terminalSafe(req.ProfileID), req.ProfileRevision)
	scanner := bufio.NewScanner(stdin)
	scanner.Buffer(make([]byte, 4096), 32001)
	for {
		fmt.Fprint(stdout, "> ")
		if !scanner.Scan() {
			return scanner.Err()
		}
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}
		if text == "/exit" || text == "/quit" {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		var saved linkedTurnResponse
		if err := client.JSON(ctx, http.MethodGet, "/v1/sessions/"+url.PathEscape(args[1])+"/turn", nil, &saved); err != nil {
			return err
		}
		if text == "/status" {
			if saved.Job != nil {
				renderAgentSnapshot(stdout, *saved.Job)
			} else {
				fmt.Fprintln(stdout, "No linked job.")
			}
			continue
		}
		if text == "/recover" {
			if saved.Turn == nil {
				fmt.Fprintln(stdout, "No saved turn.")
				continue
			}
			if err := conversationCommand(ctx, client, []string{"recover", args[1], saved.Turn.ID}, stdout, false); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(text, "/") {
			action := strings.TrimPrefix(text, "/")
			if action == "stop" {
				action = "cancel"
			}
			if action != "pause" && action != "cancel" && action != "resume" {
				fmt.Fprintln(stdout, "Use /status, /pause, /stop, /resume, /recover or /exit.")
				continue
			}
			if saved.Job == nil {
				fmt.Fprintln(stdout, "No linked job.")
				continue
			}
			var snapshot agentRunSnapshot
			if err := client.JSON(ctx, http.MethodPost, "/api/v2/jobs/"+url.PathEscape(saved.Job.RunID)+"/"+action, map[string]any{}, &snapshot); err != nil {
				return err
			}
			renderAgentSnapshot(stdout, snapshot)
			continue
		}
		if saved.Job != nil && saved.Job.Status != "completed" && saved.Job.Status != "failed" && saved.Job.Status != "cancelled" {
			if !saved.Job.CanSteer {
				fmt.Fprintln(stdout, "Message not submitted: resolve the current job's access, approval or uncertain outcome first.")
				renderAgentSnapshot(stdout, *saved.Job)
				continue
			}
			instructionID := uuid.NewString()
			fmt.Fprintf(stdout, "Instruction request: %s\n", instructionID)
			var snapshot agentRunSnapshot
			if err := client.JSON(ctx, http.MethodPost, "/api/v2/jobs/"+url.PathEscape(saved.Job.RunID)+"/steer", map[string]string{"request_id": instructionID, "instruction": text}, &snapshot); err != nil {
				return fmt.Errorf("%w; inspect job before retrying instruction %s", err, instructionID)
			}
			renderAgentSnapshot(stdout, snapshot)
			continue
		}
		if err := conversationCommand(ctx, client, []string{"send", args[1], "--profile", req.ProfileID, "--profile-revision", fmt.Sprint(req.ProfileRevision), "--", text}, stdout, false); err != nil {
			return err
		}
	}
}

type linkedTurnResponse struct {
	Turn             *sessions.Turn    `json:"turn"`
	Job              *agentRunSnapshot `json:"job,omitempty"`
	RecoveryRequired bool              `json:"recovery_required,omitempty"`
}

// Existing conversations and jobs remain service-owned. Observing a turn does
// not call recover, release, approve, or resume.
func conversationCommand(ctx context.Context, client *serviceclient.Client, args []string, stdout io.Writer, jsonMode bool) error {
	if len(args) < 2 || !validProfileID(args[1]) {
		return usage("session send|turn|recover NAME [options]")
	}
	path := "/v1/sessions/" + url.PathEscape(args[1]) + "/turn"
	method := http.MethodGet
	var body any
	requestID := ""
	wait := false
	switch args[0] {
	case "turn":
		if len(args) != 2 {
			return usage("session turn NAME")
		}
	case "recover":
		if len(args) != 3 || len(args[2]) < 8 || len(args[2]) > 128 || !validProfileID(args[2]) {
			return usage("session recover NAME REQUEST_ID")
		}
		requestID = args[2]
		method = http.MethodPost
		path += "/recover"
		body = map[string]any{"id": requestID}
	case "send":
		req, follow, err := parseTaskSubmission(args[2:])
		if err != nil {
			return err
		}
		if req.ProfileID == "" || req.Style != "" || req.MaxIterations != 0 {
			return usage("session send NAME --profile ID --profile-revision N [--request-id ID] [--wait] <message>")
		}
		requestID = req.RequestID
		wait = follow
		method = http.MethodPost
		path = "/v1/sessions/" + url.PathEscape(args[1]) + "/generate"
		body = map[string]any{"content": req.Prompt, "request_id": requestID, "agent": sessions.AgentSelection{ID: req.ProfileID, Revision: req.ProfileRevision}, "durable": true, "profile": "interactive"}
		if req.Model != "" {
			body.(map[string]any)["model_id"] = req.Model
		}
	default:
		return usage("Unknown conversation operation")
	}
	if err := client.RequireCapability(ctx, "agent-conversations-v1"); err != nil {
		return err
	}
	var result linkedTurnResponse
	if err := client.JSON(ctx, method, path, body, &result); err != nil {
		if requestID != "" {
			return fmt.Errorf("%w (request %s; inspect session turn before retrying the same request)", err, terminalSafe(requestID))
		}
		return err
	}
	if result.Turn != nil && requestID != "" && result.Turn.ID != requestID {
		return fmt.Errorf("The service returned a different turn; inspect the conversation before retrying")
	}
	if result.Job != nil && (result.Turn == nil || result.Turn.JobID != result.Job.RunID || !validAgentStatus(result.Job.Status)) {
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
	fmt.Fprintf(stdout, "Turn: %s\n", terminalSafe(result.Turn.ID))
	if result.Job == nil {
		fmt.Fprintf(stdout, "Status: %s\n%s\n", terminalSafe(result.Turn.Status), terminalSafe(result.Turn.Output))
		return nil
	}
	fmt.Fprintf(stdout, "Run: %s\n", terminalSafe(result.Job.RunID))
	renderAgentSnapshot(stdout, *result.Job)
	if result.RecoveryRequired {
		fmt.Fprintf(stdout, "Saved work needs conversation bookkeeping. No action was repeated.\n  offgrid session recover %s %s\n", terminalSafe(args[1]), terminalSafe(result.Turn.ID))
	}
	if wait && (result.Job.Status == "pending" || result.Job.Status == "running" || result.Job.Status == "waiting_for_children") {
		response, err := client.Do(ctx, http.MethodGet, "/api/v2/jobs/"+url.PathEscape(result.Job.RunID)+"/events", "", nil)
		if err != nil {
			return err
		}
		defer response.Body.Close()
		return renderAgentStreamTo(stdout, stdout, response.Body)
	}
	return nil
}
