package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/integrations"
)

type openClawHost struct{ home string }

func newOpenClawHost(home string) *openClawHost { return &openClawHost{home} }
func (*openClawHost) Profile() integrations.Profile {
	a, _ := integrations.NewDefaultRegistry().Get("openclaw")
	return a.Profile()
}

func (h *openClawHost) Inspect(ctx context.Context, o integrations.HostOptions) (integrations.HostState, error) {
	pluginPath, _ := integrations.PluginBundlePath("openclaw", h.home)
	configPath := os.Getenv("OPENCLAW_CONFIG_PATH")
	if configPath == "" {
		root := os.Getenv("OPENCLAW_STATE_DIR")
		if root == "" {
			root = filepath.Join(h.home, ".openclaw")
		}
		configPath = filepath.Join(root, "openclaw.json")
	}
	s := integrations.HostState{PluginPath: pluginPath, Models: map[string]int{}, DynamicModels: true, ConfigFiles: []string{configPath}}
	var err error
	s.Executable, err = findOpenClawExecutable(h.home)
	if err != nil {
		if os.Getenv("OFFGRID_OPENCLAW_BIN") != "" {
			return s, err
		}
		return s, nil
	}
	s.Installed = true
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	version, err := externalOutput(ctx, openClawCommand, s.Executable, []string{"--version"})
	if err != nil {
		return s, fmt.Errorf("OpenClaw version inspection failed: %w", err)
	}
	s.Version = strings.Split(version, "\n")[0]
	if !strings.Contains(s.Version, "OpenClaw") {
		return s, fmt.Errorf("unrecognized OpenClaw version response")
	}
	location, err := externalOutput(ctx, openClawCommand, s.Executable, []string{"config", "file", "--json"})
	if err != nil {
		return s, fmt.Errorf("OpenClaw config location could not be inspected")
	}
	var file struct {
		Path string `json:"path"`
	}
	if json.Unmarshal([]byte(location), &file) != nil || !filepath.IsAbs(file.Path) {
		return s, fmt.Errorf("OpenClaw returned an unsupported config location")
	}
	s.ConfigFiles = []string{file.Path}
	s.ConfigDigest, err = integrations.ConfigurationDigest(s.ConfigFiles)
	if err != nil {
		return s, err
	}
	plugin, pluginErr := externalOutput(ctx, openClawCommand, s.Executable, []string{"plugins", "inspect", "offgrid", "--json"})
	var loaded struct {
		Plugin struct {
			RootDir string `json:"rootDir"`
			Status  string `json:"status"`
		} `json:"plugin"`
	}
	if pluginErr == nil && json.Unmarshal([]byte(plugin), &loaded) == nil && loaded.Plugin.Status == "loaded" && filepath.Clean(loaded.Plugin.RootDir) == filepath.Clean(s.PluginPath) {
		if digest, err := integrations.InspectPluginBundle("openclaw", s.PluginPath); err == nil {
			s.Plugin = true
			s.PluginDigest = digest
		}
	}
	provider, err := externalOutput(ctx, openClawCommand, s.Executable, []string{"config", "get", "models.providers.offgrid", "--json"})
	if err != nil {
		s.ConfigError = "OffGrid provider is not configured in OpenClaw"
		return s, nil
	}
	var config struct {
		BaseURL string `json:"baseUrl"`
		Models  []struct {
			ID      string `json:"id"`
			Context int    `json:"contextWindow"`
		} `json:"models"`
	}
	if json.Unmarshal([]byte(provider), &config) != nil {
		s.ConfigError = "OpenClaw returned an invalid provider config"
		return s, nil
	}
	s.BaseURL = config.BaseURL
	for _, model := range config.Models {
		if model.ID == "" || model.Context < 1 {
			s.ConfigError = "OpenClaw contains invalid OffGrid model metadata"
			return s, nil
		}
		if _, exists := s.Models[model.ID]; exists {
			s.ConfigError = "OpenClaw contains duplicate OffGrid model entries"
			return s, nil
		}
		s.Models[model.ID] = model.Context
		if s.DefaultModel == "" {
			s.DefaultModel = model.ID
		}
	}
	primary, err := externalOutput(ctx, openClawCommand, s.Executable, []string{"config", "get", "agents.defaults.model.primary"})
	if err == nil && strings.HasPrefix(primary, "offgrid/") {
		s.DefaultModel = strings.TrimPrefix(primary, "offgrid/")
	}
	return s, nil
}

func (*openClawHost) InstallRuntime(ctx context.Context, _ integrations.HostOptions) error {
	return installOpenClawRuntime(ctx)
}
func (h *openClawHost) InstallPlugin(ctx context.Context, o integrations.HostOptions, s integrations.HostState) error {
	plugin, err := integrations.InstallPluginBundle("openclaw", h.home, o.Force)
	if err != nil {
		return err
	}
	return ensureOpenClawPlugin(ctx, s.Executable, plugin.Path, o.Force)
}

