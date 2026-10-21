package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/integrations"
)

// IntegrationDependencies keep plugin installation and local process discovery
// at the OS boundary. Listing and setup use the injected HTTP client.
type IntegrationDependencies struct {
	HomeDir  func() (string, error)
	Install  func(string, string, bool) (integrations.InstallResult, error)
	LookPath func(string) (string, error)
}

type IntegrationStatus struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	ProviderID     string   `json:"provider_id"`
	Transport      string   `json:"transport"`
	Ready          bool     `json:"ready"`
	Status         string   `json:"status"`
	ModelID        string   `json:"model_id"`
	ContextWindow  int      `json:"context_window"`
	MinimumContext int      `json:"minimum_context"`
	Warnings       []string `json:"warnings"`
}

type IntegrationsResponse struct {
	Provider     string              `json:"provider"`
	BaseURL      string              `json:"base_url"`
	Integrations []IntegrationStatus `json:"integrations"`
}

type IntegrationSetupResponse struct {
	Integration IntegrationStatus  `json:"integration"`
	Setup       integrations.Setup `json:"setup"`
}

func integrationCommand(ctx context.Context, args []string, env Environment) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		printIntegrationHelp(env)
		return nil
	}
	switch strings.ToLower(args[0]) {
	case "list", "ls", "check", "status":
		return integrationList(ctx, args[1:], env)
	case "setup", "show":
		return integrationSetup(ctx, args[1:], env)
	case "install", "add":
		return integrationInstall(args[1:], env)
	default:
		integrationError(env, fmt.Sprintf("Unknown integrations command: %s", args[0]))
		printIntegrationHelp(env)
		return nil
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
	fmt.Fprintf(w, "  %-34s %sShow runtime readiness%s\n", "offgrid integrations list", s.Dim, s.Reset)
	fmt.Fprintf(w, "  %-34s %sGenerate provider configuration%s\n", "offgrid integrations setup <id>", s.Dim, s.Reset)
	fmt.Fprintf(w, "  %-34s %sInstall the first-party provider plugin%s\n", "offgrid integrations install <id>", s.Dim, s.Reset)
	fmt.Fprintf(w, "\n  %sSupported%s  hermes, openclaw\n", s.Muted, s.Reset)
	fmt.Fprintf(w, "  %sOptions%s    --model <id> --base-url <url> --force\n\n", s.Muted, s.Reset)
}

func integrationError(env Environment, message string) {
	Presenter{Stderr: env.Stderr, Style: env.Style}.Error(message)
}

func integrationFlags(args []string, env Environment) (serverURL, modelID, publicURL string, err error) {
	if env.LocalAdmin.LoadWithPriority == nil || env.Getenv == nil {
		return "", "", "", fmt.Errorf("local configuration adapter unavailable")
	}
	cfg, err := env.LocalAdmin.LoadWithPriority(env.Getenv("OFFGRID_CONFIG"))
	if err != nil {
		return "", "", "", err
	}
	if cfg == nil {
		return "", "", "", fmt.Errorf("local configuration unavailable")
	}
	serverURL = fmt.Sprintf("http://127.0.0.1:%d", cfg.ServerPort)
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--server":
			index++
			if index >= len(args) {
				return "", "", "", fmt.Errorf("--server requires a URL")
			}
			serverURL = strings.TrimRight(args[index], "/")
		case "--model":
			index++
			if index >= len(args) {
				return "", "", "", fmt.Errorf("--model requires an ID")
			}
			modelID = args[index]
		case "--base-url":
			index++
			if index >= len(args) {
				return "", "", "", fmt.Errorf("--base-url requires a URL")
			}
			publicURL = args[index]
		default:
			return "", "", "", fmt.Errorf("unknown option: %s", args[index])
		}
	}
	return serverURL, modelID, publicURL, nil
}

