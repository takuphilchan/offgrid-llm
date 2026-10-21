package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
)

const SessionHelp = `Saved conversations on the connected OffGrid service:
  offgrid session list
  offgrid session show NAME
  offgrid session create NAME --model MODEL
  offgrid session send NAME --profile ID --profile-revision N [--request-id ID] [--wait] MESSAGE
  offgrid session chat NAME --profile ID --profile-revision N
  offgrid session turn NAME
  offgrid session recover NAME REQUEST_ID
  offgrid session delete NAME [--work retain|stop]
  offgrid session export NAME [FILE]
  offgrid export-session NAME --format markdown|json|txt [--output FILE]

Set OFFGRID_SERVER_URL and OFFGRID_API_KEY for a remote or protected service.
There is no direct-file fallback. Export defaults to stdout; FILE must not exist.
`

func LoadServiceSession(ctx context.Context, client *serviceclient.Client, name string) (*sessions.Session, error) {
	if strings.TrimSpace(name) == "" {
		return nil, Usage("session name is required")
	}
	var session sessions.Session
	if err := client.JSON(ctx, http.MethodGet, "/v1/sessions/"+url.PathEscape(name), nil, &session); err != nil {
		return nil, err
	}
	if session.Name != name || session.CreatedAt.IsZero() || session.UpdatedAt.IsZero() {
		return nil, &serviceclient.Error{Code: "invalid_response", Message: "The service returned an incomplete or mismatched conversation."}
	}
	if session.Messages == nil {
		session.Messages = []sessions.Message{}
	}
	return &session, nil
}

func RunSessionService(ctx context.Context, client *serviceclient.Client, args []string, stdout io.Writer, jsonMode bool, renderStream func(io.Writer, io.Writer, io.Reader) error) error {
	return RunSessionServiceWithFiles(ctx, client, args, stdout, jsonMode, renderStream,
		func(path string) (SessionExportFile, error) {
			return os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		}, os.Remove)
}

func RunSessionServiceWithFiles(ctx context.Context, client *serviceclient.Client, args []string, stdout io.Writer, jsonMode bool, renderStream func(io.Writer, io.Writer, io.Reader) error, create func(string) (SessionExportFile, error), remove func(string) error) error {
	if len(args) == 0 {
		return Usage("session command required")
	}
	switch args[0] {
	case "send", "turn", "recover":
		return RunConversationService(ctx, client, args, stdout, jsonMode, renderStream)
	case "create":
		if len(args) != 4 || args[2] != "--model" || !ValidProfileID(args[1]) || args[3] == "" {
			return Usage("session create NAME --model MODEL")
		}
		var result sessions.Session
		if err := client.JSON(ctx, http.MethodPost, "/v1/sessions", map[string]string{"name": args[1], "model_id": args[3]}, &result); err != nil {
			return err
		}
		if result.Name != args[1] {
			return fmt.Errorf("The service returned a different conversation identity")
		}
		if jsonMode {
			return json.NewEncoder(stdout).Encode(result)
		}
		_, err := fmt.Fprintf(stdout, "Created conversation: %s\n", TerminalSafe(result.Name))
		return err
	case "list", "ls":
		if len(args) != 1 {
			return Usage("session list takes no arguments")
		}
		var response map[string]json.RawMessage
		if err := client.JSON(ctx, http.MethodGet, "/v1/sessions", nil, &response); err != nil {
			return err
		}
		data, ok := response["sessions"]
		if !ok {
			return &serviceclient.Error{Code: "invalid_response", Message: "The service returned no conversation inventory."}
		}
		var list []sessions.Session
		if err := json.Unmarshal(data, &list); err != nil {
			return &serviceclient.Error{Code: "invalid_response", Message: "The service returned an invalid conversation inventory."}
		}
		if list == nil {
			list = []sessions.Session{}
		}
		for _, session := range list {
			if session.Name == "" {
				return &serviceclient.Error{Code: "invalid_response", Message: "The service returned an invalid conversation identity."}
			}
		}
		if jsonMode {
			return json.NewEncoder(stdout).Encode(map[string]any{"sessions": list})
		}
		if len(list) == 0 {
			_, err := fmt.Fprintln(stdout, "No saved conversations.")
			return err
		}
		for _, session := range list {
			if _, err := fmt.Fprintf(stdout, "%s  Â·  %s  Â·  %d messages\n", TerminalSafe(session.Name), TerminalSafe(session.ModelID), len(session.Messages)); err != nil {
				return err
			}
		}
		return nil
	case "show", "view":
		if len(args) != 2 {
			return Usage("session show requires NAME")
		}
		session, err := LoadServiceSession(ctx, client, args[1])
		if err != nil {
			return err
		}
		if jsonMode {
			return json.NewEncoder(stdout).Encode(session)
		}
		_, err = fmt.Fprint(stdout, TerminalSafe(ExportSessionText(session)))
		return err
	case "delete", "del", "rm":
		if (len(args) != 2 && len(args) != 4) || strings.TrimSpace(args[1]) == "" {
			return Usage("session delete requires NAME [--work retain|stop]")
		}
		path := "/v1/sessions/" + url.PathEscape(args[1])
		if len(args) == 4 {
			if args[2] != "--work" || (args[3] != "retain" && args[3] != "stop") {
				return Usage("Use --work retain or --work stop")
			}
			path += "?work=" + args[3]
		}
		var result struct {
			Success *bool `json:"success"`
		}
		if err := client.JSON(ctx, http.MethodDelete, path, nil, &result); err != nil {
			return err
		}
		if result.Success == nil || !*result.Success {
			return &serviceclient.Error{Code: "invalid_response", Message: "The service did not confirm conversation deletion. Refresh its state before retrying."}
		}
		if jsonMode {
			return json.NewEncoder(stdout).Encode(map[string]any{"success": true, "name": args[1]})
		}
		_, err := fmt.Fprintf(stdout, "OK Deleted conversation: %s\n", TerminalSafe(args[1]))
		return err
	case "export":
		if len(args) < 2 || len(args) > 3 {
			return Usage("session export requires NAME [FILE]")
		}
		file := ""
		if len(args) == 3 {
			file = args[2]
		}
		return ExportServiceSessionWithFiles(ctx, client, args[1], "markdown", file, stdout, jsonMode, create, remove)
	default:
		return Usage("unknown session command %q", args[0])
	}
}

