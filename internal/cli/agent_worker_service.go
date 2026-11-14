package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Noninteractive controls accept the complete shared envelope. Caller-owned
// request IDs and revisions survive retries from another process unchanged.
func agentWorkerService(ctx context.Context, args []string, env Environment) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" {
		usage := "agent worker show ID | messages ID [AFTER] | command < envelope.json"
		if env.Options.JSON {
			return json.NewEncoder(env.Stdout).Encode(map[string]string{"usage": usage})
		}
		_, err := fmt.Fprintln(env.Stdout, usage)
		return err
	}
	makeClient := env.Service.ControlClient
	if makeClient == nil {
		makeClient = env.Service.Client
	}
	if makeClient == nil {
		return fmt.Errorf("authenticated service client unavailable")
	}
	client, err := makeClient()
	if err != nil {
		return err
	}
	var result any
	switch args[0] {
	case "show":
		if len(args) != 2 {
			return Usage("agent worker show ID")
		}
		result, err = client.Worker(ctx, args[1])
	case "messages":
		if len(args) < 2 || len(args) > 3 {
			return Usage("agent worker messages ID [AFTER]")
		}
		after := int64(0)
		if len(args) == 3 {
			after, err = strconv.ParseInt(args[2], 10, 64)
			if err != nil || after < 0 {
				return Usage("AFTER must be a nonnegative cursor")
			}
		}
		result, err = client.WorkerMessages(ctx, args[1], after, 100)
	case "command":
		if len(args) != 1 || env.Stdin == nil {
			return Usage("agent worker command < envelope.json")
		}
		var command api.WorkerCommand
		data, readErr := io.ReadAll(io.LimitReader(env.Stdin, (64<<10)+1))
		if readErr != nil {
			return readErr
		}
		if len(data) > 64<<10 {
			return Usage("worker command exceeds 64 KiB")
		}
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if err = decoder.Decode(&command); err != nil {
			return Usage("invalid worker command: %v", err)
		}
		var extra any
		if err = decoder.Decode(&extra); err != io.EOF {
			return Usage("expected exactly one worker command")
		}
		if command.RequestID == "" || command.TargetID == "" || command.ExpectedRevision < 1 {
			return Usage("worker command requires request_id, target_id and expected_revision")
		}
		result, err = client.WorkerCommand(ctx, command)
	default:
		return Usage("agent worker show ID | messages ID [AFTER] | command < envelope.json")
	}
	if err != nil {
		return err
	}
	if env.Options.JSON {
		return json.NewEncoder(env.Stdout).Encode(result)
	}
	return printWorkerJSON(env.Stdout, result)
}
