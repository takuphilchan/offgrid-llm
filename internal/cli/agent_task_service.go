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
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

type TaskSubmission struct {
	Prompt          string `json:"prompt"`
	Model           string `json:"model"`
	RequestID       string `json:"request_id"`
	Style           string `json:"style,omitempty"`
	MaxIterations   int    `json:"max_iterations,omitempty"`
	ProfileID       string `json:"profile_id,omitempty"`
	ProfileRevision int    `json:"profile_revision,omitempty"`
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
	if req.Prompt == "" || (req.Model == "" && req.ProfileID == "") || (req.ProfileID == "") != (req.ProfileRevision == 0) || len(req.RequestID) < 8 || len(req.RequestID) > 128 {
		return req, wait, Usage("agent run <task> (--model <installed model> | --profile ID --profile-revision N) [--request-id <id>] [--wait]")
	}
	return req, wait, nil
}

func SubmitTask(ctx context.Context, client *serviceclient.Client, req TaskSubmission) (*AgentRunSnapshot, error) {
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
