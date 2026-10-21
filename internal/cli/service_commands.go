package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

// ServiceDependencies are supplied by the process adapter. The authenticated
// client remains the only route to service-owned conversations and jobs.
type ServiceDependencies struct {
	Client                  func() (*serviceclient.Client, error)
	ControlClient           func() (*serviceclient.Client, error)
	RenderStream            func(io.Writer, io.Writer, io.Reader) error
	Chat                    func(context.Context, *serviceclient.Client, []string, io.Reader, io.Writer, bool) error
	CreateArtifactFile      func(string) (ArtifactFile, error)
	OpenProfileFile         func(string) (io.ReadCloser, error)
	CreateSessionExportFile func(string) (SessionExportFile, error)
	RemoveSessionExportFile func(string) error
}

func serviceClient(env Environment) (*serviceclient.Client, error) {
	if env.Service.Client == nil {
		return nil, fmt.Errorf("authenticated service client unavailable")
	}
	return env.Service.Client()
}

func sessionServiceCommand(ctx context.Context, args []string, env Environment) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		if env.Options.JSON {
			return json.NewEncoder(env.Stdout).Encode(map[string]string{"help": SessionHelp})
		}
		_, err := fmt.Fprint(env.Stdout, SessionHelp)
		return err
	}
	client, err := serviceClient(env)
	if err != nil {
		return err
	}
	if args[0] == "chat" {
		if env.Service.Chat == nil {
			return fmt.Errorf("interactive conversation adapter unavailable")
		}
		return env.Service.Chat(ctx, client, args, env.Stdin, env.Stdout, env.Options.JSON)
	}
	return RunSessionServiceWithFiles(ctx, client, args, env.Stdout, env.Options.JSON, env.Service.RenderStream, env.Service.CreateSessionExportFile, env.Service.RemoveSessionExportFile)
}

func RunSessionCommand(ctx context.Context, args []string, env Environment) error {
	return sessionServiceCommand(ctx, args, env)
}

func sessionExportCommand(ctx context.Context, args []string, env Environment) error {
	if len(args) == 0 {
		return Usage("export-session requires NAME [--format markdown|json|txt] [--output FILE]")
	}
	if args[0] == "--help" || args[0] == "-h" {
		return sessionServiceCommand(ctx, []string{"help"}, env)
	}
	format, file := "markdown", ""
	for i := 1; i < len(args); i++ {
		if i+1 >= len(args) {
			return Usage("missing value for %s", args[i])
		}
		switch args[i] {
		case "--format":
			format = args[i+1]
		case "--output":
			file = args[i+1]
		default:
			return Usage("unknown export option %q", args[i])
		}
		i++
	}
	client, err := serviceClient(env)
	if err != nil {
		return err
	}
	return ExportServiceSessionWithFiles(ctx, client, args[0], format, file, env.Stdout, env.Options.JSON, env.Service.CreateSessionExportFile, env.Service.RemoveSessionExportFile)
}

func RunSessionExportCommand(ctx context.Context, args []string, env Environment) error {
	return sessionExportCommand(ctx, args, env)
}

func knowledgeServiceCommand(ctx context.Context, args []string, env Environment) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		if env.Options.JSON {
			return json.NewEncoder(env.Stdout).Encode(map[string]string{"usage": KnowledgeUsage})
		}
		if _, err := fmt.Fprintln(env.Stdout, KnowledgeUsage); err != nil {
			return err
		}
		_, err := fmt.Fprintln(env.Stdout, "Uses OFFGRID_SERVER_URL and OFFGRID_API_KEY. --json emits one result; Ctrl+C cancels the request.")
		return err
	}
	client, err := serviceClient(env)
	if err != nil {
		return err
	}
	result, err := RunKnowledgeService(ctx, client, args, env.Stdin, env.Stderr, env.Options.JSON)
	if err != nil {
		return err
	}
	if env.Options.JSON {
		return json.NewEncoder(env.Stdout).Encode(result)
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(env.Stdout, TerminalSafe(string(data)))
	return err
}

