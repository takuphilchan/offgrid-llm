package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Pending file selections are separate from stored conversation sources. Failed
// generation keeps the same references; removing a selection never deletes a
// source from history or makes a fresh inference request.
type ConversationFileDraft struct {
	Files         []sessions.Attachment
	FailedUploads []string // local paths only; never sent as model context
}

func (d *ConversationFileDraft) uploadFailed(path string) {
	for _, previous := range d.FailedUploads {
		if previous == path {
			return
		}
	}
	d.FailedUploads = append(d.FailedUploads, path)
}

func (d *ConversationFileDraft) uploadResolved(path string) {
	for i, previous := range d.FailedUploads {
		if previous == path {
			d.FailedUploads = append(d.FailedUploads[:i], d.FailedUploads[i+1:]...)
			return
		}
	}
}

func conversationFilePath(arg string) string {
	arg = strings.TrimSpace(arg)
	if len(arg) > 1 && ((arg[0] == '"' && arg[len(arg)-1] == '"') || (arg[0] == '\'' && arg[len(arg)-1] == '\'')) {
		return arg[1 : len(arg)-1]
	}
	return arg
}

func IsConversationFileCommand(input string) bool {
	name, _, _ := strings.Cut(strings.TrimSpace(input), " ")
	switch name {
	case "/attach", "/attachments", "/remove-attachment", "/retry-attachment", "/use-attachment", "/attachment-text-only", "/preview-attachment":
		return true
	}
	return false
}

