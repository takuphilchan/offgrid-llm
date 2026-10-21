package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

const agentProfilesHelp = `Reusable agents on the connected service:
  offgrid agent profiles list
  offgrid agent profiles show ID
  offgrid agent profiles effective ID
  offgrid agent profiles resources
  offgrid agent profiles create FILE
  offgrid agent profiles update ID REVISION FILE
  offgrid agent profiles archive ID REVISION

FILE contains ProfileSettings JSON; use - to read stdin. No credentials belong
in instructions. Updates/archive require the revision you reviewed; conflicts
never overwrite changes. These commands do not execute tools or download models.
Use --json for structured output.
`

func ValidProfileID(id string) bool {
	return strings.TrimSpace(id) != "" && id != "." && id != ".." && !strings.ContainsAny(id, "/\\?#\x00\r\n")
}

func ReadProfileSettings(name string, stdin io.Reader) (agents.ProfileSettings, error) {
	return ReadProfileSettingsWithOpen(name, stdin, func(path string) (io.ReadCloser, error) { return os.Open(path) })
}

func ReadProfileSettingsWithOpen(name string, stdin io.Reader, open func(string) (io.ReadCloser, error)) (agents.ProfileSettings, error) {
	var settings agents.ProfileSettings
	input := stdin
	if name != "-" {
		if open == nil {
			return settings, fmt.Errorf("profile file reader unavailable")
		}
		file, err := open(name)
		if err != nil {
			return settings, fmt.Errorf("read profile settings: %w", err)
		}
		defer file.Close()
		input = file
	}
	data, err := io.ReadAll(io.LimitReader(input, 65537))
	if err != nil || len(data) > 65536 {
		return settings, Usage("Profile settings must be JSON of at most 64 KiB")
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&settings) != nil || decoder.Decode(new(any)) != io.EOF {
		return settings, Usage("Expected one ProfileSettings JSON object, without unknown fields")
	}
	if settings.Name == "" || settings.Model == "" || settings.MaxIterations < 1 || settings.MaxIterations > 50 {
		return settings, Usage("Profile settings require name, installed model and max_iterations between 1 and 50")
	}
	return settings, nil
}

func RunProfileService(ctx context.Context, client *serviceclient.Client, args []string, stdin io.Reader, stdout io.Writer, jsonMode bool) error {
	return RunProfileServiceWithOpen(ctx, client, args, stdin, stdout, jsonMode, func(path string) (io.ReadCloser, error) { return os.Open(path) })
}

func RunProfileServiceWithOpen(ctx context.Context, client *serviceclient.Client, args []string, stdin io.Reader, stdout io.Writer, jsonMode bool, open func(string) (io.ReadCloser, error)) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" {
		if jsonMode {
			return json.NewEncoder(stdout).Encode(map[string]string{"help": agentProfilesHelp})
		}
		_, err := fmt.Fprint(stdout, agentProfilesHelp)
		return err
	}
	method, path := http.MethodGet, "/api/v2/agent-profiles"
	var body any
	var requestedID string
	switch args[0] {
	case "list", "resources":
		if len(args) != 1 {
			return Usage("agent profiles %s takes no arguments", args[0])
		}
		if args[0] == "resources" {
			path += "/resources"
		}
	case "show", "effective":
		if len(args) != 2 || !ValidProfileID(args[1]) {
			return Usage("agent profiles %s ID", args[0])
		}
		requestedID = args[1]
		path += "/" + url.PathEscape(requestedID)
		if args[0] == "effective" {
			path += "/effective"
		}
	case "create", "update", "archive":
		var revision int
		file := ""
		if args[0] == "create" {
			if len(args) != 2 {
				return Usage("agent profiles create FILE")
			}
			file = args[1]
			method = http.MethodPost
		} else {
			expected := 3
			if args[0] == "update" {
				expected = 4
			}
			if len(args) != expected || !ValidProfileID(args[1]) {
				return Usage("agent profiles %s ID REVISION [FILE]", args[0])
			}
			var err error
			revision, err = strconv.Atoi(args[2])
			if err != nil || revision < 1 {
				return Usage("A positive reviewed revision is required")
			}
			requestedID = args[1]
			path += "/" + url.PathEscape(requestedID)
			method = http.MethodPut
			if args[0] == "archive" {
				path += "/archive"
				method = http.MethodPost
			} else {
				file = args[3]
			}
		}
		payload := map[string]any{"expected_revision": revision}
		if file != "" {
			settings, err := ReadProfileSettingsWithOpen(file, stdin, open)
			if err != nil {
				return err
			}
			payload["settings"] = settings
		}
		body = payload
	default:
		return Usage("Unknown agent profiles command %q", args[0])
	}
	if err := client.RequireCapability(ctx, "agent-profiles-v1"); err != nil {
		return err
	}
	var result json.RawMessage
	if err := client.JSON(ctx, method, path, body, &result); err != nil {
		return err
	}
	if jsonMode || args[0] == "effective" || args[0] == "resources" {
		// JSON encoding escapes terminal controls even in remote metadata.
		return json.NewEncoder(stdout).Encode(result)
	}
	if args[0] == "list" {
		var inventory struct {
			Profiles []agents.AgentProfile `json:"profiles"`
		}
		if json.Unmarshal(result, &inventory) != nil {
			return fmt.Errorf("Invalid profile inventory")
		}
		for _, p := range inventory.Profiles {
			fmt.Fprintf(stdout, "%s  revision %d  %s  (%s)\n", TerminalSafe(p.ID), p.Revision, TerminalSafe(p.Settings.Name), TerminalSafe(p.Settings.Model))
		}
		return nil
	}
	if args[0] == "archive" {
		_, err := fmt.Fprintf(stdout, "Archived: %s. Existing jobs and revisions are retained.\n", TerminalSafe(requestedID))
		return err
	}
	var p agents.AgentProfile
	if json.Unmarshal(result, &p) != nil || p.ID == "" || p.Revision < 1 || requestedID != "" && p.ID != requestedID {
		return fmt.Errorf("Invalid profile response; inspect saved state before retrying")
	}
	_, err := fmt.Fprintf(stdout, "%s  revision %d\n%s\nModel: %s\n%s\n", TerminalSafe(p.ID), p.Revision, TerminalSafe(p.Settings.Name), TerminalSafe(p.Settings.Model), TerminalSafe(p.Settings.Instructions))
	return err
}