func RunKnowledgeCommand(ctx context.Context, args []string, env Environment) error {
	return knowledgeServiceCommand(ctx, args, env)
}

func agentServiceCommand(ctx context.Context, args []string, env Environment) error {
	if len(args) == 0 {
		return legacyAgentCommand(ctx, args, env)
	}
	if args[0] == "profiles" {
		client, err := serviceClient(env)
		if err != nil {
			return err
		}
		return RunProfileServiceWithOpen(ctx, client, args[1:], env.Stdin, env.Stdout, env.Options.JSON, env.Service.OpenProfileFile)
	}
	if args[0] == "run" {
		return taskSubmitCommand(ctx, args[1:], env)
	}
	if args[0] == "artifact" {
		return RunArtifactDownload(ctx, env.Service.Client, args[1:], env.Stdout, env.Options.JSON, env.Service.CreateArtifactFile)
	}
	if !agentControlAction(args[0]) {
		return legacyAgentCommand(ctx, args, env)
	}
	makeControlClient := env.Service.ControlClient
	if makeControlClient == nil {
		makeControlClient = env.Service.Client
	}
	if makeControlClient == nil {
		return fmt.Errorf("authenticated service client unavailable")
	}
	client, err := makeControlClient()
	if err != nil {
		return err
	}
	snapshot, err := AgentControlService(ctx, client, args)
	if err != nil {
		return err
	}
	if len(snapshot.Evidence) > 0 {
		return json.NewEncoder(env.Stdout).Encode(snapshot.Evidence)
	}
	if env.Options.JSON {
		return json.NewEncoder(env.Stdout).Encode(snapshot)
	}
	if _, err := fmt.Fprintf(env.Stdout, "Run: %s\n", TerminalSafe(snapshot.RunID)); err != nil {
		return err
	}
	RenderAgentSnapshot(env.Stdout, *snapshot)
	return nil
}

func RunAgentCommand(ctx context.Context, args []string, env Environment) error {
	return agentServiceCommand(ctx, args, env)
}

func agentControlAction(action string) bool {
	switch action {
	case "status", "approve", "deny", "cancel", "resume", "reconcile", "pause", "takeover", "steer", "reconnect", "export":
		return true
	}
	return false
}

func taskSubmitCommand(ctx context.Context, args []string, env Environment) error {
	req, wait, err := ParseTaskSubmission(args)
	if err != nil {
		return err
	}
	client, err := serviceClient(env)
	if err != nil {
		return err
	}
	if !env.Options.JSON {
		if _, err := fmt.Fprintf(env.Stderr, "Request: %s\n", req.RequestID); err != nil {
			return err
		}
	}
	snapshot, err := SubmitTask(ctx, client, req)
	if err != nil {
		return fmt.Errorf("%w (retry this submission with --request-id %s)", err, req.RequestID)
	}
	if env.Options.JSON {
		return json.NewEncoder(env.Stdout).Encode(snapshot)
	}
	if _, err := fmt.Fprintf(env.Stdout, "Run: %s\n", TerminalSafe(snapshot.RunID)); err != nil {
		return err
	}
	RenderAgentSnapshot(env.Stdout, *snapshot)
	if wait && (snapshot.Status == "running" || snapshot.Status == "pending" || snapshot.Status == "waiting_for_children") {
		if env.Service.RenderStream == nil {
			return fmt.Errorf("agent stream renderer unavailable")
		}
		response, err := client.Do(ctx, http.MethodGet, "/api/v2/jobs/"+url.PathEscape(snapshot.RunID)+"/events", "", nil)
		if err != nil {
			return err
		}
		defer response.Body.Close()
		return env.Service.RenderStream(env.Stdout, env.Stderr, response.Body)
	}
	return nil
}

// RunTaskSubmitCommand preserves the legacy command adapter while the process
// entry is moved to Execute. It does not create another service-side executor.
func RunTaskSubmitCommand(ctx context.Context, args []string, env Environment) error {
	return taskSubmitCommand(ctx, args, env)
}
