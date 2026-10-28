package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

type interactiveAgentOptions struct {
	model, profile, resume, workspace string
	folder, access                    string
	revision                          int
}

func parseInteractiveAgent(args []string) (interactiveAgentOptions, error) {
	var o interactiveAgentOptions
	seen := map[string]bool{}
	for i := 0; i < len(args); i += 2 {
		if i+1 >= len(args) || seen[args[i]] || strings.HasPrefix(args[i+1], "--") {
			return o, Usage("Unknown or incomplete option: %s", args[i])
		}
		seen[args[i]] = true
		switch args[i] {
		case "--workspace-folder":
			o.folder = args[i+1]
		case "--workspace-access":
			o.access = args[i+1]
		case "--workspace":
			if !ValidProfileID(args[i+1]) {
				return o, Usage("Select a valid approved execution session")
			}
			o.workspace = args[i+1]
		case "--model":
			o.model = args[i+1]
		case "--profile":
			o.profile = args[i+1]
		case "--resume":
			o.resume = args[i+1]
		case "--profile-revision":
			n, err := strconv.Atoi(args[i+1])
			if err != nil || n < 1 {
				return o, Usage("A positive profile revision is required")
			}
			o.revision = n
		default:
			return o, Usage("Unknown or incomplete option: %s", args[i])
		}
	}
	if o.profile != "" && !ValidProfileID(o.profile) || o.resume != "" && !ValidProfileID(o.resume) || o.revision != 0 && o.profile == "" {
		return o, Usage("Use a valid profile/conversation identity")
	}
	if o.folder != "" && o.workspace != "" {
		return o, Usage("Choose a local --workspace-folder or an already approved --workspace SESSION, not both")
	}
	if o.access != "" && o.folder == "" {
		return o, Usage("Use --workspace-access only with a new --workspace-folder")
	}
	if _, err := executionPermissions(o.access); err != nil {
		return o, err
	}
	return o, nil
}

// Interactive agent is an adapter for canonical conversations, not a second
// agent loop or independent ten-decision jobs. No client-local tools execute.
func runInteractiveAgent(ctx context.Context, args []string, env Environment) (result error) {
	stopInput := context.AfterFunc(ctx, func() {
		if input, ok := env.Stdin.(io.ReadCloser); ok {
			_ = input.Close()
		}
	})
	defer stopInput()
	options, err := parseInteractiveAgent(args)
	if err != nil {
		return err
	}
	if env.Options.JSON {
		return Usage("Use agent run or session send with --json for noninteractive work")
	}
	client, err := serviceClient(env)
	if err != nil {
		return err
	}
	if err = client.RequireCapability(ctx, "agent-conversations-v1"); err != nil {
		return err
	}
	input := bufio.NewReader(env.Stdin)
	profile, err := selectInteractiveProfile(ctx, client, options, input, env.Stdout)
	if err != nil {
		return err
	}
	if options.model != "" && options.model != profile.Settings.Model {
		return Usage("The selected model does not match this profile's pinned model. Choose a matching profile or save a new profile revision before starting work")
	}
	ctx = context.WithValue(ctx, agentSessionKey{}, agentSessionPresentation{Profile: profile, Style: env.Style, ReadLine: env.Service.ReadAgentLine})
	var owned ExecutionLease
	var demand *conversationWorkspace
	if options.workspace == "" && env.Execution.OnDemand != nil {
		available, checkErr := env.Execution.OnDemand(ctx, client)
		if checkErr != nil {
			return checkErr
		}
		if available {
			request := TaskSubmission{Folder: options.folder, Access: options.access, ExecutionRequestVersion: 1}
			if err := captureLaunchFolder(&request, env); err != nil {
				return err
			}
			demand = &conversationWorkspace{request: request, env: env}
			defer func() {
				if demand.lease != nil {
					result = errors.Join(result, closeExecutionLease(demand.lease))
				}
			}()
		}
	}
	if demand == nil && options.folder != "" {
		owned, err = openCLIWorkspace(ctx, client, options.folder, options.access, input, env.Stdout, env)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
	} else if demand == nil && options.workspace == "" && env.Execution.Available != nil {
		if available, checkErr := env.Execution.Available(ctx, client); checkErr == nil && available {
			owned, err = openCLIWorkspace(ctx, client, "", options.access, input, env.Stdout, env)
			if err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				fmt.Fprintln(env.Stdout, "No local workspace attached. Conversation continues without a local file/command grant.")
			}
		}
	}
	if owned != nil {
		defer func() { result = errors.Join(result, closeExecutionLease(owned)) }()
		options.workspace = owned.Resource().Reference.Session
		fmt.Fprintf(env.Stdout, "Workspace session: %s (ends when this CLI host closes)\n", TerminalSafe(options.workspace))
	}
	name := options.resume
	if name == "" {
		name = "agent-" + uuid.NewString()
		model := profile.Settings.Model
		if options.model != "" {
			model = options.model
		}
		if err = RunSessionService(ctx, client, []string{"create", name, "--model", model}, io.Discard, false, env.Service.RenderStream); err != nil {
			return fmt.Errorf("%w; inspect conversation %s before retrying creation", err, name)
		}
	}
	chatArgs := []string{"chat", name, "--profile", profile.ID, "--profile-revision", strconv.Itoa(profile.Revision)}
	if options.model != "" {
		chatArgs = append(chatArgs, "--model", options.model)
	}
	if options.workspace != "" && options.workspace != "none" {
		chatArgs = append(chatArgs, "--workspace", options.workspace)
	}
	if demand != nil {
		return runConversationChatWithAccess(ctx, client, chatArgs, input, env.Stdout, false, env.Service.RenderStream, demand)
	}
	if env.Service.Chat != nil {
		return env.Service.Chat(ctx, client, chatArgs, input, env.Stdout, false)
	}
	return RunConversationChat(ctx, client, chatArgs, input, env.Stdout, false, env.Service.RenderStream)
}