func (d *ConversationFileDraft) Command(ctx context.Context, client *serviceclient.Client, name, input string) (bool, string, error) {
	command, arg, _ := strings.Cut(strings.TrimSpace(input), " ")
	arg = strings.TrimSpace(arg)
	switch command {
	case "/preview-attachment":
		fields := strings.Fields(arg)
		if len(fields) < 1 || len(fields) > 2 {
			return true, "", Usage("/preview-attachment N [source-unit]")
		}
		n, err := strconv.Atoi(fields[0])
		if err != nil || n < 1 || n > len(d.Files) {
			return true, "", Usage("choose an attachment number from /attachments")
		}
		unit := 1
		if len(fields) == 2 {
			unit, err = strconv.Atoi(fields[1])
			if err != nil || unit < 1 {
				return true, "", Usage("source-unit must be positive")
			}
		}
		preview, err := client.PreviewSessionFile(ctx, name, d.Files[n-1])
		if err != nil {
			return true, "", err
		}
		if unit > len(preview.Units) {
			return true, "", Usage("source unit is unavailable")
		}
		part := preview.Units[unit-1]
		body := []rune(part.Text)
		suffix := ""
		if len(body) > 12000 {
			body = body[:12000]
			suffix = "\n[Display truncated at 12,000 characters; the stored source is unchanged.]"
		}
		return true, fmt.Sprintf("%s · %s %d · %s\nSource: %s\n%s%s", TerminalSafe(preview.Name), TerminalSafe(part.Kind), part.Index, TerminalSafe(part.State), preview.SHA256, TerminalSafe(string(body)), suffix), nil
	case "/use-attachment":
		if len(d.Files) >= 8 {
			return true, "", Usage("remove an attachment first; at most eight files")
		}
		items, err := client.SessionFiles(ctx, name)
		if err != nil {
			return true, "", err
		}
		for _, selected := range d.Files {
			if selected.ID == arg {
				return true, "", Usage("this source is already selected")
			}
		}
		for _, item := range items {
			if item.ID == arg {
				d.Files = append(d.Files, item)
				return true, "Selected: " + TerminalSafe(item.Name), nil
			}
		}
		return true, "", Usage("source not found; inspect session attachments NAME")
	case "/retry-attachment", "/attachment-text-only":
		n, err := strconv.Atoi(arg)
		if err != nil || n < 1 || n > len(d.Files) {
			return true, "", Usage("%s N from /attachments", command)
		}
		var item sessions.Attachment
		if command == "/attachment-text-only" {
			item, err = client.PrepareSessionFileTextOnly(ctx, name, d.Files[n-1])
		} else {
			item, err = client.PrepareSessionFile(ctx, name, d.Files[n-1])
		}
		if err != nil {
			return true, "", err
		}
		d.Files[n-1] = item
		if item.Coverage == "native_text_only" {
			return true, "Available text selected: " + TerminalSafe(item.Name) + ". Image/scanned content remains unread; answers cannot cover the whole document.", nil
		}
		return true, "Ready: " + TerminalSafe(item.Name), nil
	case "/attach":
		if arg == "" {
			return true, "", Usage("/attach PATH")
		}
		if len(d.Files) >= 8 {
			return true, "", Usage("remove an attachment first; at most eight files")
		}
		filename, data, err := ReadChatFile(conversationFilePath(arg))
		if err != nil {
			d.uploadFailed(conversationFilePath(arg))
			return true, "", err
		}
		item, err := client.UploadSessionFile(ctx, name, filename, data)
		if err != nil {
			d.uploadFailed(conversationFilePath(arg))
			var remote *serviceclient.Error
			if errors.As(err, &remote) && remote.AttachmentID != "" {
				if items, readErr := client.SessionFiles(ctx, name); readErr == nil {
					for _, a := range items {
						if a.ID == remote.AttachmentID {
							d.Files = append(d.Files, a)
							d.uploadResolved(conversationFilePath(arg))
							break
						}
					}
				}
			}
			return true, "", err
		}
		d.Files = append(d.Files, item)
		d.uploadResolved(conversationFilePath(arg))
		if item.Coverage == "native_text_only" {
			return true, "Attached: " + TerminalSafe(item.Name) + ". Text ready; images and figures have not been interpreted.", nil
		}
		return true, "Attached: " + TerminalSafe(item.Name), nil
	case "/attachments":
		if arg != "" {
			return true, "", Usage("/attachments takes no arguments")
		}
		if len(d.Files)+len(d.FailedUploads) == 0 {
			return true, "No files selected for the next message. Saved sources remain in conversation history.", nil
		}
		var text strings.Builder
		for _, path := range d.FailedUploads {
			fmt.Fprintf(&text, "Upload not confirmed: %s. Inspect saved sources, attach again explicitly, or /remove-attachment all.\n", TerminalSafe(path))
		}
		for i, item := range d.Files {
			fmt.Fprintf(&text, "%d. %s · %s\n", i+1, TerminalSafe(item.Name), TerminalSafe(item.Status))
			if item.ErrorCode != "" {
				fmt.Fprintf(&text, "  %s\n", TerminalSafe(item.ErrorCode))
			}
			if item.Coverage == "native_text_only" {
				fmt.Fprintln(&text, "  Native text only; image/scanned content remains unread.")
			}
		}
		return true, strings.TrimSpace(text.String()), nil
	case "/remove-attachment":
		if arg == "all" {
			d.Files = nil
			d.FailedUploads = nil
			return true, "Files removed from the draft. Saved sources remain available.", nil
		}
		n, err := strconv.Atoi(arg)
		if err != nil || n < 1 || n > len(d.Files) {
			return true, "", Usage("/remove-attachment N from /attachments, or /remove-attachment all")
		}
		d.Files = append(d.Files[:n-1], d.Files[n:]...)
		return true, "File removed from the draft.", nil
	}
	return false, "", nil
}

