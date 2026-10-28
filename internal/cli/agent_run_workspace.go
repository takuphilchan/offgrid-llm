package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

func prepareRunWorkspace(ctx context.Context, client *serviceclient.Client, req *TaskSubmission, env Environment) (ExecutionLease, error) {
	terminal := env.Execution.Interactive != nil && env.Execution.Interactive()
	if terminal && !env.Options.JSON && !req.Detach && req.ResponseProfile == "" {
		identity, err := client.Identity(ctx)
		if err != nil {
			return nil, err
		}
		if identity.Supports("agent-response-profiles-v1") {
			req.ResponseProfile = "interactive"
		}
	}
	automatic := terminal && !env.Options.JSON && !req.Detach && req.Workspace == ""
	if (automatic || req.Folder != "") && req.Workspace == "" && !env.Options.JSON && !req.Detach && env.Execution.OnDemand != nil {
		ready, err := env.Execution.OnDemand(ctx, client)
		if err != nil {
			return nil, err
		}
		if ready {
			if err := captureLaunchFolder(req, env); err != nil {
				return nil, err
			}
			req.ExecutionRequestVersion = 1
			return nil, nil // No executor, file probe or consent for conversation.
		}
	}
	if req.Folder == "" && !automatic {
		if req.Access != "" {
			return nil, Usage("Use --workspace-access with --workspace-folder or an interactive terminal run")
		}
		return nil, nil
	}
	if env.Options.JSON || req.Detach {
		return nil, Usage("Approve a workspace session first and select --workspace SESSION for JSON or detached work")
	}
	if automatic && req.Folder == "" {
		if env.Execution.Available == nil {
			return nil, fmt.Errorf("Local execution is unavailable. Install the matching CLI/executor and enable the service execution preview; no task was submitted in the service directory")
		}
		available, err := env.Execution.Available(ctx, client)
		if err != nil || !available {
			return nil, fmt.Errorf("Local workspace unavailable; update the matching CLI/executor and service before running local file work (no task submitted): %w", errors.Join(err, errors.New("native execution not ready")))
		}
	}
	stopInput := context.AfterFunc(ctx, func() {
		if input, ok := env.Stdin.(interface{ Close() error }); ok {
			_ = input.Close()
		}
	})
	defer stopInput()
	lease, err := openCLIWorkspace(ctx, client, req.Folder, req.Access, bufio.NewReader(env.Stdin), env.Stdout, env)
	if err != nil {
		return nil, fmt.Errorf("Local workspace was not approved or could not start; no task was submitted: %w", err)
	}
	ref := lease.Resource().Reference
	if err = ref.Validate(); err != nil {
		_ = lease.Close()
		return nil, err
	}
	req.Execution = &ref
	fmt.Fprintf(env.Stdout, "Workspace: %s · %s\n", TerminalSafe(lease.Resource().Root), TerminalSafe(ref.Host))
	return lease, nil
}

type launchFolderCheck struct{ check func() error }

func captureLaunchFolder(req *TaskSubmission, env Environment) error {
	var err error
	if req.Folder == "" && env.Execution.CurrentFolder != nil {
		req.Folder, err = env.Execution.CurrentFolder()
		if err != nil {
			return err
		}
	}
	if env.Execution.CaptureFolder != nil {
		var check func() error
		req.Folder, check, err = env.Execution.CaptureFolder(req.Folder)
		if check != nil {
			req.FolderCheck = &launchFolderCheck{check: check}
		}
	}
	return err
}

func foregroundRunLive(status string) bool {
	return status == "running" || status == "pending" || status == "waiting_for_children"
}

// Never retry the submission or tool call. Only observe/stop the already known
// canonical root, using a bounded context independent of an interrupted stream.
func settleForegroundRun(client *serviceclient.Client, id string, owned bool, cause error) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	snapshot, err := AgentControlService(ctx, client, []string{"status", id})
	if err != nil || snapshot != nil && (foregroundRunLive(snapshot.Status) || owned && (snapshot.Status == "waiting_for_approval" || snapshot.Status == "waiting_for_input")) {
		_, stopErr := AgentControlService(ctx, client, []string{"cancel", id})
		if stopErr != nil {
			return errors.Join(cause, fmt.Errorf("Stop is not confirmed for %s; inspect this saved run before retrying: %w", TerminalSafe(id), stopErr))
		}
		if cause == nil {
			return fmt.Errorf("Foreground work ended before completion; the same run was stopped. Inspect %s before continuing", TerminalSafe(id))
		}
		return cause
	}
	if cause != nil {
		return cause
	}
	if err != nil {
		return err
	}
	if snapshot.Status == "failed" || snapshot.Status == "cancelled" || snapshot.Status == "uncertain" || snapshot.Status == "interrupted" {
		return fmt.Errorf("Run %s ended as %s; inspect saved work before continuing", TerminalSafe(id), TerminalSafe(snapshot.Status))
	}
	return nil
}

func streamForegroundRun(ctx context.Context, client *serviceclient.Client, snapshot *AgentRunSnapshot, owned bool, env Environment) (result error) {
	defer func() { result = settleForegroundRun(client, snapshot.RunID, owned, result) }()
	if env.Service.RenderStream == nil {
		return fmt.Errorf("agent stream renderer unavailable")
	}
	response, err := client.Events(ctx, "/api/v2/jobs/"+url.PathEscape(snapshot.RunID)+"/events")
	if err != nil {
		return err
	}
	defer response.Body.Close()
	return env.Service.RenderStream(env.Stdout, env.Stderr, response.Body)
}