func selectInteractiveProfile(ctx context.Context, client *serviceclient.Client, o interactiveAgentOptions, input *bufio.Reader, out io.Writer) (agents.AgentProfile, error) {
	if o.profile != "" {
		var p agents.AgentProfile
		path := "/api/v2/agent-profiles/" + url.PathEscape(o.profile)
		if o.revision > 0 {
			path += "?revision=" + strconv.Itoa(o.revision)
		}
		if err := client.JSON(ctx, http.MethodGet, path, nil, &p); err != nil {
			return p, err
		}
		if p.ID != o.profile || p.Archived || p.Revision < 1 || o.revision > 0 && o.revision != p.Revision {
			return p, fmt.Errorf("The service returned an unavailable or mismatched profile")
		}
		return p, nil
	}
	var catalog struct {
		Profiles []agents.AgentProfile `json:"profiles"`
	}
	if err := client.JSON(ctx, http.MethodGet, "/api/v2/agent-profiles", nil, &catalog); err != nil {
		return agents.AgentProfile{}, err
	}
	profiles := []agents.AgentProfile{}
	for _, p := range catalog.Profiles {
		if p.Archived {
			continue
		}
		if !ValidProfileID(p.ID) || p.Revision < 1 {
			return agents.AgentProfile{}, fmt.Errorf("The service returned an invalid profile")
		}
		profiles = append(profiles, p)
	}
	if len(profiles) == 0 {
		return setupInteractiveProfile(ctx, client, o.model, input, out)
	}
	if len(profiles) == 1 {
		return profiles[0], nil
	}
	fmt.Fprintln(out, "Choose an agent profile:")
	for i, p := range profiles {
		fmt.Fprintf(out, "  %d. %s - %s (revision %d)\n", i+1, TerminalSafe(p.Settings.Name), TerminalSafe(p.Settings.Model), p.Revision)
	}
	fmt.Fprint(out, "Profile number: ")
	line, err := input.ReadString('\n')
	if err != nil {
		return agents.AgentProfile{}, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil || n < 1 || n > len(profiles) {
		return agents.AgentProfile{}, Usage("Choose a listed profile number or use --profile ID")
	}
	return profiles[n-1], nil
}

func setupInteractiveProfile(ctx context.Context, client *serviceclient.Client, model string, input *bufio.Reader, out io.Writer) (agents.AgentProfile, error) {
	var catalog struct {
		Data []struct {
			ID   string `json:"id"`
			Type string `json:"type"`
		} `json:"data"`
	}
	if err := client.JSON(ctx, http.MethodGet, "/v1/models", nil, &catalog); err != nil {
		return agents.AgentProfile{}, err
	}
	models := []string{}
	for _, m := range catalog.Data {
		if m.ID != "" && (m.Type == "" || m.Type == "chat") {
			models = append(models, m.ID)
		}
	}
	if len(models) == 0 {
		return agents.AgentProfile{}, fmt.Errorf("No installed chat model is available; no model was downloaded")
	}
	if model == "" {
		fmt.Fprintln(out, "Choose an installed model for a new General profile:")
		for i, id := range models {
			fmt.Fprintf(out, "  %d. %s\n", i+1, TerminalSafe(id))
		}
		fmt.Fprint(out, "Model number: ")
		line, err := input.ReadString('\n')
		if err != nil {
			return agents.AgentProfile{}, err
		}
		n, err := strconv.Atoi(strings.TrimSpace(line))
		if err != nil || n < 1 || n > len(models) {
			return agents.AgentProfile{}, Usage("Choose a listed model number")
		}
		model = models[n-1]
	} else {
		found := false
		for _, id := range models {
			found = found || id == model
		}
		if !found {
			return agents.AgentProfile{}, fmt.Errorf("Selected model is not installed")
		}
	}
	fmt.Fprintf(out, "Create General with %s and a 30-decision budget? No file, terminal or computer authority is granted. [y/N] ", TerminalSafe(model))
	line, err := input.ReadString('\n')
	if err != nil {
		return agents.AgentProfile{}, err
	}
	if strings.ToLower(strings.TrimSpace(line)) != "y" && strings.ToLower(strings.TrimSpace(line)) != "yes" {
		return agents.AgentProfile{}, fmt.Errorf("Profile setup cancelled; no profile was created")
	}
	var profile agents.AgentProfile
	settings := agents.ProfileSettings{Name: "General", Model: model, MaxIterations: 30}
	err = client.JSON(ctx, http.MethodPost, "/api/v2/agent-profiles", map[string]any{"settings": settings}, &profile)
	if err != nil {
		return profile, err
	}
	if !ValidProfileID(profile.ID) || profile.Revision != 1 {
		return profile, fmt.Errorf("Profile creation was not confirmed; inspect profiles before retrying")
	}
	return profile, nil
}
