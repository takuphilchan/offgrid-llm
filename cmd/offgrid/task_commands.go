package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/takuphilchan/offgrid-llm/internal/output"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

type taskSubmission struct {
	Prompt        string `json:"prompt"`
	Model         string `json:"model"`
	RequestID     string `json:"request_id"`
	Style         string `json:"style,omitempty"`
	MaxIterations int    `json:"max_iterations,omitempty"`
}

func parseTaskSubmission(args []string) (taskSubmission, bool, error) {
	req := taskSubmission{RequestID: uuid.NewString()}
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
			return req, wait, usage("Missing value for %s", arg)
		}
		i++
		value := args[i]
		switch arg {
		case "--model":
			req.Model = value
		case "--request-id":
			req.RequestID = value
		case "--style":
			if value == "plan" {
				value = "plan-execute"
			}
			if value != "react" && value != "cot" && value != "plan-execute" {
				return req, wait, usage("Invalid task style")
			}
			req.Style = value
		case "--max-steps":
			n, err := strconv.Atoi(value)
			if err != nil || n < 1 || n > 50 {
				return req, wait, usage("Use 1 to 50 steps")
			}
			req.MaxIterations = n
		default:
			return req, wait, usage("Unknown task option %s", arg)
		}
	}
	req.Prompt = strings.TrimSpace(strings.Join(prompt, " "))
	if req.Prompt == "" || req.Model == "" || len(req.RequestID) < 8 || len(req.RequestID) > 128 {
		return req, wait, usage("agent run <task> --model <installed model> [--request-id <id>] [--wait]")
	}
	return req, wait, nil
}

func submitTask(ctx context.Context, client *serviceclient.Client, req taskSubmission) (*agentRunSnapshot, error) {
	data, _ := json.Marshal(req)
	response, err := client.Do(ctx, http.MethodPost, "/api/v2/jobs", "application/json", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	var snapshot agentRunSnapshot
	if err = serviceclient.DecodeContext(ctx, response.Body, &snapshot); err != nil {
		return nil, err
	}
	if snapshot.RunID == "" || !validAgentStatus(snapshot.Status) {
		return nil, fmt.Errorf("The service returned an invalid saved task")
	}
	return &snapshot, nil
}

func runTaskSubmit(ctx context.Context, args []string) error {
	req, wait, err := parseTaskSubmission(args)
	if err != nil {
		return err
	}
	client, err := newCommandClient()
	if err != nil {
		return err
	}
	if !output.JSONMode {
		fmt.Fprintf(os.Stderr, "Request: %s\n", req.RequestID)
	}
	snapshot, err := submitTask(ctx, client, req)
	if err != nil {
		return fmt.Errorf("%w (retry this submission with --request-id %s)", err, req.RequestID)
	}
	if output.JSONMode {
		return json.NewEncoder(os.Stdout).Encode(snapshot)
	}
	fmt.Fprintf(os.Stdout, "Run: %s\n", terminalSafe(snapshot.RunID))
	renderAgentSnapshot(os.Stdout, *snapshot)
	if wait && (snapshot.Status == "running" || snapshot.Status == "pending" || snapshot.Status == "waiting_for_children") {
		response, err := client.Do(ctx, http.MethodGet, "/api/v2/jobs/"+snapshot.RunID+"/events", "", nil)
		if err != nil {
			return err
		}
		defer response.Body.Close()
		return renderAgentStreamTo(os.Stdout, os.Stderr, response.Body)
	}
	return nil
}

func openComputerWorkspace(task string) error {
	link := "offgrid://computer"
	if task != "" {
		if len(task) != 36 || !strings.HasPrefix(task, "run-") {
			return usage("Invalid task ID")
		}
		for _, c := range task[4:] {
			if !strings.ContainsRune("0123456789abcdef", c) {
				return usage("Invalid task ID")
			}
		}
		link += "?task=" + task
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", link)
	case "darwin":
		cmd = exec.Command("open", link)
	default:
		cmd = exec.Command("xdg-open", link)
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("open the matching OffGrid Desktop application to grant local access: %w", err)
	}
	return nil
}
