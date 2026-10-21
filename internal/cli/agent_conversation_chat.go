package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

func RunConversationChat(ctx context.Context, client *serviceclient.Client, args []string, stdin io.Reader, stdout io.Writer, jsonMode bool, renderStream func(io.Writer, io.Writer, io.Reader) error) error {
	if jsonMode {
		return Usage("Interactive profile chat requires a terminal; use session send --json for automation")
	}
	if len(args) < 2 || !ValidProfileID(args[1]) {
		return Usage("session chat NAME --profile ID --profile-revision N")
	}
	for i := 2; i < len(args); i += 2 {
		if i+1 >= len(args) || (args[i] != "--profile" && args[i] != "--profile-revision") {
			return Usage("session chat accepts only --profile ID and --profile-revision N")
		}
	}
	req, _, err := ParseTaskSubmission(append(append([]string{}, args[2:]...), "interactive"))
	if err != nil {
		return err
	}
	if req.ProfileID == "" || req.Style != "" || req.MaxIterations != 0 || req.Prompt != "interactive" {
		return Usage("session chat NAME --profile ID --profile-revision N")
	}
	if err := client.RequireCapability(ctx, "agent-conversations-v1"); err != nil {
		return err
	}
	if _, err := LoadServiceSession(ctx, client, args[1]); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Conversation: %s\nAgent: %s Ã‚Â· revision %d\n/status /pause /stop /resume /recover /exit. Exiting does not cancel saved work.\n", TerminalSafe(args[1]), TerminalSafe(req.ProfileID), req.ProfileRevision)
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
		var saved LinkedTurnResponse
		if err := client.JSON(ctx, http.MethodGet, "/v1/sessions/"+url.PathEscape(args[1])+"/turn", nil, &saved); err != nil {
			return err
		}
		if text == "/status" {
			if saved.Job != nil {
				RenderAgentSnapshot(stdout, *saved.Job)
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
			if err := RunConversationService(ctx, client, []string{"recover", args[1], saved.Turn.ID}, stdout, false, renderStream); err != nil {
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
			var snapshot AgentRunSnapshot
			if err := client.JSON(ctx, http.MethodPost, "/api/v2/jobs/"+url.PathEscape(saved.Job.RunID)+"/"+action, map[string]any{}, &snapshot); err != nil {
				return err
			}
			RenderAgentSnapshot(stdout, snapshot)
			continue
		}
		if saved.Job != nil && saved.Job.Status != "completed" && saved.Job.Status != "failed" && saved.Job.Status != "cancelled" {
			if !saved.Job.CanSteer {
				fmt.Fprintln(stdout, "Message not submitted: resolve the current job's access, approval or uncertain outcome first.")
				RenderAgentSnapshot(stdout, *saved.Job)
				continue
			}
			instructionID := uuid.NewString()
			fmt.Fprintf(stdout, "Instruction request: %s\n", instructionID)
			var snapshot AgentRunSnapshot
			if err := client.JSON(ctx, http.MethodPost, "/api/v2/jobs/"+url.PathEscape(saved.Job.RunID)+"/steer", map[string]string{"request_id": instructionID, "instruction": text}, &snapshot); err != nil {
				return fmt.Errorf("%w; inspect job before retrying instruction %s", err, instructionID)
			}
			RenderAgentSnapshot(stdout, snapshot)
			continue
		}
		if err := RunConversationService(ctx, client, []string{"send", args[1], "--profile", req.ProfileID, "--profile-revision", fmt.Sprint(req.ProfileRevision), "--", text}, stdout, false, renderStream); err != nil {
			return err
		}
	}
}
