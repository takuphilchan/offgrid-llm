package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

type AgentDefaultScope struct {
	Origin    string `json:"origin"`
	Workspace string `json:"workspace"`
	Actor     string `json:"actor"`
}

type AgentDefault struct {
	Version   int               `json:"version"`
	Scope     AgentDefaultScope `json:"scope"`
	ProfileID string            `json:"profile_id"`
	Revision  int               `json:"revision"`
}

func (d AgentDefault) Valid(scope AgentDefaultScope) bool {
	return d.Version == 1 && d.Scope == scope && scope.Origin != "" && scope.Workspace != "" && scope.Actor != "" && ValidProfileID(d.ProfileID) && d.Revision > 0
}

// No credentials, grants, model substitutions or execution state are persisted.
type AgentDefaultStore interface {
	Load(AgentDefaultScope) (*AgentDefault, error)
	Save(AgentDefaultScope, *AgentDefault) error // nil forgets this scope only
}

func agentDefaultScope(ctx context.Context, client *serviceclient.Client) (AgentDefaultScope, error) {
	identity, err := client.Identity(ctx)
	if err != nil {
		return AgentDefaultScope{}, err
	}
	if identity.Product != "offgrid" || identity.WorkspaceID == "" {
		return AgentDefaultScope{}, fmt.Errorf("the service has no verified workspace identity")
	}
	var account struct {
		Required      *bool `json:"auth_required"`
		Authenticated bool  `json:"authenticated"`
		Guest         bool  `json:"guest"`
		User          *struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	if err := client.JSON(ctx, http.MethodGet, "/v1/users/me", nil, &account); err != nil {
		return AgentDefaultScope{}, err
	}
	actor := ""
	if !account.Guest && account.Authenticated && account.User != nil && account.User.ID != "" {
		actor = "user:" + account.User.ID
	} else if !account.Guest && account.Required != nil && !*account.Required && !account.Authenticated {
		actor = "local-admin"
	}
	if actor == "" {
		return AgentDefaultScope{}, fmt.Errorf("a verified signed-in identity is required to remember a default")
	}
	return AgentDefaultScope{Origin: client.Origin(), Workspace: identity.WorkspaceID, Actor: actor}, nil
}

func resolveAgentResume(ctx context.Context, client *serviceclient.Client, options interactiveAgentOptions) (interactiveAgentOptions, error) {
	if options.resume == "" {
		return options, nil
	}
	session, err := LoadServiceSession(ctx, client, options.resume)
	if err != nil {
		return options, err
	}
	if session.Turn == nil || session.Turn.Agent == nil {
		if options.profile == "" {
			return options, Usage("This conversation has no saved agent identity. Resume with --profile ID (and --profile-revision N when known)")
		}
		return options, nil
	}
	selection := session.Turn.Agent
	if !ValidProfileID(selection.ID) || selection.Revision < 1 {
		return options, fmt.Errorf("Saved agent identity is invalid; no work was started")
	}
	if options.profile != "" && options.profile != selection.ID || options.revision > 0 && options.revision != selection.Revision {
		return options, Usage("Resume must use the conversation's saved profile and revision. Use /agent to start a separate conversation")
	}
	if options.model != "" && options.model != session.Turn.Model {
		return options, Usage("Resume must use the conversation's saved model")
	}
	options.profile, options.revision = selection.ID, selection.Revision
	return options, nil
}

func rememberedAgent(ctx context.Context, client *serviceclient.Client, env Environment, options interactiveAgentOptions) interactiveAgentOptions {
	if env.Service.AgentDefaults == nil || options.profile != "" || options.resume != "" {
		return options
	}
	scope, err := agentDefaultScope(ctx, client)
	if err != nil {
		return options
	} // A legacy service still supports normal selection.
	value, err := env.Service.AgentDefaults.Load(scope)
	if err != nil {
		fmt.Fprintln(env.Stdout, "Saved agent preference could not be read. Choose an agent for this session.")
		return options
	}
	if value == nil {
		return options
	}
	if !value.Valid(scope) {
		fmt.Fprintln(env.Stdout, "Saved agent preference is invalid. Choose an agent again.")
		return options
	}
	current, err := selectInteractiveProfile(ctx, client, interactiveAgentOptions{profile: value.ProfileID}, nil, io.Discard, env.Style)
	if err != nil || current.Revision != value.Revision || options.model != "" && options.model != current.Settings.Model {
		fmt.Fprintln(env.Stdout, "Saved agent is unavailable, changed, or uses another model. Choose an agent again; the default has not been changed.")
		return options
	}
	fmt.Fprintf(env.Stdout, "Using saved agent %s. /agent to choose another; /default clear to forget.\n", TerminalSafe(current.Settings.Name))
	options.profile, options.revision = value.ProfileID, value.Revision
	return options
}

func agentDefaultCommand(ctx context.Context, client *serviceclient.Client, store AgentDefaultStore, profile agents.AgentProfile, text string, out io.Writer) {
	if store == nil {
		fmt.Fprintln(out, "Remembered defaults are unavailable in this entry point.")
		return
	}
	if text != "/default" && text != "/default show" && text != "/default clear" {
		fmt.Fprintln(out, "Use /default to remember this agent, /default show, or /default clear.")
		return
	}
	scope, err := agentDefaultScope(ctx, client)
	if err != nil {
		fmt.Fprintf(out, "Default not changed: %s\n", TerminalSafe(err.Error()))
		return
	}
	switch text {
	case "/default show":
		value, loadErr := store.Load(scope)
		if loadErr != nil {
			err = loadErr
		} else if value == nil {
			fmt.Fprintln(out, "No default agent for this service, workspace and user.")
		} else if !value.Valid(scope) {
			err = fmt.Errorf("invalid saved preference")
		} else {
			fmt.Fprintf(out, "Default: %s · revision %d\n", TerminalSafe(value.ProfileID), value.Revision)
		}
	case "/default clear":
		err = store.Save(scope, nil)
		if err == nil {
			fmt.Fprintln(out, "Default forgotten. This conversation and its permissions are unchanged.")
		}
	default:
		value := AgentDefault{Version: 1, Scope: scope, ProfileID: profile.ID, Revision: profile.Revision}
		if !value.Valid(scope) {
			err = fmt.Errorf("invalid profile identity")
		} else {
			err = store.Save(scope, &value)
		}
		if err == nil {
			fmt.Fprintf(out, "Remembered %s (revision %d) for this service, workspace and user. No access was granted.\n", TerminalSafe(profile.Settings.Name), profile.Revision)
		}
	}
	if err != nil {
		fmt.Fprintf(out, "Could not update or inspect the default: %s\n", TerminalSafe(err.Error()))
	}
}

func isAgentDefaultCommand(text string) bool {
	return text == "/default" || strings.HasPrefix(text, "/default ")
}
