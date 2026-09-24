package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
	"net/http"
	"os"
	"strings"
)

func runComputerCommand(ctx context.Context, args []string) error {
	if len(args) > 0 {
		if args[0] == "setup" {
			if len(args) > 2 {
				return usage("computer setup [RUN_ID]")
			}
			id := ""
			if len(args) == 2 {
				id = args[1]
			}
			return openComputerWorkspace(id)
		}
		if args[0] == "pause" || args[0] == "resume" || args[0] == "takeover" || (args[0] == "stop" && len(args) == 2) {
			command := append([]string(nil), args...)
			if command[0] == "stop" {
				command[0] = "cancel"
			}
			return runAgentControl(ctx, command)
		}
		if args[0] == "run" {
			for _, arg := range args[1:] {
				if arg == "--model" {
					return runTaskSubmit(ctx, args[1:])
				}
			}
		}
	}
	if len(args) == 0 {
		return usage("computer setup [RUN_ID] | status | targets | run <task> --model <model> [--wait] | pause|resume|takeover|stop RUN_ID | check <model>")
	}
	path, method := "", http.MethodGet
	var body []byte
	requireVision := false
	switch args[0] {
	case "check":
		if len(args) != 2 && (len(args) != 3 || args[2] != "--require-vision") {
			return usage("computer check <model> [--require-vision]")
		}
		requireVision = len(args) == 3
		path, method = "/api/v2/computer/model-check", http.MethodPost
		body, _ = json.Marshal(map[string]string{"model": args[1]})
	case "status":
		path = "/api/v2/computer/capabilities"
	case "targets":
		path = "/api/v2/computer/sessions"
	case "pair":
		path = "/api/v2/computer/pairing"
		method = http.MethodPost
		body = []byte(`{}`)
	case "stop":
		path = "/api/v2/computer/stop"
		method = http.MethodPost
		body = []byte(`{}`)
	case "run":
		if len(args) < 4 {
			return usage("computer run <session> <model> [--expect <exact-page-text>] <task>")
		}
		expected, promptArgs := "", args[3:]
		if args[3] == "--expect" {
			if len(args) < 6 || strings.TrimSpace(args[4]) == "" || len(args[4]) > 1000 {
				return usage("computer run <session> <model> [--expect <exact-page-text>] <task>")
			}
			expected, promptArgs = args[4], args[5:]
		}
		if strings.TrimSpace(strings.Join(promptArgs, " ")) == "" {
			return usage("A task is required")
		}
		path = "/v1/agents/run"
		method = http.MethodPost
		body, _ = json.Marshal(map[string]any{"computer_session": args[1], "model": args[2], "computer_expected_text": expected, "prompt": strings.Join(promptArgs, " "), "async": true})
	default:
		return usage("Unknown computer command. Use status, targets, pair, stop, or run. Browser companion installation is documented in computer/README.md.")
	}
	if args[0] != "run" && args[0] != "check" && len(args) != 1 {
		return usage("Unexpected computer command arguments")
	}
	client, err := newCommandClient()
	if err != nil {
		return err
	}
	response, err := client.Do(ctx, method, path, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer response.Body.Close()
	var value json.RawMessage
	if err = serviceclient.DecodeContext(ctx, response.Body, &value); err != nil {
		return err
	}
	// Structured output is usable from pipes; no progress or credentials in stderr.
	var formatted bytes.Buffer
	if err = json.Indent(&formatted, value, "", "  "); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, formatted.String())
	if args[0] == "check" {
		var check struct {
			Passed  bool   `json:"passed"`
			Message string `json:"message"`
			Vision  *struct {
				Installed bool   `json:"installed"`
				Passed    bool   `json:"passed"`
				Message   string `json:"message"`
			} `json:"vision"`
		}
		if err := json.Unmarshal(value, &check); err != nil {
			return err
		}
		if !check.Passed {
			return fmt.Errorf("%s", check.Message)
		}
		if requireVision && (check.Vision == nil || !check.Vision.Installed || !check.Vision.Passed) {
			if check.Vision != nil && check.Vision.Message != "" {
				return fmt.Errorf("%s", check.Vision.Message)
			}
			return fmt.Errorf("local vision is not installed or did not pass the current runtime check")
		}
	}
	return nil
}
