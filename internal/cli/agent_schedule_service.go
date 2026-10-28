package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/internal/jsonwire"
)

const agentSchedulesHelp = `Owned calendar schedules on the connected service:
  offgrid agent schedules list
  offgrid agent schedules show ID
  offgrid agent schedules history ID
  offgrid agent schedules create FILE
  offgrid agent schedules update ID REVISION FILE
  offgrid agent schedules disable ID REVISION

FILE is JSON with settings (name, prompt, enabled, calendar: zone, hour, minute,
weekdays 0=Sunday..6=Saturday), profile_id, profile_revision, optional execution
reference and max_iterations. Use - for stdin. Review the exact profile, model,
folder, permissions and budget before saving. Updates require the reviewed
revision; re-enable requires a complete update and live resource validation.
Only future slots run while the service is awake. Missed or uncertain work is
never replayed. Inspect/history/disable do not start a job or mint host access.
Results are JSON, including escaped terminal controls.
`

type scheduleInput struct {
	Settings         agents.ScheduleSettings `json:"settings"`
	ProfileID        string                  `json:"profile_id"`
	ProfileRevision  int                     `json:"profile_revision"`
	Execution        *execution.Reference    `json:"execution,omitempty"`
	MaxIterations    int                     `json:"max_iterations,omitempty"`
	ExpectedRevision int                     `json:"expected_revision"`
}

func agentScheduleCommand(ctx context.Context, args []string, env Environment) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" {
		if env.Options.JSON {
			return json.NewEncoder(env.Stdout).Encode(map[string]string{"help": agentSchedulesHelp})
		}
		_, err := fmt.Fprint(env.Stdout, agentSchedulesHelp)
		return err
	}
	client, err := serviceClient(env)
	if err != nil {
		return err
	}
	if err = client.RequireCapability(ctx, "agent-schedules-v1"); err != nil {
		return err
	}
	method, path := http.MethodGet, "/api/v2/agent-schedules"
	var body any
	switch args[0] {
	case "list":
		if len(args) != 1 {
			return Usage("agent schedules list")
		}
	case "show", "history":
		if len(args) != 2 || !ValidProfileID(args[1]) {
			return Usage("agent schedules %s ID", args[0])
		}
		path += "/" + url.PathEscape(args[1])
		if args[0] == "history" {
			path += "/occurrences"
		}
	case "disable":
		if len(args) != 3 || !ValidProfileID(args[1]) {
			return Usage("agent schedules disable ID REVISION")
		}
		revision, e := contextRevision(args[2], false)
		if e != nil {
			return e
		}
		path += "/" + url.PathEscape(args[1]) + "/disable"
		method = http.MethodPost
		body = map[string]int{"expected_revision": revision}
	case "create", "update":
		name := ""
		revision := 0
		if args[0] == "create" {
			if len(args) != 2 {
				return Usage("agent schedules create FILE")
			}
			name = args[1]
			method = http.MethodPost
		} else {
			if len(args) != 4 || !ValidProfileID(args[1]) {
				return Usage("agent schedules update ID REVISION FILE")
			}
			revision, err = contextRevision(args[2], false)
			if err != nil {
				return err
			}
			path += "/" + url.PathEscape(args[1])
			name = args[3]
			method = http.MethodPut
		}
		input := env.Stdin
		if name != "-" {
			if env.Service.OpenProfileFile == nil {
				return fmt.Errorf("schedule file reader unavailable")
			}
			file, e := env.Service.OpenProfileFile(name)
			if e != nil {
				return e
			}
			defer file.Close()
			input = file
		}
		if input == nil {
			return Usage("Provide schedule JSON on stdin")
		}
		data, e := io.ReadAll(io.LimitReader(input, 65537))
		if e != nil {
			return e
		}
		if len(data) > 65536 || jsonwire.Validate(data) != nil {
			return Usage("Schedule must be one JSON object of at most 64 KiB")
		}
		var write scheduleInput
		decoder := json.NewDecoder(strings.NewReader(string(data)))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&write) != nil || decoder.Decode(new(any)) != io.EOF || write.ExpectedRevision != 0 || !ValidProfileID(write.ProfileID) || write.ProfileRevision < 1 {
			return Usage("Provide a schedule policy and exact profile revision, without authority or expected_revision fields; update revision belongs on the command line")
		}
		if _, e = write.Settings.Calendar.Canonical(); e != nil {
			return e
		}
		write.ExpectedRevision = revision
		body = write
	default:
		return Usage("Unknown agent schedules command %q", args[0])
	}
	var result json.RawMessage
	if err = client.JSON(ctx, method, path, body, &result); err != nil {
		return err
	}
	return json.NewEncoder(env.Stdout).Encode(result)
}
