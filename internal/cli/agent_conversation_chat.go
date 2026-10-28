package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

func RunConversationChat(ctx context.Context, client *serviceclient.Client, args []string, stdin io.Reader, stdout io.Writer, jsonMode bool, renderStream func(io.Writer, io.Writer, io.Reader) error) error {
	return runConversationChatWithAccess(ctx, client, args, stdin, stdout, jsonMode, renderStream, nil)
}

func runConversationChatWithAccess(ctx context.Context, client *serviceclient.Client, args []string, stdin io.Reader, stdout io.Writer, jsonMode bool, renderStream func(io.Writer, io.Writer, io.Reader) error, workspace *conversationWorkspace) error {
	stopInput := context.AfterFunc(ctx, func() {
		if input, ok := stdin.(io.ReadCloser); ok {
			_ = input.Close()
		}
	})
	defer stopInput()
	if jsonMode {
		return Usage("Interactive profile chat requires a terminal; use session send --json for automation")
	}
	if len(args) < 2 || !ValidProfileID(args[1]) {
		return Usage("session chat NAME --profile ID --profile-revision N")
	}
	for i := 2; i < len(args); i += 2 {
		if i+1 >= len(args) || (args[i] != "--profile" && args[i] != "--profile-revision" && args[i] != "--model" && args[i] != "--workspace") {
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
	if req.Workspace != "" {
		ref, resource, err := resolveExecutionWorkspace(ctx, client, req.Workspace)
		if err != nil {
			return err
		}
		req.Execution = &ref
		fmt.Fprintf(stdout, "Execution: %s · %s (directory %s, grant revision %d)\n", TerminalSafe(ref.Host), TerminalSafe(resource.Root), TerminalSafe(resource.Directory), ref.Revision)
	}
	presentation := sessionPresentation(ctx, req.ProfileID, req.ProfileRevision)
	stdout = agentSessionWriter{stdout}
	agentSessionHeader(stdout, presentation)
	reader, ok := stdin.(*bufio.Reader)
	if !ok {
		reader = bufio.NewReader(stdin)
	}
	if workspace != nil {
		workspace.env.Stdin = reader
		workspace.env.Stdout = stdout
		workspace.env.Stderr = agentSessionWriter{workspace.env.Stderr}
	}
	for {
		var line string
		var readErr error
		if presentation.ReadLine != nil {
			line, readErr = presentation.ReadLine(reader, stdout, presentation.Style, AgentCommandSuggestions())
		} else {
			agentSessionPrompt(stdout, presentation.Style)
			line, readErr = readConversationLine(reader)
		}
		if readErr != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if errors.Is(readErr, io.EOF) {
				return nil
			}
			return readErr
		}
		text := strings.TrimSpace(line)
		if text == "" {
			continue
		}
		if text == "/exit" || text == "/quit" {
			fmt.Fprintf(stdout, "\nConversation saved. Exiting does not cancel saved work.\nResume: offgrid agent chat --resume %s --profile %s --profile-revision %d\n", TerminalSafe(args[1]), TerminalSafe(req.ProfileID), req.ProfileRevision)
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if renderAgentSessionCommand(stdout, text, args[1], presentation) {
			continue
		}
		if !knownAgentSlashCommand(text) {
			fmt.Fprintln(stdout, "Unknown command. Type /help to see session commands. No message was submitted.")
			continue
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
		sendArgs := []string{"send", args[1], "--profile", req.ProfileID, "--profile-revision", fmt.Sprint(req.ProfileRevision)}
		if req.Model != "" {
			sendArgs = append(sendArgs, "--model", req.Model)
		}
		requestID := uuid.NewString()
		sendArgs = append(sendArgs, "--request-id", requestID, "--wait", "--", text)
		if err := runConversationServiceWithAccess(ctx, client, sendArgs, stdout, false, renderStream, req.Execution, workspace); err != nil {
			if ctx.Err() != nil {
				return errors.Join(err, stopInteractiveTurn(ctx, client, args[1], requestID, stdout))
			}
			if stopped := stoppedInteractiveTurn(ctx, client, args[1], requestID); stopped != nil {
				if stopped.Status == "completed" {
					RenderAgentSnapshot(stdout, *stopped)
				} else {
					fmt.Fprintln(stdout, "Saved turn remains available. Use /status to inspect it or /exit to close this session.")
				}
				continue
			}
			return errors.Join(err, stopInteractiveTurn(ctx, client, args[1], requestID, stdout))
		}
	}
}

// Share the reader with on-demand consent. A second Scanner could consume
// the approval or next message into its private buffer.
func readConversationLine(reader *bufio.Reader) (string, error) {
	var line []byte
	for {
		part, err := reader.ReadSlice('\n')
		if len(line)+len(part) > 32001 {
			return "", Usage("Message exceeds 32000 bytes")
		}
		line = append(line, part...)
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		if errors.Is(err, io.EOF) && len(line) > 0 {
			return string(line), nil
		}
		return string(line), err
	}
}

// A dropped event stream alone does not cancel service-owned work. Ctrl+C may
// request Stop only for the exact turn this foreground invocation submitted.
func stopInteractiveTurn(parent context.Context, client *serviceclient.Client, name, requestID string, stdout io.Writer) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), 5*time.Second)
	defer cancel()
	var saved LinkedTurnResponse
	if err := client.JSON(ctx, http.MethodGet, "/v1/sessions/"+url.PathEscape(name)+"/turn", nil, &saved); err != nil {
		return fmt.Errorf("Stop not confirmed for request %s; inspect the saved conversation: %w", requestID, err)
	}
	if saved.Turn == nil || saved.Turn.ID != requestID || saved.Job == nil || saved.Turn.JobID != saved.Job.RunID {
		return fmt.Errorf("Stop not confirmed for request %s; inspect the saved conversation before retrying", requestID)
	}
	if saved.Job.Status == "completed" || saved.Job.Status == "failed" || saved.Job.Status == "cancelled" {
		return nil
	}
	var result AgentRunSnapshot
	if err := client.JSON(ctx, http.MethodPost, "/api/v2/jobs/"+url.PathEscape(saved.Job.RunID)+"/cancel", map[string]any{}, &result); err != nil {
		return fmt.Errorf("Stop not confirmed for saved job %s: %w", saved.Job.RunID, err)
	}
	if result.RunID != saved.Job.RunID || !ValidAgentStatus(result.Status) {
		return fmt.Errorf("Invalid stop acknowledgment; inspect saved job %s", saved.Job.RunID)
	}
	fmt.Fprintf(stdout, "Stop requested for saved job %s; actions already dispatched are not undone.\n", TerminalSafe(result.RunID))
	return nil
}
