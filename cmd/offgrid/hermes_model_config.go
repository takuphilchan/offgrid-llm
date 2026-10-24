package main

import (
	"strconv"
)

// Credentials are saved privately through the configuration transaction, not argv.
func hermesConfigurationCommands(setup cliIntegrationSetupResponse, baseURL string) [][]string {
	return [][]string{
		// Hermes bridges custom config keys to the environment, but warns about
		// unknown top-level keys unless --force is supplied.
		{"config", "set", "OFFGRID_BASE_URL", baseURL, "--force"},
		{"config", "set", "model.default", setup.Integration.ModelID},
		{"config", "set", "model.provider", "offgrid"},
		{"config", "set", "model.base_url", baseURL},
		{"config", "set", "model.context_length", strconv.Itoa(setup.Integration.ContextWindow)},
	}
}