func (*openClawHost) Configure(ctx context.Context, o integrations.HostOptions, s integrations.HostState, setup integrations.SetupResponse, tx *integrations.Transaction) error {
	var rendered struct {
		Models struct {
			Providers map[string]json.RawMessage `json:"providers"`
		} `json:"models"`
	}
	if err := json.Unmarshal([]byte(setup.Setup.Content), &rendered); err != nil {
		return fmt.Errorf("invalid generated OpenClaw provider")
	}
	var provider map[string]any
	if err := json.Unmarshal(rendered.Models.Providers["offgrid"], &provider); err != nil {
		return fmt.Errorf("invalid generated OffGrid provider")
	}
	if provider["baseUrl"] != o.Endpoint() {
		return fmt.Errorf("generated provider does not match the selected endpoint")
	}
	models, ok := provider["models"].([]any)
	if !ok || len(models) != 1 {
		return fmt.Errorf("generated provider must contain the selected model")
	}
	model, ok := models[0].(map[string]any)
	if !ok || model["id"] != setup.Integration.ModelID {
		return fmt.Errorf("generated provider does not match the selected model")
	}
	// The external runtime owns its credential config. Send through stdin,
	// never a command argument or public setup/status response.
	key := o.APIKey
	if key == "" {
		key = "offgrid-local"
	}
	provider["apiKey"] = key
	existing, err := readOpenClawModels(ctx, s.Executable)
	if err != nil {
		return err
	}
	client, err := integrations.NewServiceClient(o.ServerURL, o.APIKey, httpClient)
	if err != nil {
		return err
	}
	catalog, err := client.Models(ctx)
	if err != nil {
		return err
	}
	if err = mergeOpenClawModels(provider, existing, catalog); err != nil {
		return err
	}
	patch, err := json.Marshal(map[string]any{"models": map[string]any{"providers": map[string]any{"offgrid": provider}}})
	if err != nil {
		return err
	}
	for _, dry := range []bool{true, false} {
		if err = tx.VerifyUnchanged(); err != nil {
			return err
		}
		args := []string{"config", "patch", "--stdin"}
		if dry {
			args = append(args, "--dry-run")
		}
		if err = externalCommand(ctx, openClawCommand, s.Executable, args, integrationEnvironment(o), bytes.NewReader(patch), io.Discard, io.Discard); err != nil {
			return fmt.Errorf("OpenClaw config patch rejected (dry-run=%t): %w", dry, err)
		}
	}
	return nil
}

func (*openClawHost) Smoke(ctx context.Context, o integrations.HostOptions, s integrations.HostState, setup integrations.SetupResponse) error {
	var out hermesOutputTail
	args := []string{"infer", "model", "run", "--local", "--model", "offgrid/" + setup.Integration.ModelID, "--prompt", "Reply briefly to confirm the model is reachable.", "--thinking", "off", "--json"}
	if err := externalCommand(ctx, openClawCommand, s.Executable, args, integrationEnvironment(o), nil, &out, io.Discard); err != nil {
		return err
	}
	_, err := parseOpenClawInferResult([]byte(out.String()), setup.Integration.ModelID)
	return err
}

func (*openClawHost) Run(ctx context.Context, o integrations.HostOptions, s integrations.HostState, setup integrations.SetupResponse, args []string) error {
	if o.Verification != nil {
		return runVerifiedOpenClaw(ctx, o, s, setup, args)
	}
	if len(args) == 0 {
		return fmt.Errorf("usage: offgrid integrations run openclaw \"your task\" [--model <id>]")
	}
	command := []string{"agent", "exec", args[0], "--model", "offgrid/" + setup.Integration.ModelID, "--code-mode", "direct"}
	// Keep the upstream default unless the user explicitly chooses a finite
	// invocation budget. This changes no persistent agent/provider configuration.
	if len(args) > 1 {
		if len(args) != 3 || args[1] != "--timeout" {
			return fmt.Errorf("usage: offgrid openclaw run \"your task\" [--model <id>] [--timeout <seconds>]")
		}
		seconds, err := strconv.Atoi(args[2])
		if err != nil || seconds < 1 || seconds > 86400 {
			return fmt.Errorf("--timeout must be between 1 and 86400 seconds; unlimited runs are not supported")
		}
		command = append(command, "--timeout", strconv.Itoa(seconds))
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(seconds)*time.Second)
		defer cancel()
	}
	return externalCommand(ctx, verificationCommand(openClawCommand, o), s.Executable, command, integrationEnvironment(o), os.Stdin, commandOutput(), os.Stderr)
}
