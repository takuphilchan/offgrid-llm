package cli

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Retain the exact envelope across an unknown response. Refreshing a revision
// or resolving a newly connected host would turn a retry into a different choice.
type conversationFolderControl struct {
	pending *api.ConversationCommand
	session string
}

func (control *conversationFolderControl) handle(ctx context.Context, client *serviceclient.Client, conversation, command, arg string, out io.Writer) (bool, error) {
	if command != "/folder" && command != "/retry-folder" {
		return false, nil
	}
	if command == "/retry-folder" {
		if arg != "" || control.pending == nil {
			return true, Usage("/retry-folder requires a pending folder command and no arguments")
		}
	} else if control.pending != nil {
		if arg != control.session {
			return true, Usage("the previous folder choice has an unknown outcome; use /retry-folder before making a different choice")
		}
	} else {
		if arg == "" {
			return true, Usage("/folder SESSION|none")
		}
		c, err := client.CanonicalConversation(ctx, conversation)
		if err != nil {
			return true, err
		}
		prepared, err := prepareConversationFolder(ctx, client, c, arg, rand.Text())
		if err != nil {
			return true, err
		}
		control.pending, control.session = prepared, arg
	}
	return true, control.dispatch(ctx, client, out)
}

func (control *conversationFolderControl) dispatch(ctx context.Context, client *serviceclient.Client, out io.Writer) error {
	receipt, err := client.ConversationCommand(ctx, *control.pending)
	if err == nil && (receipt.Conversation == nil || receipt.Conversation.ID != control.pending.TargetID) {
		err = &serviceclient.Error{Code: "invalid_response", Message: "The service did not confirm the selected conversation folder."}
	}
	if err != nil {
		var remote *serviceclient.Error
		if errors.As(err, &remote) && (remote.Status == http.StatusBadRequest || remote.Status == http.StatusConflict || remote.Status == http.StatusUnprocessableEntity) {
			control.pending, control.session = nil, ""
		} else {
			fmt.Fprintf(out, "Folder command retained. /retry-folder uses request %s.\n", TerminalSafe(control.pending.RequestID))
		}
		return err
	}
	control.pending, control.session = nil, ""
	_, err = fmt.Fprintln(out, "Folder command acknowledged.")
	return err
}