func integrationJSON(ctx context.Context, endpoint string, target any, env Environment) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	response, err := env.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var payload struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if decodeErr := json.NewDecoder(response.Body).Decode(&payload); decodeErr == nil && payload.Error.Message != "" {
			return fmt.Errorf("%s", payload.Error.Message)
		}
		return fmt.Errorf("service returned HTTP %d", response.StatusCode)
	}
	return json.NewDecoder(response.Body).Decode(target)
}

func integrationList(ctx context.Context, args []string, env Environment) error {
	serverURL, _, _, err := integrationFlags(args, env)
	if err != nil {
		integrationError(env, err.Error())
		return nil
	}
	var response IntegrationsResponse
	if err := integrationJSON(ctx, serverURL+"/v1/integrations", &response, env); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		integrationError(env, fmt.Sprintf("Cannot inspect the OffGrid service: %v", err))
		Presenter{Stdout: env.Stdout, Style: env.Style}.Info("Start it with: offgrid serve")
		return nil
	}
	if env.Options.JSON {
		return localJSON(env, response)
	}
	s := env.Style
	w := env.Stdout
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  %sExternal agent providers%s\n", s.Primary+s.Bold, s.Reset)
	fmt.Fprintf(w, "  %sProvider identity: %s (OpenAI-compatible transport)%s\n\n", s.Muted, response.Provider, s.Reset)
	for _, item := range response.Integrations {
		state := s.Success + "ready"
		if !item.Ready {
			state = s.Accent + "needs setup"
		}
		fmt.Fprintf(w, "  %s%-14s%s %-18s %s%s%s\n", s.Bold, item.ID, s.Reset, item.Name, state, s.Reset, s.Reset)
		fmt.Fprintf(w, "  %-16s provider=%s  model=%s  context=%d/%d\n", "", item.ProviderID, emptyIntegrationFallback(item.ModelID, "none"), item.ContextWindow, item.MinimumContext)
		for _, warning := range item.Warnings {
			fmt.Fprintf(w, "  %-16s %s! %s%s\n", "", s.Accent, warning, s.Reset)
		}
	}
	fmt.Fprintln(w)
	return nil
}

func integrationSetup(ctx context.Context, args []string, env Environment) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		integrationError(env, "Usage: offgrid integrations setup <hermes|openclaw> [--model <id>] [--base-url <url>]")
		return nil
	}
	id := strings.ToLower(args[0])
	serverURL, modelID, publicURL, err := integrationFlags(args[1:], env)
	if err != nil {
		integrationError(env, err.Error())
		return nil
	}
	if publicURL == "" {
		publicURL = serverURL
	}
	endpoint := serverURL + "/v1/integrations/" + url.PathEscape(id) + "/setup"
	query := url.Values{}
	if modelID != "" {
		query.Set("model", modelID)
	}
	query.Set("base_url", publicURL)
	if encoded := query.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}
	var response IntegrationSetupResponse
	if err := integrationJSON(ctx, endpoint, &response, env); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		integrationError(env, fmt.Sprintf("Cannot generate setup: %v", err))
		return nil
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
		integrationError(env, "Usage: offgrid integrations install <hermes|openclaw> [--force]")
		return nil
	}
	force := false
	for _, arg := range args[1:] {
		if arg == "--force" {
			force = true
		} else {
			integrationError(env, fmt.Sprintf("Unknown option: %s", arg))
			return nil
		}
	}
	if env.Integrations.HomeDir == nil || env.Integrations.Install == nil || env.Integrations.LookPath == nil {
		return fmt.Errorf("integration installation adapter unavailable")
	}
	home, err := env.Integrations.HomeDir()
	if err != nil {
		integrationError(env, fmt.Sprintf("Cannot resolve user home: %v", err))
		return nil
	}
	result, err := env.Integrations.Install(args[0], home, force)
	if err != nil {
		integrationError(env, err.Error())
		return nil
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
	if result.Integration == "openclaw" {
		if _, err := env.Integrations.LookPath("openclaw"); err == nil {
			Presenter{Stdout: w, Style: s}.Info("Run the displayed openclaw plugins install command to review and register the local plugin.")
		}
	}
	return nil
}

func emptyIntegrationFallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
