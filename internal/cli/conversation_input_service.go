package cli

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func sendCanonicalConversation(ctx context.Context, client *serviceclient.Client, id string, request serviceclient.ConversationRequest, out io.Writer, jsonMode bool) error {
	return sendCanonicalInput(ctx, client, id, request.RequestID, api.ConversationInboxInput{Content: request.Content, Model: request.Model, Origin: "typed", ResponseProfile: request.ResponseProfile, MaxTokens: request.MaxTokens}, out, jsonMode)
}

func sendCanonicalInput(ctx context.Context, client *serviceclient.Client, id, requestID string, input api.ConversationInboxInput, out io.Writer, jsonMode bool) error {
	if input.ResponseProfile != "" || input.MaxTokens != 0 || strings.TrimSpace(input.Content) == "" {
		if err := client.RequireCapability(ctx, "conversation-input-options-v1"); err != nil {
			return err
		}
	}
	entry, err := client.SubmitSessionInput(ctx, id, requestID, input)
	if err != nil {
		return err
	}
	if jsonMode {
		return json.NewEncoder(out).Encode(entry)
	}
	_, err = fmt.Fprintf(out, "Input accepted: %s\nRequest: %s\nWork continues in the service; use session show or session chat to observe it.\n", TerminalSafe(entry.ID), TerminalSafe(requestID))
	return err
}

func startCanonicalConversationChat(ctx context.Context, client *serviceclient.Client, in io.Reader, out io.Writer, jsonMode bool) error {
	if jsonMode {
		return Usage("interactive chat is not JSON mode; use session send NAME MESSAGE --json")
	}
	if err := client.RequireCapability(ctx, "conversation-input-admission-v1"); err != nil {
		return err
	}
	name := "conversation-" + rand.Text()
	var saved sessions.Session
	if err := client.JSON(ctx, http.MethodPost, "/v1/sessions", map[string]string{"name": name}, &saved); err != nil {
		return err
	}
	c, err := client.ImportSessionConversation(ctx, name, rand.Text())
	if err != nil {
		return err
	}
	saved.CanonicalID = c.ID
	return RunCanonicalConversationChat(ctx, client, &saved, in, out)
}

func canonicalChatControl(ctx context.Context, client *serviceclient.Client, id, command, arg string) error {
	c, err := client.CanonicalConversation(ctx, id)
	if err != nil {
		return err
	}
	switch command {
	case "/folder":
		_, err = selectConversationFolder(ctx, client, c, arg, rand.Text())
		return err
	case "/pause", "/stop", "/continue":
		if c.FocusedWorkID == "" {
			return Usage("there is no focused work; /stop-input INPUT_ID withdraws conversation input")
		}
		w, err := client.WorkItem(ctx, c.FocusedWorkID)
		if err != nil {
			return err
		}
		_, err = client.WorkCommand(ctx, api.WorkCommand{RequestID: rand.Text(), TargetID: w.ID, ExpectedRevision: w.Revision, Kind: command[1:]})
		return err
	case "/stop-input", "/continue-input":
		if arg == "" {
			return Usage("%s INPUT_ID", command)
		}
		kind := "stop"
		if command == "/continue-input" {
			kind = "continue"
		}
		_, err = client.ControlConversationInput(ctx, id, api.ConversationInputControlCommand{RequestID: rand.Text(), TargetID: arg, ExpectedRevision: c.Revision, Kind: kind})
		return err
	case "/model":
		if arg == "" {
			return Usage("/model MODEL")
		}
		c.Preferences.Model = arg
		_, err = client.ConversationCommand(ctx, api.ConversationCommand{RequestID: rand.Text(), TargetID: id, ExpectedRevision: c.Revision, Kind: "configure_conversation", Payload: api.ConversationCommandPayload{Preferences: &c.Preferences}})
		return err
	case "/project":
		if arg == "" {
			return Usage("/project PROJECT_ID")
		}
		_, err = client.ConversationCommand(ctx, api.ConversationCommand{RequestID: rand.Text(), TargetID: id, ExpectedRevision: c.Revision, Kind: "select_project", Payload: api.ConversationCommandPayload{ProjectID: arg}})
		return err
	case "/profile":
		parts := strings.Fields(arg)
		if len(parts) < 1 || len(parts) > 2 {
			return Usage("/profile ID [REVISION], or /profile none")
		}
		if parts[0] == "none" {
			if len(parts) != 1 {
				return Usage("/profile none")
			}
			c.Preferences.ProfileID, c.Preferences.ProfileRevision = "", 0
		} else {
			path := "/api/v2/agent-profiles/" + url.PathEscape(parts[0])
			if len(parts) == 2 {
				revision, e := strconv.Atoi(parts[1])
				if e != nil || revision < 1 {
					return Usage("profile revision must be positive")
				}
				path += "?revision=" + strconv.Itoa(revision)
			}
			var profile struct {
				ID       string `json:"id"`
				Revision int    `json:"revision"`
			}
			if err = client.JSON(ctx, http.MethodGet, path, nil, &profile); err != nil {
				return err
			}
			if profile.ID != parts[0] || profile.Revision < 1 {
				return Usage("the service returned an invalid profile revision")
			}
			c.Preferences.ProfileID, c.Preferences.ProfileRevision = profile.ID, profile.Revision
		}
		_, err = client.ConversationCommand(ctx, api.ConversationCommand{RequestID: rand.Text(), TargetID: id, ExpectedRevision: c.Revision, Kind: "configure_conversation", Payload: api.ConversationCommandPayload{Preferences: &c.Preferences}})
		return err
	}
	return Usage("unknown command; use /help")
}
