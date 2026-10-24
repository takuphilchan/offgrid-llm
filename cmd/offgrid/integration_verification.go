package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/integrations"
	"github.com/takuphilchan/offgrid-llm/internal/integrations/verification"
	"gopkg.in/yaml.v3"
)

func verificationCommand(factory externalCommandFactory, o integrations.HostOptions) externalCommandFactory {
	return func(path string, args []string) (*exec.Cmd, error) {
		cmd, err := factory(path, args)
		if err == nil && o.Verification != nil {
			cmd.Dir = o.Verification.Workspace
		}
		return cmd, err
	}
}

func verificationPrompt(args []string) error {
	if len(args) != 1 || strings.TrimSpace(args[0]) == "" || strings.HasPrefix(args[0], "-") {
		return fmt.Errorf("verification requires one quoted task; interactive/resumed runs and passthrough options are unsupported")
	}
	return nil
}

func (h *hermesHost) CheckVerification(ctx context.Context, _ integrations.HostOptions, s integrations.HostState, args []string) (string, error) {
	if err := verificationPrompt(args); err != nil {
		return "", err
	}
	if s.Version != "Hermes Agent v0.21.2 (2026.9.11) · upstream f364c197" || os.Getenv("HERMES_SAFE_MODE") == "1" {
		return "", fmt.Errorf("verification requires the reviewed Hermes 0.21.2/f364c197 hooks with safe mode disabled")
	}
	path, _ := integrations.PluginBundlePath("hermes-verifier", h.home)
	if _, err := integrations.InspectPluginBundle("hermes-verifier", path); err != nil {
		return "", fmt.Errorf("verification hooks unavailable: explicitly install offgrid integrations install hermes-verifier, then enable the plugin")
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	value, err := externalOutput(ctx, hermesCommand, s.Executable, []string{"config", "get", "plugins.enabled"})
	var enabled []string
	if err != nil || yaml.Unmarshal([]byte(value), &enabled) != nil {
		return "", fmt.Errorf("cannot verify Hermes plugin enablement")
	}
	for _, id := range enabled {
		if id == "offgrid-verifier" {
			return verification.HermesAdapter, nil
		}
	}
	return "", fmt.Errorf("explicitly enable the reviewed plugin with hermes plugins enable offgrid-verifier")
}

func (*openClawHost) CheckVerification(ctx context.Context, _ integrations.HostOptions, s integrations.HostState, args []string) (string, error) {
	if err := verificationPrompt(args); err != nil {
		return "", err
	}
	if s.Version != "OpenClaw 2026.10.4 (383c124)" {
		return "", fmt.Errorf("verification requires the reviewed OpenClaw 2026.10.4/383c124 hooks")
	}
	if _, err := integrations.InspectPluginBundle("openclaw", s.PluginPath); err != nil {
		return "", fmt.Errorf("verification requires the matching trusted OpenClaw provider bundle")
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	value, err := externalOutput(ctx, openClawCommand, s.Executable, []string{"config", "get", "plugins.entries.offgrid", "--json"})
	var entry struct {
		Enabled bool `json:"enabled"`
		Hooks   struct {
			Conversation bool  `json:"allowConversationAccess"`
			Prompt       *bool `json:"allowPromptInjection"`
		} `json:"hooks"`
	}
	if err != nil || json.Unmarshal([]byte(value), &entry) != nil || !entry.Enabled || !entry.Hooks.Conversation || (entry.Hooks.Prompt != nil && !*entry.Hooks.Prompt) {
		return "", fmt.Errorf("verification requires an explicitly enabled OffGrid plugin with hooks.allowConversationAccess=true and prompt hooks permitted; review OpenClaw hook permissions first (nothing was changed)")
	}
	return verification.OpenClawAdapter, nil
}
