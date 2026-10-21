package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/cli"
	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/integrations"
	"github.com/takuphilchan/offgrid-llm/internal/output"
)

// Hermes and OpenClaw still use these typed response aliases until their
// process-launching adapters move in the next task.
type cliIntegrationStatus = cli.IntegrationStatus
type cliIntegrationsResponse = cli.IntegrationsResponse
type cliIntegrationSetupResponse = cli.IntegrationSetupResponse

func handleIntegrations(ctx context.Context, args []string) int {
	return executeCommand(ctx, func(ctx context.Context) error {
		env := localAdminEnvironment()
		env.HTTP = httpClient
		env.Integrations = cli.IntegrationDependencies{
			HomeDir:  os.UserHomeDir,
			Install:  integrations.InstallPluginBundle,
			LookPath: exec.LookPath,
		}
		return cli.RunIntegrationCommand(ctx, args, env)
	})
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
