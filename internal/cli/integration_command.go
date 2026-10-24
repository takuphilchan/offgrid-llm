package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/integrations"
)

// IntegrationDependencies keep plugin installation and local process discovery
// at the OS boundary. Listing and setup use the injected HTTP client.
type IntegrationDependencies struct {
	Managed  func(context.Context, string, string, []string) error
	HomeDir  func() (string, error)
	Install  func(string, string, bool) (integrations.InstallResult, error)
	LookPath func(string) (string, error)
}

type IntegrationStatus = integrations.ServiceStatus
type IntegrationsResponse = integrations.ListResponse
type IntegrationSetupResponse = integrations.SetupResponse

func integrationCommand(ctx context.Context, args []string, env Environment) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		printIntegrationHelp(env)
		return nil
	}
	switch strings.ToLower(args[0]) {
	case "status", "check":
		if len(args) < 2 || strings.HasPrefix(args[1], "-") {
			return integrationList(ctx, args[1:], env)
		}
		if env.Integrations.Managed == nil {
			return fmt.Errorf("managed integration adapter unavailable")
		}
		return env.Integrations.Managed(ctx, "status", args[1], args[2:])
	case "configure", "test", "run":
		if len(args) < 2 || strings.HasPrefix(args[1], "-") {
			return fmt.Errorf("usage: offgrid integrations %s <id> [options]", args[0])
		}
		if env.Integrations.Managed == nil {
			return fmt.Errorf("managed integration adapter unavailable")
		}
		return env.Integrations.Managed(ctx, args[0], args[1], args[2:])
	case "list", "ls":
		return integrationList(ctx, args[1:], env)
	case "setup", "show":
		return integrationSetup(ctx, args[1:], env)
	case "install", "add":
		return integrationInstall(args[1:], env)
	default:
		return fmt.Errorf("unknown integrations command: %s", args[0])
	}
}

func RunIntegrationCommand(ctx context.Context, args []string, env Environment) error {
	return integrationCommand(ctx, args, env)
}

func printIntegrationHelp(env Environment) {
	s := env.Style
	w := env.Stdout
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  %sExternal agent integrations%s\n", s.Primary+s.Bold, s.Reset)
	fmt.Fprintf(w, "  %sInstall OffGrid as a native provider in supported agent runtimes%s\n\n", s.Muted, s.Reset)
	fmt.Fprintf(w, "  %-34s %sShow service/model compatibility%s\n", "offgrid integrations list", s.Dim, s.Reset)
	fmt.Fprintf(w, "  %-34s %sGenerate provider configuration%s\n", "offgrid integrations setup <id>", s.Dim, s.Reset)
	fmt.Fprintf(w, "  %-34s %sInstall the first-party provider plugin%s\n", "offgrid integrations install <id>", s.Dim, s.Reset)
	fmt.Fprintf(w, "  %-34s %sConfigure and verify a host integration%s\n", "offgrid integrations configure <id>", s.Dim, s.Reset)
	fmt.Fprintf(w, "  %-34s %sInspect host preflight without inference%s\n", "offgrid integrations status <id>", s.Dim, s.Reset)
	fmt.Fprintf(w, "  %-34s %sVerify a model reply%s\n", "offgrid integrations test <id>", s.Dim, s.Reset)
	fmt.Fprintf(w, "  %-34s %sRun the external agent%s\n", "offgrid integrations run <id> [task]", s.Dim, s.Reset)
	fmt.Fprintf(w, "\n  %sDiscover integration IDs with offgrid integrations list%s\n", s.Muted, s.Reset)
	fmt.Fprintf(w, "  %sOptions%s    --model <id> --base-url <url> --force\n\n", s.Muted, s.Reset)
	fmt.Fprintln(w, "  Experimental run-only file verification: --verify-contract <file.json>")
	fmt.Fprintln(w, "  Hermes hook bundle (explicit install/enable): offgrid integrations install hermes-verifier")
}

func integrationOptions(args []string, env Environment) (integrations.HostOptions, error) {
	if env.LocalAdmin.LoadWithPriority == nil || env.Getenv == nil {
		return integrations.HostOptions{}, fmt.Errorf("local configuration adapter unavailable")
	}
	cfg, err := env.LocalAdmin.LoadWithPriority(env.Getenv("OFFGRID_CONFIG"))
	if err != nil {
		return integrations.HostOptions{}, err
	}
	if cfg == nil {
		return integrations.HostOptions{}, fmt.Errorf("local configuration unavailable")
	}
	server := env.Getenv("OFFGRID_SERVER_URL")
	if server == "" {
		server = fmt.Sprintf("http://127.0.0.1:%d", cfg.ServerPort)
	}
	o, _, err := integrations.ParseHostOptions(args, integrations.HostOptions{ServerURL: server, APIKey: env.Getenv("OFFGRID_API_KEY")}, false)
	return o, err
}

