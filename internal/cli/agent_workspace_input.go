package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

// Foreground access is attached to the submitted root. Never resubmit a prompt
// after approval, loss of a response, a model error or an interrupted stream.
func runWithWorkspaceInput(ctx context.Context, client *serviceclient.Client, req TaskSubmission, snapshot *AgentRunSnapshot, env Environment, retained ...*ExecutionLease) (result error) {
	var owned ExecutionLease
	if len(retained) > 0 {
		owned = *retained[0]
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	stopWatch := func() {}
	watch := func(lease ExecutionLease) {
		watchCtx, stop := context.WithCancel(runCtx)
		stopWatch = stop
		go func() {
			select {
			case <-lease.Done():
				if watchCtx.Err() == nil {
					cancel()
				}
			case <-watchCtx.Done():
			}
		}()
	}
	if owned != nil {
		watch(owned)
	}
	defer func() {
		stopWatch()
		if owned != nil {
			result = errors.Join(result, settleForegroundRun(client, snapshot.RunID, true, result))
			if len(retained) > 0 {
				*retained[0] = owned
			} else {
				result = errors.Join(result, closeExecutionLease(owned))
			}
		}
	}()
	for {
		rendered := false
		if foregroundRunLive(snapshot.Status) {
			if err := streamForegroundRun(runCtx, client, snapshot, false, env); err != nil {
				return err
			}
			rendered = true
		}
		current, err := AgentControlService(runCtx, client, []string{"status", snapshot.RunID})
		if err != nil {
			return err
		}
		snapshot = current
		input := snapshot.PendingInput
		if snapshot.Status != "waiting_for_input" || input == nil || input.Kind != "workspace" {
			if !foregroundRunLive(snapshot.Status) {
				if !rendered {
					RenderAgentSnapshot(env.Stdout, *snapshot)
				}
				return settleForegroundRun(client, snapshot.RunID, owned != nil, nil)
			}
			continue
		}
		if owned != nil {
			// A permission upgrade replaces only this invocation's owned host.
			// Stop it conclusively before asking for a new local decision.
			stopWatch()
			if err := closeExecutionLease(owned); err != nil {
				return err
			}
			owned = nil
			if len(retained) > 0 {
				*retained[0] = nil
			}
		}
		want, ok := execution.AccessPermissions(input.Mode)
		if !ok {
			return fmt.Errorf("Unsupported workspace request; no local authority granted")
		}
		access := input.Mode
		if req.Access != "" {
			ceiling, e := executionPermissions(req.Access)
			if e != nil || !execution.PermissionsWithin(want, ceiling) {
				return fmt.Errorf("The task requests more workspace access than --workspace-access permits; no authority granted")
			}
			// A user's ceiling is not a request to grant extra privileges.
		}
		fmt.Fprintln(env.Stdout, "This task needs local workspace access.")
		if req.FolderCheck != nil {
			if err := req.FolderCheck.check(); err != nil {
				return settleForegroundRun(client, snapshot.RunID, true, err)
			}
		}
		reader, ok := env.Stdin.(*bufio.Reader)
		if !ok {
			reader = bufio.NewReader(env.Stdin)
		}
		leaseContext := runCtx
		if len(retained) > 0 {
			// Conversation-owned authority outlives an individual turn's watcher.
			// The outer conversation still cancels/closes it on exit or Stop.
			leaseContext = ctx
		}
		owned, err = openCLIWorkspace(leaseContext, client, req.Folder, access, reader, env.Stdout, env)
		if err != nil {
			return settleForegroundRun(client, snapshot.RunID, true, fmt.Errorf("Workspace access was not established: %w", err))
		}
		watch(owned)
		resource := owned.Resource()
		body := map[string]any{"input_id": input.ID, "execution": resource.Reference}
		var attached AgentRunSnapshot
		if err := client.JSON(runCtx, http.MethodPost, "/api/v2/jobs/"+url.PathEscape(snapshot.RunID)+"/input", body, &attached); err != nil {
			return fmt.Errorf("Workspace attachment was not confirmed; inspect saved job %s (no resubmission): %w", snapshot.RunID, err)
		}
		if attached.RunID != snapshot.RunID || !ValidAgentStatus(attached.Status) {
			return fmt.Errorf("Invalid workspace attachment acknowledgment")
		}
		fmt.Fprintf(env.Stdout, "Workspace: %s · %s\n", TerminalSafe(resource.Root), TerminalSafe(resource.Reference.Host))
		snapshot = &attached
	}
}