type SessionExportFile interface {
	io.Writer
	Sync() error
	Close() error
}

func ExportServiceSession(ctx context.Context, client *serviceclient.Client, name, format, filename string, stdout io.Writer, jsonMode bool) error {
	return ExportServiceSessionWithFiles(ctx, client, name, format, filename, stdout, jsonMode,
		func(path string) (SessionExportFile, error) {
			return os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		}, os.Remove)
}

func ExportServiceSessionWithFiles(ctx context.Context, client *serviceclient.Client, name, format, filename string, stdout io.Writer, jsonMode bool, create func(string) (SessionExportFile, error), remove func(string) error) error {
	switch format {
	case "markdown", "md", "json", "text", "txt":
	default:
		return Usage("unsupported export format %q", format)
	}
	session, err := LoadServiceSession(ctx, client, name)
	if err != nil {
		return err
	}
	var content string
	switch format {
	case "markdown", "md":
		content = ExportSessionMarkdown(session)
	case "text", "txt":
		content = ExportSessionText(session)
	case "json":
		data, err := json.MarshalIndent(session, "", "  ")
		if err != nil {
			return err
		}
		content = string(data)
	}
	if filename != "" {
		if err := ctx.Err(); err != nil {
			return err
		}
		if create == nil || remove == nil {
			return fmt.Errorf("session export file adapter unavailable")
		}
		file, err := create(filename)
		if err != nil {
			return fmt.Errorf("create export (existing files are not overwritten): %w", err)
		}
		_, err = io.WriteString(file, content)
		if err == nil {
			err = file.Sync()
		}
		if closeErr := file.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			_ = remove(filename)
			return err
		}
		if jsonMode {
			return json.NewEncoder(stdout).Encode(map[string]any{"success": true, "path": filename, "format": format})
		}
		_, err = fmt.Fprintf(stdout, "OK Exported conversation to %s\n", TerminalSafe(filename))
		return err
	}
	if jsonMode {
		return json.NewEncoder(stdout).Encode(map[string]string{"format": format, "content": content})
	}
	_, err = fmt.Fprintln(stdout, TerminalSafe(content))
	return err
}