func integrationClient(o integrations.HostOptions, env Environment) (*integrations.ServiceClient, error) {
	return integrations.NewServiceClient(o.ServerURL, o.APIKey, env.HTTP)
}

func integrationList(ctx context.Context, args []string, env Environment) error {
	o, err := integrationOptions(args, env)
	if err != nil {
		return err
	}
	client, err := integrationClient(o, env)
	if err != nil {
		return err
	}
	response, err := client.List(ctx, o.ModelID)
	if err != nil {
		return err
	}
	if env.Options.JSON {
		return localJSON(env, response)
	}
	fmt.Fprintln(env.Stdout, "\n  External agents: service/model compatibility (host installation and workflow unverified)")
	for _, item := range response.Integrations {
		state := "compatible"
		if !item.Ready {
			state = "blocked"
		}
		fmt.Fprintf(env.Stdout, "  %-14s %-18s %s\n", item.ID, item.Name, state)
		fmt.Fprintf(env.Stdout, "    model=%s context=%d minimum=%d\n", emptyIntegrationFallback(item.ModelID, "none"), item.ContextWindow, item.MinimumContext)
		for _, warning := range item.Warnings {
			fmt.Fprintf(env.Stdout, "    %s\n", warning)
		}
	}
	return nil
}

func integrationSetup(ctx context.Context, args []string, env Environment) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return fmt.Errorf("usage: offgrid integrations setup <id> [--model <id>] [--base-url <url>]")
	}
	o, err := integrationOptions(args[1:], env)
	if err != nil {
		return err
	}
	client, err := integrationClient(o, env)
	if err != nil {
		return err
	}
	response, err := client.Setup(ctx, strings.ToLower(args[0]), o.ModelID, o.Endpoint())
	if err != nil {
		return err
	}
	if env.Options.JSON {
		return localJSON(env, response)
	}
	printIntegrationSetup(response, env)
	return nil
}

func printIntegrationSetup(response IntegrationSetupResponse, env Environment) {
	s := env.Style
	w := env.Stdout
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  %s%s + OffGrid%s\n", s.Primary+s.Bold, response.Integration.Name, s.Reset)
	fmt.Fprintf(w, "  %sProvider ID%s  %s\n", s.Muted, s.Reset, response.Setup.ProviderID)
	fmt.Fprintf(w, "  %sInstall%s      %s\n", s.Muted, s.Reset, response.Setup.Install)
	fmt.Fprintf(w, "  %sConfig%s       %s\n\n", s.Muted, s.Reset, response.Setup.ConfigFile)
	for key, value := range response.Setup.Environment {
		fmt.Fprintf(w, "  export %s=%q\n", key, value)
	}
	fmt.Fprintf(w, "\n%s\n", response.Setup.Content)
	for _, command := range response.Setup.Verify {
		fmt.Fprintf(w, "  %s$%s %s\n", s.Muted, s.Reset, command)
	}
	for _, warning := range response.Integration.Warnings {
		fmt.Fprintf(w, "  %s! %s%s\n", s.Accent, warning, s.Reset)
	}
	fmt.Fprintln(w)
}

func integrationInstall(args []string, env Environment) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return fmt.Errorf("usage: offgrid integrations install <id> [--force]")
	}
	force := false
	for _, arg := range args[1:] {
		if arg == "--force" {
			force = true
		} else {
			return fmt.Errorf("unknown option: %s", arg)
		}
	}
	if env.Integrations.HomeDir == nil || env.Integrations.Install == nil || env.Integrations.LookPath == nil {
		return fmt.Errorf("integration installation adapter unavailable")
	}
	home, err := env.Integrations.HomeDir()
	if err != nil {
		return fmt.Errorf("cannot resolve user home: %w", err)
	}
	result, err := env.Integrations.Install(args[0], home, force)
	if err != nil {
		return err
	}
	if env.Options.JSON {
		return localJSON(env, result)
	}
	s := env.Style
	w := env.Stdout
	Presenter{Stdout: w, Style: s}.Success(fmt.Sprintf("Prepared the native OffGrid provider at %s", result.Path))
	for _, command := range result.Next {
		fmt.Fprintf(w, "  %s%s%s\n", s.Muted, command, s.Reset)
	}
	return nil
}

func emptyIntegrationFallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
