package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/takuphilchan/offgrid-llm/internal/cli"
	"github.com/takuphilchan/offgrid-llm/internal/integrations"
	"github.com/takuphilchan/offgrid-llm/internal/output"
)

// Portable response aliases are shared with CLI rendering and fixture tests.
type cliIntegrationStatus = cli.IntegrationStatus
type cliIntegrationsResponse = cli.IntegrationsResponse
type cliIntegrationSetupResponse = cli.IntegrationSetupResponse

func handleIntegrations(ctx context.Context, args []string) int {
	return executeCommand(ctx, func(ctx context.Context) error {
		env := localAdminEnvironment()
		env.HTTP = httpClient
		env.Integrations = cli.IntegrationDependencies{
			Managed:  managedIntegration,
			HomeDir:  os.UserHomeDir,
			Install:  integrations.InstallPluginBundle,
			LookPath: exec.LookPath,
		}
		return cli.RunIntegrationCommand(ctx, args, env)
	})
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
