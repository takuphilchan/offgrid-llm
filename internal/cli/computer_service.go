package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

// ComputerDependencies supplies the reviewed host callbacks and authenticated
// service client for one invocation. No callback is inferred from model text.
type ComputerDependencies struct {
	Setup   func(context.Context, string) error
	Control func(context.Context, []string) error
	Submit  func(context.Context, []string) error
	Client  func() (*serviceclient.Client, error)
	Stdout  io.Writer
}

func computerCommand(ctx context.Context, args []string, env Environment) error {
	deps := env.Computer
	deps.Stdout = env.Stdout
	return RunComputer(ctx, args, deps)
}

// RunComputer chooses between reviewed host adapters and the authenticated
// service operation. An absent adapter fails closed rather than opening a
// browser or dispatching a task from a different execution path.
func RunComputer(ctx context.Context, args []string, deps ComputerDependencies) error {
	if len(args) > 0 {
		switch args[0] {
		case "setup":
			if len(args) > 2 {
				return Usage("computer setup [RUN_ID]")
			}
			if deps.Setup == nil {
				return fmt.Errorf("computer setup adapter unavailable")
			}
			id := ""
			if len(args) == 2 {
				id = args[1]
			}
			return deps.Setup(ctx, id)
		case "pause", "resume", "takeover":
			if deps.Control == nil {
				return fmt.Errorf("computer control adapter unavailable")
			}
			return deps.Control(ctx, append([]string(nil), args...))
		case "stop":
			if len(args) == 2 {
				if deps.Control == nil {
					return fmt.Errorf("computer control adapter unavailable")
				}
				return deps.Control(ctx, []string{"cancel", args[1]})
			}
		case "run":
			for _, arg := range args[1:] {
				if arg == "--model" {
					if deps.Submit == nil {
						return fmt.Errorf("computer task adapter unavailable")
					}
					return deps.Submit(ctx, append([]string(nil), args[1:]...))
				}
			}
		}
	}
	return RunComputerService(ctx, args, deps.Client, deps.Stdout)
}

// RunComputerService handles the service-backed computer subcommands.
func RunComputerService(ctx context.Context, args []string, makeClient func() (*serviceclient.Client, error), stdout io.Writer) error {
	if len(args) == 0 {
		return Usage("computer setup [RUN_ID] | status | targets | run <task> --model <model> [--wait] | pause|resume|takeover|stop RUN_ID | check <model>")
	}
	path, method := "", http.MethodGet
	var body []byte
	requireVision := false
	switch args[0] {
	case "check":
		if len(args) != 2 && (len(args) != 3 || args[2] != "--require-vision") {
			return Usage("computer check <model> [--require-vision]")
		}
		requireVision = len(args) == 3
		path, method = "/api/v2/computer/model-check", http.MethodPost
		body, _ = json.Marshal(map[string]string{"model": args[1]})
	case "status":
		path = "/api/v2/computer/capabilities"
	case "targets":
		path = "/api/v2/computer/sessions"
	case "pair":
		path, method, body = "/api/v2/computer/pairing", http.MethodPost, []byte(`{}`)
	case "stop":
		path, method, body = "/api/v2/computer/stop", http.MethodPost, []byte(`{}`)
	case "run":
		if len(args) < 4 {
			return Usage("computer run <session> <model> [--expect <exact-page-text>] <task>")
		}
		expected, promptArgs := "", args[3:]
		if args[3] == "--expect" {
			if len(args) < 6 || strings.TrimSpace(args[4]) == "" || len(args[4]) > 1000 {
				return Usage("computer run <session> <model> [--expect <exact-page-text>] <task>")
			}
			expected, promptArgs = args[4], args[5:]
		}
		if strings.TrimSpace(strings.Join(promptArgs, " ")) == "" {
			return Usage("A task is required")
		}
		path, method = "/v1/agents/run", http.MethodPost
		body, _ = json.Marshal(map[string]any{"computer_session": args[1], "model": args[2], "computer_expected_text": expected, "prompt": strings.Join(promptArgs, " "), "async": true})
	default:
		return Usage("Unknown computer command. Use status, targets, pair, stop, or run. Browser companion installation is documented in apps/computer/README.md.")
	}
	if args[0] != "run" && args[0] != "check" && len(args) != 1 {
		return Usage("Unexpected computer command arguments")
	}
	if makeClient == nil {
		return fmt.Errorf("computer service client unavailable")
	}
	client, err := makeClient()
	if err != nil {
		return err
	}
	response, err := client.Do(ctx, method, path, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer response.Body.Close()
	var value json.RawMessage
	if err := serviceclient.DecodeContext(ctx, response.Body, &value); err != nil {
		return err
	}
	var formatted bytes.Buffer
	if err := json.Indent(&formatted, value, "", "  "); err != nil {
		return err
	}
	_, _ = fmt.Fprintln(stdout, formatted.String())
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
