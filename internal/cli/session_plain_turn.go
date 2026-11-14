package cli

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func plainConversationArgs(args []string) bool {
	for _, arg := range args {
		if arg == "--profile" || strings.HasPrefix(arg, "--profile=") {
			return false
		}
	}
	return true
}

// Ordinary conversation shares service-owned history and attachment validation
// with the app. Profile-driven agent dispatch keeps its existing governed path.
func RunPlainConversationTurn(ctx context.Context, client *serviceclient.Client, args []string, out io.Writer, jsonMode bool) error {
	if len(args) < 2 {
		return Usage("session send NAME [--file PATH] [--attachment ID] [--model MODEL] MESSAGE")
	}
	name := args[1]
	request := serviceclient.ConversationRequest{RequestID: rand.Text()}
	var paths, ids, words []string
	questionID := ""
	intent := ""
	for i := 2; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			words = append(words, args[i+1:]...)
			break
		}
		switch arg {
		case "--file", "--attachment", "--model", "--request-id", "--image", "--answer", "--intent":
			if i+1 == len(args) {
				return Usage("%s needs a value", arg)
			}
			i++
			value := args[i]
			switch arg {
			case "--intent":
				if value != "message" && value != "new_work" && value != "correction" {
					return Usage("--intent must be message, new_work or correction; use --answer QUESTION_ID for answers")
				}
				intent = value
			case "--answer":
				questionID = value
			case "--file":
				paths = append(paths, value)
			case "--image":
				if _, _, err := ChatImageCommand("/image "+value, &request.Images); err != nil {
					return err
				}
			case "--attachment":
				ids = append(ids, value)
			case "--model":
				request.Model = value
			case "--request-id":
				request.RequestID = value
			}
		case "--rag":
			request.Knowledge = true
		default:
			if strings.HasPrefix(arg, "--") {
				return Usage("unknown conversation option %s", arg)
			}
			words = append(words, arg)
		}
	}
	request.Content = strings.Join(words, " ")
	if questionID != "" && intent != "" {
		return Usage("--answer and --intent cannot be combined")
	}
	if request.Content == "" && len(paths)+len(ids)+len(request.Images) == 0 {
		return Usage("message or file is required")
	}
	if len(paths)+len(ids) > 8 {
		return Usage("at most eight files may be attached")
	}
	saved, err := LoadServiceSession(ctx, client, name)
	if err != nil {
		return err
	}
	if saved.CanonicalID != "" {
		if len(paths)+len(ids)+len(request.Images) > 0 || request.Knowledge {
			return Usage("this conversation currently accepts text input; document and image admission is not enabled")
		}
		if questionID != "" {
			return sendCanonicalInput(ctx, client, saved.CanonicalID, request.RequestID, api.ConversationInboxInput{Content: request.Content, Model: request.Model, Origin: "typed", Intent: "answer", QuestionID: questionID}, out, jsonMode)
		}
		if intent != "" {
			return sendCanonicalInput(ctx, client, saved.CanonicalID, request.RequestID, api.ConversationInboxInput{Content: request.Content, Model: request.Model, Origin: "typed", Intent: intent}, out, jsonMode)
		}
		return sendCanonicalConversation(ctx, client, saved.CanonicalID, request, out, jsonMode)
	}
	if questionID != "" {
		return Usage("--answer requires a saved canonical conversation question")
	}
	if len(paths)+len(ids)+len(request.Images) == 0 && !request.Knowledge && serviceclient.PlainSessionCanMigrate(saved) {
		if identity, e := client.Identity(ctx); e == nil && identity.Supports("conversation-input-admission-v1") {
			conversation, e := client.ImportSessionConversation(ctx, name, request.RequestID)
			if e != nil {
				return e
			}
			return sendCanonicalInput(ctx, client, conversation.ID, request.RequestID, api.ConversationInboxInput{Content: request.Content, Model: request.Model, Origin: "typed", Intent: intent}, out, jsonMode)
		}
	}
	if intent != "" {
		return Usage("explicit input intent requires a canonical conversation and conversation input support on the service")
	}
	if len(paths)+len(ids) > 0 {
		if err := client.RequireCapability(ctx, "session-files-v1"); err != nil {
			return err
		}
	}
	if len(ids) > 0 {
		items, err := client.SessionFiles(ctx, name)
		if err != nil {
			return err
		}
		seen := map[string]bool{}
		for _, id := range ids {
			var found *sessions.Attachment
			for i := range items {
				if items[i].ID == id {
					found = &items[i]
					break
				}
			}
			if found == nil || found.Status != "ready" || found.Revision == "" || seen[id] {
				return Usage("attachment is unavailable, unprepared or duplicated; inspect session attachments")
			}
			seen[id] = true
			request.Attachments = append(request.Attachments, sessions.AttachmentReference{ID: found.ID, Revision: found.Revision})
		}
	}
	for _, path := range paths {
		filename, data, err := ReadChatFile(path)
		if err != nil {
			return err
		}
		item, err := client.UploadSessionFile(ctx, name, filename, data)
		if err != nil {
			return err
		}
		request.Attachments = append(request.Attachments, sessions.AttachmentReference{ID: item.ID, Revision: item.Revision})
	}
	return sendPlainConversation(ctx, client, name, request, out, jsonMode)
}

func sendPlainConversation(ctx context.Context, client *serviceclient.Client, name string, request serviceclient.ConversationRequest, out io.Writer, jsonMode bool) error {
	if len(request.Images) > 0 {
		if err := api.ValidateSessionImages(request.Images); err != nil {
			return err
		}
	}
	result, err := client.Conversation(ctx, name, request, func(delta string) error {
		if jsonMode {
			return nil
		}
		_, err := io.WriteString(out, TerminalSafe(delta))
		return err
	})
	if err != nil {
		return err
	}
	if jsonMode {
		return json.NewEncoder(out).Encode(result)
	}
	_, err = fmt.Fprintln(out)
	if err == nil {
		if notice := DocumentCoverageNotice(result.Session); notice != "" {
			_, err = fmt.Fprintln(out, notice)
		}
	}
	return err
}
