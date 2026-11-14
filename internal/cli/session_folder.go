package cli

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func selectConversationFolder(ctx context.Context, client *serviceclient.Client, c *api.Conversation, session, requestID string) (*api.ConversationCommandReceipt, error) {
	command, err := prepareConversationFolder(ctx, client, c, session, requestID)
	if err != nil {
		return nil, err
	}
	return client.ConversationCommand(ctx, *command)
}

func prepareConversationFolder(ctx context.Context, client *serviceclient.Client, c *api.Conversation, session, requestID string) (*api.ConversationCommand, error) {
	if session == "" {
		return nil, Usage("/folder SESSION|none")
	}
	command := api.ConversationCommand{RequestID: requestID, TargetID: c.ID, ExpectedRevision: c.Revision, Kind: "clear_project_folder"}
	if session != "none" {
		ref, resource, err := resolveExecutionWorkspace(ctx, client, session)
		if err != nil {
			return nil, err
		}
		if !resource.ProjectContext {
			return nil, fmt.Errorf("this execution host does not support persistent project context; update the matching host before selecting it")
		}
		command.Kind = "use_project_folder"
		command.Payload.Folder = &api.ExecutionReference{Host: ref.Host, Session: ref.Session, Grant: ref.Grant, Revision: ref.Revision}
	}
	return &command, nil
}

func runSessionFolder(ctx context.Context, client *serviceclient.Client, args []string, out io.Writer, jsonMode bool) error {
	if len(args) != 3 {
		return Usage("session folder NAME SESSION|none")
	}
	saved, err := LoadServiceSession(ctx, client, args[1])
	if err != nil {
		return err
	}
	var c *api.Conversation
	if saved.CanonicalID == "" {
		if err = client.RequireCapability(ctx, "conversation-input-admission-v1"); err != nil {
			return err
		}
		c, err = client.ImportSessionConversation(ctx, saved.Name, rand.Text())
	} else {
		c, err = client.CanonicalConversation(ctx, saved.CanonicalID)
	}
	if err != nil {
		return err
	}
	receipt, err := selectConversationFolder(ctx, client, c, args[2], rand.Text())
	if err != nil {
		return err
	}
	if receipt.Conversation == nil {
		return fmt.Errorf("the service returned no conversation after folder selection")
	}
	if jsonMode {
		return json.NewEncoder(out).Encode(receipt)
	}
	_, err = fmt.Fprintf(out, "Conversation folder updated. Project: %s\n", TerminalSafe(receipt.Conversation.ProjectID))
	return err
}
