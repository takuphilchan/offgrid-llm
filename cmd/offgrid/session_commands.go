package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/output"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
)

const sessionHelp = `Saved conversations on the connected OffGrid service:
  offgrid session list
  offgrid session show NAME
  offgrid session delete NAME
  offgrid session export NAME [FILE]
  offgrid export-session NAME --format markdown|json|txt [--output FILE]

Set OFFGRID_SERVER_URL and OFFGRID_API_KEY for a remote or protected service.
There is no direct-file fallback. Export defaults to stdout; FILE must not exist.
`

func runSessionCommand(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		if output.JSONMode {
			return json.NewEncoder(os.Stdout).Encode(map[string]string{"help": sessionHelp})
		}
		_, err := fmt.Fprint(os.Stdout, sessionHelp)
		return err
	}
	client, err := newCommandClient()
	if err != nil {
		return err
	}
	return sessionCommand(ctx, client, args, os.Stdout, output.JSONMode)
}

func loadServiceSession(ctx context.Context, client *serviceclient.Client, name string) (*sessions.Session, error) {
	if strings.TrimSpace(name) == "" {
		return nil, usage("session name is required")
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

func sessionCommand(ctx context.Context, client *serviceclient.Client, args []string, stdout io.Writer, jsonMode bool) error {
	if len(args) == 0 {
		return usage("session command required")
	}
	switch args[0] {
	case "list", "ls":
		if len(args) != 1 {
			return usage("session list takes no arguments")
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
			if _, err := fmt.Fprintf(stdout, "%s  ·  %s  ·  %d messages\n", terminalSafe(session.Name), terminalSafe(session.ModelID), len(session.Messages)); err != nil {
				return err
			}
		}
		return nil
	case "show", "view":
		if len(args) != 2 {
			return usage("session show requires NAME")
		}
		session, err := loadServiceSession(ctx, client, args[1])
		if err != nil {
			return err
		}
		if jsonMode {
			return json.NewEncoder(stdout).Encode(session)
		}
		_, err = fmt.Fprint(stdout, terminalSafe(exportSessionText(session)))
		return err
	case "delete", "del", "rm":
		if len(args) != 2 || strings.TrimSpace(args[1]) == "" {
			return usage("session delete requires NAME")
		}
		var result struct {
			Success *bool `json:"success"`
		}
		if err := client.JSON(ctx, http.MethodDelete, "/v1/sessions/"+url.PathEscape(args[1]), nil, &result); err != nil {
			return err
		}
		if result.Success == nil || !*result.Success {
			return &serviceclient.Error{Code: "invalid_response", Message: "The service did not confirm conversation deletion. Refresh its state before retrying."}
		}
		if jsonMode {
			return json.NewEncoder(stdout).Encode(map[string]any{"success": true, "name": args[1]})
		}
		_, err := fmt.Fprintf(stdout, "OK Deleted conversation: %s\n", terminalSafe(args[1]))
		return err
	case "export":
		if len(args) < 2 || len(args) > 3 {
			return usage("session export requires NAME [FILE]")
		}
		file := ""
		if len(args) == 3 {
			file = args[2]
		}
		return exportServiceSession(ctx, client, args[1], "markdown", file, stdout, jsonMode)
	default:
		return usage("unknown session command %q", args[0])
	}
}

func runSessionExportCommand(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return usage("export-session requires NAME [--format markdown|json|txt] [--output FILE]")
	}
	if args[0] == "--help" || args[0] == "-h" {
		return runSessionCommand(ctx, []string{"help"})
	}
	format, file := "markdown", ""
	for i := 1; i < len(args); i++ {
		if i+1 >= len(args) {
			return usage("missing value for %s", args[i])
		}
		switch args[i] {
		case "--format":
			format = args[i+1]
		case "--output":
			file = args[i+1]
		default:
			return usage("unknown export option %q", args[i])
		}
		i++
	}
	client, err := newCommandClient()
	if err != nil {
		return err
	}
	return exportServiceSession(ctx, client, args[0], format, file, os.Stdout, output.JSONMode)
}

func exportServiceSession(ctx context.Context, client *serviceclient.Client, name, format, filename string, stdout io.Writer, jsonMode bool) error {
	switch format {
	case "markdown", "md", "json", "text", "txt":
	default:
		return usage("unsupported export format %q", format)
	}
	session, err := loadServiceSession(ctx, client, name)
	if err != nil {
		return err
	}
	var content string
	switch format {
	case "markdown", "md":
		content = exportSessionMarkdown(session)
	case "text", "txt":
		content = exportSessionText(session)
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
		file, err := os.OpenFile(filename, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
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
			os.Remove(filename)
			return err
		}
		if jsonMode {
			return json.NewEncoder(stdout).Encode(map[string]any{"success": true, "path": filename, "format": format})
		}
		_, err = fmt.Fprintf(stdout, "OK Exported conversation to %s\n", terminalSafe(filename))
		return err
	}
	if jsonMode {
		return json.NewEncoder(stdout).Encode(map[string]string{"format": format, "content": content})
	}
	_, err = fmt.Fprintln(stdout, terminalSafe(content))
	return err
}
