package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/integrations"
)

type hermesHost struct{ home string }

func newHermesHost(home string) *hermesHost { return &hermesHost{home} }
func (*hermesHost) Profile() integrations.Profile {
	a, _ := integrations.NewDefaultRegistry().Get("hermes")
	return a.Profile()
}

func (h *hermesHost) Inspect(ctx context.Context, o integrations.HostOptions) (integrations.HostState, error) {
	state := integrations.HostState{PluginPath: integrations.HermesPluginPath(h.home), DynamicModels: true, Models: map[string]int{}, ConfigFiles: []string{filepath.Join(integrations.HermesHome(h.home), "config.yaml"), filepath.Join(integrations.HermesHome(h.home), ".env")}}
	var err error
	state.ConfigDigest, err = integrations.ConfigurationDigest(state.ConfigFiles)
	if err != nil {
		return state, err
	}
	state.Executable, err = findHermesExecutable(h.home)
	if err != nil {
		if os.Getenv("OFFGRID_HERMES_BIN") != "" {
			return state, err
		}
		return state, nil
	}
	state.Installed = true
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	version, err := inspectHermesVersion(ctx, state.Executable)
	if err != nil {
		return state, fmt.Errorf("Hermes version inspection failed: %w", err)
	}
	state.Version = strings.Split(version, "\n")[0]
	if !strings.Contains(state.Version, "Hermes") {
		return state, fmt.Errorf("unrecognized Hermes version response")
	}
	if digest, err := integrations.InspectPluginBundle("hermes", state.PluginPath); err == nil {
		state.Plugin = true
		state.PluginDigest = digest
	}
	read := func(key string) string {
		value, err := externalOutput(ctx, hermesCommand, state.Executable, []string{"config", "get", key})
		if err != nil {
			state.ConfigError = "Cannot read Hermes configuration; use managed configure to review it"
			return ""
		}
		return value
	}
	provider := read("model.provider")
	state.DefaultModel = read("model.default")
	state.BaseURL = read("model.base_url")
	contextText := read("model.context_length")
	if contextText != "" && contextText != "null" && contextText != "None" {
		state.ContextWindow, err = strconv.Atoi(contextText)
		if err != nil || state.ContextWindow < 0 {
			state.ConfigError = "Hermes context setting is invalid"
		}
	}
	if provider != "offgrid" {
		state.ConfigError = "Hermes is not configured for the OffGrid provider"
		state.DefaultModel = ""
	}
	return state, nil
}

// Hermes --version may synchronously check GitHub before exiting. Probe its
// installed executable with a private, disposable home so optional update
// telemetry cannot consume the host-inspection deadline. Only this version
// command receives the override; configuration is read from the user's home.
func inspectHermesVersion(ctx context.Context, executable string) (string, error) {
	home, err := os.MkdirTemp("", "offgrid-hermes-version-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(home)
	if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte("updates:\n  check: false\n"), 0600); err != nil {
		return "", err
	}
	var out hermesOutputTail
	env := mergedEnvironment(os.Environ(), map[string]string{"HERMES_HOME": home})
	if err := externalCommand(ctx, hermesCommand, executable, []string{"--version"}, env, nil, &out, io.Discard); err != nil {
		return "", err
	}
	return strings.TrimSpace(out.String()), nil
}

func (*hermesHost) InstallRuntime(ctx context.Context, _ integrations.HostOptions) error {
	return installHermesRuntime(ctx)
}
func (h *hermesHost) InstallPlugin(_ context.Context, o integrations.HostOptions, _ integrations.HostState) error {
	_, err := integrations.InstallPluginBundle("hermes", h.home, o.Force)
	return err
}

func (*hermesHost) Configure(ctx context.Context, o integrations.HostOptions, s integrations.HostState, setup integrations.SetupResponse, tx *integrations.Transaction) error {
	key := o.APIKey
	if key == "" {
		key = "offgrid-local"
	}
	if len(s.ConfigFiles) != 2 {
		return fmt.Errorf("Hermes configuration targets were not inspected")
	}
	if err := tx.SetEnvironment(s.ConfigFiles[1], map[string]string{"OFFGRID_API_KEY": key}); err != nil {
		return err
	}
	commands := hermesConfigurationCommands(setup, o.Endpoint())
	for _, args := range commands {
		if err := tx.VerifyUnchanged(); err != nil {
			return err
		}
		if err := externalCommand(ctx, hermesCommand, s.Executable, args, integrationEnvironment(o), nil, io.Discard, io.Discard); err != nil {
			return fmt.Errorf("Hermes setting %s could not be saved: %w", args[2], err)
		}
		if err := tx.Checkpoint(); err != nil {
			return err
		}
	}
	if o.WithBrowser {
		if err := installHermesOptionalTools(ctx); err != nil {
			return fmt.Errorf("provider configured; optional browser installation failed: %w", err)
		}
	}
	if o.RunDoctor {
		return externalCommand(ctx, hermesCommand, s.Executable, []string{"doctor"}, integrationEnvironment(o), nil, commandOutput(), os.Stderr)
	}
	return nil
}

func (*hermesHost) Smoke(ctx context.Context, o integrations.HostOptions, s integrations.HostState, setup integrations.SetupResponse) error {
	const expected = "HERMES_OFFGRID_OK"
	var out hermesOutputTail
	args := []string{"chat", "--oneshot", "--provider", "offgrid", "--model", setup.Integration.ModelID, "--toolsets", "none", "--ignore-rules", "--max-turns", "2", "-q", "Reply with exactly: " + expected}
	err := externalCommand(ctx, hermesCommand, s.Executable, args, integrationEnvironment(o), nil, &out, io.Discard)
	if err != nil {
		return err
	}
	if !containsExactOutputLine(out.String(), expected) {
		return fmt.Errorf("Hermes did not return the expected model reply")
	}
	return nil
}

func (*hermesHost) Run(ctx context.Context, o integrations.HostOptions, s integrations.HostState, setup integrations.SetupResponse, args []string) error {
	for _, arg := range args {
		if arg == "--provider" || strings.HasPrefix(arg, "--provider=") || arg == "-m" || strings.HasPrefix(arg, "--model") || strings.HasPrefix(arg, "--base-url") {
			return fmt.Errorf("managed Hermes runs require model and endpoint options before the -- separator, and use the offgrid provider")
		}
	}
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		args = append([]string{"-q", args[0]}, args[1:]...)
	}
	command := append([]string{"chat", "--provider", "offgrid", "--model", setup.Integration.ModelID}, args...)
	if o.Verification != nil {
		command = append(command, "--oneshot")
	}
	return externalCommand(ctx, verificationCommand(hermesCommand, o), s.Executable, command, integrationEnvironment(o), os.Stdin, commandOutput(), os.Stderr)
}
