package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/integrations"
	"github.com/takuphilchan/offgrid-llm/internal/output"
)

type cliIntegrationStatus struct {
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

type cliIntegrationsResponse struct {
	Provider     string                 `json:"provider"`
	BaseURL      string                 `json:"base_url"`
	Integrations []cliIntegrationStatus `json:"integrations"`
}

type cliIntegrationSetupResponse struct {
	Integration cliIntegrationStatus `json:"integration"`
	Setup       integrations.Setup   `json:"setup"`
}

func handleIntegrations(args []string) {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		printIntegrationsHelp()
		return
	}
	switch strings.ToLower(args[0]) {
	case "list", "ls", "check", "status":
		handleIntegrationsList(args[1:])
	case "setup", "show":
		handleIntegrationSetup(args[1:])
	case "install", "add":
		handleIntegrationInstall(args[1:])
	default:
		printError(fmt.Sprintf("Unknown integrations command: %s", args[0]))
		printIntegrationsHelp()
	}
}

func printIntegrationsHelp() {
	fmt.Println()
	fmt.Printf("  %sExternal agent integrations%s\n", brandPrimary+colorBold, colorReset)
	fmt.Printf("  %sInstall OffGrid as a native provider in supported agent runtimes%s\n\n", brandMuted, colorReset)
	fmt.Printf("  %-34s %sShow runtime readiness%s\n", "offgrid integrations list", colorDim, colorReset)
	fmt.Printf("  %-34s %sGenerate provider configuration%s\n", "offgrid integrations setup <id>", colorDim, colorReset)
	fmt.Printf("  %-34s %sInstall the first-party provider plugin%s\n", "offgrid integrations install <id>", colorDim, colorReset)
	fmt.Printf("\n  %sSupported%s  hermes, openclaw\n", brandMuted, colorReset)
	fmt.Printf("  %sOptions%s    --model <id> --base-url <url> --force\n\n", brandMuted, colorReset)
}

func handleIntegrationsList(args []string) {
	serverURL, _, _, err := integrationFlags(args)
	if err != nil {
		printError(err.Error())
		return
	}
	var response cliIntegrationsResponse
	if err := getIntegrationJSON(serverURL+"/v1/integrations", &response); err != nil {
		printError(fmt.Sprintf("Cannot inspect the OffGrid service: %v", err))
		printInfo("Start it with: offgrid serve")
		return
	}
	if outputJSON(response) {
		return
	}
	fmt.Println()
	fmt.Printf("  %sExternal agent providers%s\n", brandPrimary+colorBold, colorReset)
	fmt.Printf("  %sProvider identity: %s (OpenAI-compatible transport)%s\n\n", brandMuted, response.Provider, colorReset)
	for _, item := range response.Integrations {
		state := brandSuccess + "ready"
		if !item.Ready {
			state = brandAccent + "needs setup"
		}
		fmt.Printf("  %s%-14s%s %-18s %s%s%s\n", colorBold, item.ID, colorReset, item.Name, state, colorReset, colorReset)
		fmt.Printf("  %-16s provider=%s  model=%s  context=%d/%d\n", "", item.ProviderID, emptyFallback(item.ModelID, "none"), item.ContextWindow, item.MinimumContext)
		for _, warning := range item.Warnings {
			fmt.Printf("  %-16s %s! %s%s\n", "", brandAccent, warning, colorReset)
		}
	}
	fmt.Println()
}

func handleIntegrationSetup(args []string) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		printError("Usage: offgrid integrations setup <hermes|openclaw> [--model <id>] [--base-url <url>]")
		return
	}
	integrationID := strings.ToLower(args[0])
	serverURL, modelID, publicURL, err := integrationFlags(args[1:])
	if err != nil {
		printError(err.Error())
		return
	}
	if publicURL == "" {
		publicURL = serverURL
	}
	response, err := fetchIntegrationSetup(serverURL, integrationID, modelID, publicURL)
	if err != nil {
		printError(fmt.Sprintf("Cannot generate setup: %v", err))
		return
	}
	if outputJSON(response) {
		return
	}
	printIntegrationSetup(response)
}

func fetchIntegrationSetup(serverURL, integrationID, modelID, publicURL string) (cliIntegrationSetupResponse, error) {
	endpoint := serverURL + "/v1/integrations/" + url.PathEscape(integrationID) + "/setup"
	query := url.Values{}
	if modelID != "" {
		query.Set("model", modelID)
	}
	query.Set("base_url", publicURL)
	if encoded := query.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}
	var response cliIntegrationSetupResponse
	if err := getIntegrationJSON(endpoint, &response); err != nil {
		return cliIntegrationSetupResponse{}, err
	}
	return response, nil
}

func printIntegrationSetup(response cliIntegrationSetupResponse) {
	fmt.Println()
	fmt.Printf("  %s%s + OffGrid%s\n", brandPrimary+colorBold, response.Integration.Name, colorReset)
	fmt.Printf("  %sProvider ID%s  %s\n", brandMuted, colorReset, response.Setup.ProviderID)
	fmt.Printf("  %sInstall%s      %s\n", brandMuted, colorReset, response.Setup.Install)
	fmt.Printf("  %sConfig%s       %s\n\n", brandMuted, colorReset, response.Setup.ConfigFile)
	for key, value := range response.Setup.Environment {
		fmt.Printf("  export %s=%q\n", key, value)
	}
	fmt.Printf("\n%s\n", response.Setup.Content)
	for _, command := range response.Setup.Verify {
		fmt.Printf("  %s$%s %s\n", brandMuted, colorReset, command)
	}
	for _, warning := range response.Integration.Warnings {
		fmt.Printf("  %s! %s%s\n", brandAccent, warning, colorReset)
	}
	fmt.Println()
}

func handleIntegrationInstall(args []string) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		printError("Usage: offgrid integrations install <hermes|openclaw> [--force]")
		return
	}
	force := false
	for _, arg := range args[1:] {
		if arg == "--force" {
			force = true
		} else {
			printError(fmt.Sprintf("Unknown option: %s", arg))
			return
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		printError(fmt.Sprintf("Cannot resolve user home: %v", err))
		return
	}
	result, err := integrations.InstallPluginBundle(args[0], home, force)
	if err != nil {
		printError(err.Error())
		return
	}
	if outputJSON(result) {
		return
	}
	printSuccess(fmt.Sprintf("Prepared the native OffGrid provider at %s", result.Path))
	for _, command := range result.Next {
		fmt.Printf("  %s%s%s\n", brandMuted, command, colorReset)
	}
	if result.Integration == "openclaw" {
		if _, err := exec.LookPath("openclaw"); err == nil {
			printInfo("Run the displayed openclaw plugins install command to review and register the local plugin.")
		}
	}
}

func integrationFlags(args []string) (serverURL, modelID, publicURL string, err error) {
	cfg, loadErr := config.LoadWithPriority(os.Getenv("OFFGRID_CONFIG"))
	if loadErr != nil {
		return "", "", "", loadErr
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

func getIntegrationJSON(endpoint string, target any) error {
	request, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")
	response, err := httpClient.Do(request)
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

func outputJSON(value any) bool {
	if !output.JSONMode {
		return false
	}
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		printError(err.Error())
		return true
	}
	fmt.Println(string(encoded))
	return true
}

func emptyFallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
