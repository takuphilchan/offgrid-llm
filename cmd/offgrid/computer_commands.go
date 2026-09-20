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
	if len(args) == 0 {
		return usage("computer status | targets | pair | stop | check <model> | run <session> <model> [--expect <page-text>] <task>")
	}
	path, method := "", http.MethodGet
	var body []byte
	switch args[0] {
	case "check":
		if len(args) != 2 {
			return usage("computer check <model>")
		}
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
		}
		if err := json.Unmarshal(value, &check); err != nil {
			return err
		}
		if !check.Passed {
			return fmt.Errorf("%s", check.Message)
		}
	}
	return nil
}
