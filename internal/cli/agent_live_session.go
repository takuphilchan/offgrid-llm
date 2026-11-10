package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

// LiveAgentStream owns input only while following a known run. It must return
// all unfinished input as drafts and relinquish input before an access prompt.
// Ordinary/piped callers keep the existing stream renderer.
type LiveAgentStream func(context.Context, *bufio.Reader, io.Writer, io.Reader, func(string) (string, bool)) ([]string, error)

// Live controls never submit a replacement task or answer an approval. A fresh
// service snapshot is authoritative; the renderer's progress can be stale.
func liveAgentControl(ctx context.Context, client *serviceclient.Client, runID string) func(string) (string, bool) {
	uncertain := false
	return func(text string) (string, bool) {
		text = strings.TrimSpace(text)
		stop := text == "/stop"
		if !stop {
			if ctx.Err() != nil {
				return "The session is stopping. Your draft is retained.", false
			}
			text = strings.TrimSpace(strings.TrimPrefix(text, "/steer "))
			if text == "" || strings.HasPrefix(text, "/") || len(text) > 32000 {
				return "Enter a correction, /queue MESSAGE, or /stop. Nothing was sent.", false
			}
			if uncertain {
				return "The previous correction was not confirmed. Inspect saved work before sending another.", false
			}
		}
		controlCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		var current AgentRunSnapshot
		path := "/api/v2/jobs/" + url.PathEscape(runID)
		if err := client.JSON(controlCtx, http.MethodGet, path, nil, &current); err != nil {
			return "Could not check this task. Your correction is retained: " + TerminalSafe(err.Error()), false
		}
		if current.RunID != runID || !ValidAgentStatus(current.Status) {
			return "Task identity could not be verified. Nothing was sent.", false
		}
		if stop {
			if current.Status == "completed" || current.Status == "cancelled" || current.Status == "failed" {
				return "Task already ended. Completed actions remain.", true
			}
			var stopped AgentRunSnapshot
			if err := client.JSON(controlCtx, http.MethodPost, path+"/cancel", map[string]any{}, &stopped); err != nil {
				return "Stop is not confirmed. Inspect this saved task: " + TerminalSafe(err.Error()), false
			}
			if stopped.RunID != runID || !ValidAgentStatus(stopped.Status) {
				return "Stop is not confirmed; the service returned an inconsistent task.", false
			}
			return "Stop response: " + TerminalSafe(stopped.Status) + ". Completed actions remain.", true
		}
		if !foregroundRunLive(current.Status) || !current.CanSteer || current.PendingApproval != nil || current.PendingInput != nil || current.UncertainCallID != "" {
			return "This task cannot accept a correction now. Your draft is retained; resolve access or inspect the outcome first.", false
		}
		id := uuid.NewString()
		var accepted AgentRunSnapshot
		if err := client.JSON(controlCtx, http.MethodPost, path+"/steer", map[string]string{"request_id": id, "instruction": text}, &accepted); err != nil {
			uncertain = true
			return fmt.Sprintf("Correction %s was not confirmed. Inspect saved work before retrying: %s", id, TerminalSafe(err.Error())), false
		}
		if accepted.RunID != runID || !ValidAgentStatus(accepted.Status) {
			uncertain = true
			return "Correction was not confirmed; inspect saved work before retrying request " + id + ".", false
		}
		return "Correction accepted for a safe boundary.", true
	}
}

func followConversationStream(ctx context.Context, client *serviceclient.Client, runID string, out, progress io.Writer, body io.Reader, fallback func(io.Writer, io.Writer, io.Reader) error) error {
	if live, ok := ctx.Value(liveAgentSessionKey{}).(*liveAgentSession); ok && live.follow != nil {
		drafts, err := live.follow(ctx, live.input, out, body, liveAgentControl(ctx, client, runID))
		live.drafts = append(live.drafts, drafts...)
		return err
	}
	return fallback(out, progress, body)
}

type liveAgentSessionKey struct{}
type liveAgentSession struct {
	input  *bufio.Reader
	follow LiveAgentStream
	drafts []string
}
