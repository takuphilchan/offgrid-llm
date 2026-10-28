package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

type TaskSubmission struct {
	ResponseProfile         string               `json:"response_profile,omitempty"`
	ExecutionRequestVersion int                  `json:"execution_request_version,omitempty"`
	Execution               *execution.Reference `json:"execution,omitempty"`
	Workspace               string               `json:"-"`
	Folder                  string               `json:"-"`
	FolderCheck             *launchFolderCheck   `json:"-"`
	Access                  string               `json:"-"`
	Detach                  bool                 `json:"-"`
	Prompt                  string               `json:"prompt"`
	Model                   string               `json:"model"`
	RequestID               string               `json:"request_id"`
	Style                   string               `json:"style,omitempty"`
	MaxIterations           int                  `json:"max_iterations,omitempty"`
	ProfileID               string               `json:"profile_id,omitempty"`
	ProfileRevision         int                  `json:"profile_revision,omitempty"`
}

func ParseTaskSubmission(args []string) (TaskSubmission, bool, error) {
	req := TaskSubmission{RequestID: uuid.NewString()}
	wait := false
	prompt := []string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			prompt = append(prompt, args[i+1:]...)
			break
		}
		if arg == "--wait" {
			wait = true
			continue
		}
		if arg == "--detach" {
			req.Detach = true
			continue
		}
		if !strings.HasPrefix(arg, "--") {
			prompt = append(prompt, arg)
			continue
		}
		if i+1 >= len(args) {
			return req, wait, Usage("Missing value for %s", arg)
		}
		i++
		value := args[i]
		switch arg {
		case "--response-profile":
			if req.ResponseProfile != "" || value != "interactive" && value != "extended" {
				return req, wait, Usage("Choose one --response-profile interactive|extended")
			}
			req.ResponseProfile = value
		case "--workspace-folder":
			if req.Folder != "" || strings.TrimSpace(value) == "" {
				return req, wait, Usage("Select one explicit local folder")
			}
			req.Folder = value
		case "--workspace-access":
			if _, err := executionPermissions(value); err != nil {
				return req, wait, err
			}
			req.Access = value
		case "--workspace":
			if req.Workspace != "" || !ValidProfileID(value) {
				return req, wait, Usage("Select one valid approved execution session with --workspace")
			}
			req.Workspace = value
		case "--model":
			req.Model = value
		case "--profile":
			if !ValidProfileID(value) {
				return req, wait, Usage("Invalid profile ID")
			}
			req.ProfileID = value
		case "--profile-revision":
			n, err := strconv.Atoi(value)
			if err != nil || n < 1 {
				return req, wait, Usage("A positive profile revision is required")
			}
			req.ProfileRevision = n
		case "--request-id":
			req.RequestID = value
		case "--style":
			if value == "plan" {
				value = "plan-execute"
			}
			if value != "react" && value != "cot" && value != "plan-execute" {
				return req, wait, Usage("Invalid task style")
			}
			req.Style = value
		case "--max-steps":
			n, err := strconv.Atoi(value)
			if err != nil || n < 1 || n > 50 {
				return req, wait, Usage("Use 1 to 50 steps")
			}
			req.MaxIterations = n
		default:
			return req, wait, Usage("Unknown task option %s", arg)
		}
	}
	req.Prompt = strings.TrimSpace(strings.Join(prompt, " "))
	if req.Detach && (wait || req.Folder != "") || req.Folder != "" && req.Workspace != "" || req.Access != "" && req.Workspace != "" {
		return req, wait, Usage("Choose one local folder or an existing workspace. --detach cannot own a local folder or combine with --wait")
	}
	if req.Prompt == "" || (req.Model == "" && req.ProfileID == "") || (req.ProfileID == "") != (req.ProfileRevision == 0) || len(req.RequestID) < 8 || len(req.RequestID) > 128 {
		return req, wait, Usage("agent run <task> (--model <installed model> | --profile ID --profile-revision N) [--request-id <id>] [--wait]")
	}
	return req, wait, nil
}

func SubmitTask(ctx context.Context, client *serviceclient.Client, req TaskSubmission) (*AgentRunSnapshot, error) {
	if req.ResponseProfile != "" {
		if err := client.RequireCapability(ctx, "agent-response-profiles-v1"); err != nil {
			return nil, err
		}
	}
	if req.Workspace != "" {
		ref, _, err := resolveExecutionWorkspace(ctx, client, req.Workspace)
		if err != nil {
			return nil, err
		}
		req.Execution = &ref
	}
	if req.ProfileID != "" {
		if err := client.RequireCapability(ctx, "agent-profiles-v1"); err != nil {
			return nil, err
		}
	}
	data, _ := json.Marshal(req)
	response, err := client.Do(ctx, http.MethodPost, "/api/v2/jobs", "application/json", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	var snapshot AgentRunSnapshot
	if err = serviceclient.DecodeContext(ctx, response.Body, &snapshot); err != nil {
		return nil, err
	}
	if snapshot.RunID == "" || !ValidAgentStatus(snapshot.Status) {
		return nil, fmt.Errorf("The service returned an invalid saved task")
	}
	return &snapshot, nil
}