func RunPlainConversationChat(ctx context.Context, client *serviceclient.Client, args []string, in io.Reader, out io.Writer, jsonMode bool) error {
	if len(args) != 2 {
		return Usage("session chat NAME; use session create NAME --model MODEL first")
	}
	if jsonMode {
		return Usage("interactive chat is not JSON mode; use session send NAME MESSAGE --json")
	}
	name := args[1]
	saved, err := LoadServiceSession(ctx, client, name)
	if err != nil {
		return err
	}
	draft := ConversationFileDraft{}
	var images []api.SessionImage
	if saved.Turn != nil && saved.Turn.Status != "completed" {
		images = append(images, saved.Turn.Images...)
	}
	if saved.Turn != nil && saved.Turn.Status != "completed" && len(saved.Turn.Attachments) > 0 {
		items, err := client.SessionFiles(ctx, name)
		if err != nil {
			return err
		}
		for _, ref := range saved.Turn.Attachments {
			found := false
			for _, a := range items {
				if a.ID == ref.ID && a.Revision == ref.Revision {
					draft.Files = append(draft.Files, a)
					found = true
				}
			}
			if !found {
				return Usage("a saved turn source is unavailable or changed; inspect session attachments before continuing")
			}
		}
	}
	return conversationChatLoop(ctx, client, saved, draft, images, saved.ModelID, false, in, out)
}

func conversationChatLoop(ctx context.Context, client *serviceclient.Client, saved *sessions.Session, draft ConversationFileDraft, images []api.SessionImage, model string, knowledge bool, in io.Reader, out io.Writer) error {
	name := saved.Name
	files := &ServiceFileConversation{Context: ctx, Client: client, Name: name, Saved: saved}
	pendingText, hasPending := "", false
	fmt.Fprintf(out, "Chat\n  %s\n  %s\n\n/attach PATH · /attachments · /remove-attachment N · /exit\n", TerminalSafe(name), TerminalSafe(model))
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 4096), 64<<10)
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		fmt.Fprint(out, "\n> ")
		if !scanner.Scan() {
			return scanner.Err()
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "/retry" {
			if !hasPending {
				fmt.Fprintln(out, "No failed message to retry.")
				continue
			}
			input = pendingText
		}
		if input == "/exit" || input == "/quit" {
			return nil
		}
		if strings.HasPrefix(input, "/") {
			if handled, message, err := ChatImageCommand(input, &images); handled {
				if err != nil {
					fmt.Fprintln(out, TerminalSafe(err.Error()))
				} else {
					fmt.Fprintln(out, message)
				}
				continue
			}
		}
		if handled, message, err := draft.Command(ctx, client, name, input); handled {
			if err != nil {
				fmt.Fprintln(out, TerminalSafe(err.Error()))
			} else {
				fmt.Fprintln(out, message)
			}
			continue
		}
		if input == "/help" {
			fmt.Fprintln(out, "/attach PATH · /attachments · /preview-attachment N [unit] · /attachment-text-only N · /remove-attachment N|all · /retry-attachment N · /use-attachment ID · /image PATH · /images · /remove-image N|all · /retry · /status · /rag · /exit")
			continue
		}
		if input == "/status" {
			fmt.Fprintf(out, "Conversation: %s\nModel: %s\nResume: offgrid session chat %s\n", TerminalSafe(name), TerminalSafe(model), TerminalSafe(name))
			continue
		}
		if input == "/rag" {
			knowledge = !knowledge
			fmt.Fprintf(out, "Knowledge retrieval: %t\n", knowledge)
			continue
		}
		if input == "/clear" {
			fmt.Fprintln(out, "This conversation has retained sources. Use /exit and start a new conversation to keep its history intact.")
			continue
		}
		if strings.HasPrefix(input, "/") {
			fmt.Fprintln(out, "Unknown command. Use /help.")
			continue
		}
		if input == "" && len(draft.Files)+len(images) == 0 {
			continue
		}
		files.Draft = draft
		if _, err := files.Stream(model, input, images, knowledge, func(delta string) error { _, err := io.WriteString(out, TerminalSafe(delta)); return err }); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			fmt.Fprintln(out, TerminalSafe(err.Error()))
			pendingText, hasPending = input, true
			fmt.Fprintln(out, "Draft retained. Resolve any file errors, then /retry or enter a revised message.")
			continue
		}
		fmt.Fprintln(out)
		if notice := DocumentCoverageNotice(files.Saved); notice != "" {
			fmt.Fprintln(out, notice)
		}
		pendingText, hasPending = "", false
		draft.Files = nil
		images = nil
	}
}
