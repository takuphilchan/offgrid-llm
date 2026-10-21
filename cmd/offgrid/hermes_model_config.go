package main

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Status checks the model Hermes will actually run, not an unrelated first
// model in OffGrid's catalog. A stale configured context must not appear ready.
func readHermesModelConfig(executable string) (string, int, error) {
	provider, err := readHermesConfigValue(executable, "model.provider")
	if err != nil {
		return "", 0, fmt.Errorf("cannot inspect Hermes provider: %w", err)
	}
	if provider != "offgrid" {
		return "", 0, fmt.Errorf("Hermes currently uses provider %q, not offgrid", provider)
	}
	model, err := readHermesConfigValue(executable, "model.default")
	if err != nil || model == "" {
		return "", 0, fmt.Errorf("cannot inspect Hermes selected model; run offgrid hermes install --model <installed-id>")
	}
	contextText, err := readHermesConfigValue(executable, "model.context_length")
	if err != nil {
		return "", 0, fmt.Errorf("cannot inspect Hermes configured context: %w", err)
	}
	contextWindow, err := strconv.Atoi(contextText)
	if err != nil || contextWindow < 1 {
		return "", 0, fmt.Errorf("Hermes configured context is invalid")
	}
	return model, contextWindow, nil
}

func readHermesConfigValue(executable, path string) (string, error) {
	args := []string{"config", "get", path}
	if err := validateIntegrationArgs("Hermes", append([]string{executable}, args...)); err != nil {
		return "", err
	}
	command := hermesExecCommand(executable, args...)
	var stdout bytes.Buffer
	command.Stdout, command.Stderr = &stdout, io.Discard
	if err := runIntegrationProcess(command); err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout.String()), nil
}

func hermesStatusAdvice(status hermesStatusResult) string {
	if status.Error != "" {
		return status.Error
	}
	if !status.Installed || !status.Plugin || !status.Service || status.Runtime == nil {
		return "run offgrid hermes install after starting the OffGrid service"
	}
	return hermesReadinessAdvice(*status.Runtime)
}

func hermesReadinessAdvice(status cliIntegrationStatus) string {
	if status.ContextWindow < status.MinimumContext {
		return fmt.Sprintf("Hermes requires an effective OffGrid context of at least %d tokens (65536 recommended); this model currently receives %d. Choose a model and memory/KV-cache configuration that actually allocates 64K. Increasing the configured ceiling alone may not help; change service settings only if RAM/VRAM allows it. Do not claim a larger Hermes context than OffGrid actually allocates", status.MinimumContext, status.ContextWindow)
	}
	if status.ModelID == "" {
		return "install a suitable chat model in OffGrid, then retry offgrid hermes install"
	}
	return "inspect offgrid integrations list, then retry offgrid hermes install"
}

func hermesConfigurationCommands(setup cliIntegrationSetupResponse, apiKey, baseURL string) [][]string {
	return [][]string{
		{"config", "set", "OFFGRID_API_KEY", apiKey},
		// Hermes bridges custom config keys to the environment, but warns about
		// unknown top-level keys unless --force is supplied.
		{"config", "set", "OFFGRID_BASE_URL", baseURL, "--force"},
		{"config", "set", "model.default", setup.Integration.ModelID},
		{"config", "set", "model.provider", "offgrid"},
		{"config", "set", "model.base_url", baseURL},
		{"config", "set", "model.context_length", strconv.Itoa(setup.Integration.ContextWindow)},
	}
}
