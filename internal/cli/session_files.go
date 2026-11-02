package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
)

// ReadChatFile reads only the path explicitly supplied by the user. Snapshot
// bytes before upload; unreadable, non-regular or growing files never become a
// text-only request or a different local path.
func ReadChatFile(path string) (string, []byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", nil, fmt.Errorf("cannot open attachment: %w", err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", nil, err
	}
	if !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > sessions.MaxAttachmentBytes {
		return "", nil, Usage("choose a regular, nonempty file of at most 50 MiB")
	}
	data, err := io.ReadAll(io.LimitReader(f, sessions.MaxAttachmentBytes+1))
	if err != nil {
		return "", nil, err
	}
	after, err := f.Stat()
	if err != nil || int64(len(data)) != info.Size() || after.Size() != info.Size() || after.ModTime() != info.ModTime() {
		return "", nil, fmt.Errorf("attachment changed while reading; no upload was made")
	}
	return filepath.Base(path), data, nil
}

func RunSessionFiles(ctx context.Context, client *serviceclient.Client, args []string, out io.Writer, jsonMode bool) error {
	if len(args) < 2 {
		return Usage("session attach NAME FILE | attachments NAME | remove-attachment NAME ID")
	}
	name := args[1]
	switch args[0] {
	case "attach":
		if len(args) != 3 {
			return Usage("session attach NAME FILE")
		}
		filename, data, err := ReadChatFile(args[2])
		if err != nil {
			return err
		}
		item, err := client.UploadSessionFile(ctx, name, filename, data)
		if err != nil {
			var remote *serviceclient.Error
			if !jsonMode && errors.As(err, &remote) && remote.AttachmentID != "" {
				fmt.Fprintf(out, "File retained: %s\nInspect: offgrid session attachments %s\n", remote.AttachmentID, TerminalSafe(name))
			}
			return err
		}
		if jsonMode {
			return json.NewEncoder(out).Encode(item)
		}
		_, err = fmt.Fprintf(out, "Attached: %s\nSource: %s\nRevision: %s\n", TerminalSafe(item.Name), item.ID, item.Revision)
		return err
	case "attachments":
		if len(args) != 2 {
			return Usage("session attachments NAME")
		}
		items, err := client.SessionFiles(ctx, name)
		if err != nil {
			return err
		}
		if jsonMode {
			return json.NewEncoder(out).Encode(map[string]any{"attachments": items})
		}
		if len(items) == 0 {
			_, err = fmt.Fprintln(out, "No attached files.")
			return err
		}
		for _, a := range items {
			if _, err = fmt.Fprintf(out, "%s  %s  %s  %d bytes\n", a.ID, TerminalSafe(a.Name), TerminalSafe(a.Status), a.Bytes); err != nil {
				return err
			}
		}
		return nil
	case "remove-attachment":
		if len(args) != 3 {
			return Usage("session remove-attachment NAME ID")
		}
		if err := client.RemoveSessionFile(ctx, name, args[2]); err != nil {
			return err
		}
		if jsonMode {
			return json.NewEncoder(out).Encode(map[string]bool{"removed": true})
		}
		_, err := fmt.Fprintln(out, "Unused source removed.")
		return err
	}
	return Usage("unknown attachment command")
}
