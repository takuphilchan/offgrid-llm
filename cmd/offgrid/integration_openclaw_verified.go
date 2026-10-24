package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/integrations"
)

// Only explicit contracts use this path: agent exec in the reviewed upstream
// revision bypasses the global lifecycle-hook preload. Use its supported local
// CLI, not private runtime imports or a second execution after missing hooks.
func runVerifiedOpenClaw(ctx context.Context, o integrations.HostOptions, s integrations.HostState, setup integrations.SetupResponse, args []string) error {
	if err := verificationPrompt(args); err != nil {
		return err
	}
	if len(s.ConfigFiles) != 1 || !filepath.IsAbs(s.ConfigFiles[0]) {
		return fmt.Errorf("contracted OpenClaw requires an inspected absolute config path")
	}
	inspectCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	value, err := externalOutput(inspectCtx, openClawCommand, s.Executable, []string{"agents", "list", "--json"})
	if err != nil {
		return fmt.Errorf("cannot inspect the OpenClaw agent roster for isolated verification")
	}
	ids, selected, err := verifiedOpenClawAgents([]byte(value))
	if err != nil {
		return err
	}
	// Snapshot only identities, never resolved credentials or assistant context.
	// OpenClaw merges this overlay over the existing JSON5/include tree. External
	// includes plus CONFIG_READONLY keep its config writers away from saved files.
	if digest, err := integrations.ConfigurationDigest(s.ConfigFiles); err != nil || digest != s.ConfigDigest {
		return fmt.Errorf("OpenClaw config changed after inspection; inspect again before running")
	}
	root, err := os.MkdirTemp("", "offgrid-openclaw-contract-")
	if err != nil {
		return fmt.Errorf("cannot create private OpenClaw session state: %w", err)
	}
	// Keep owned session evidence for inspection. Never recursively remove a tree
	// an external agent has populated, and never copy personal session/auth stores.
	fmt.Fprintf(os.Stderr, "Contracted OpenClaw session state: %s\n", root)
	configPath := filepath.Join(root, "openclaw.json")
	entries := map[string]any{}
	for _, id := range ids {
		entries[id] = map[string]any{"workspace": o.Verification.Workspace, "agentDir": filepath.Join(root, "agents", id, "agent")}
	}
	overlay := map[string]any{
		"$include": s.ConfigFiles[0],
		"agents":   map[string]any{"defaults": map[string]any{"workspace": o.Verification.Workspace, "skipBootstrap": true}, "entries": entries},
		"session":  map[string]any{"store": filepath.Join(root, "agents", "{agentId}", "sessions", "sessions.json")},
	}
	data, err := json.Marshal(overlay)
	if err != nil {
		return err
	}
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("cannot write private OpenClaw overlay: %w", err)
	}
	includeRoots := filepath.Dir(s.ConfigFiles[0])
	if previous := os.Getenv("OPENCLAW_INCLUDE_ROOTS"); previous != "" {
		includeRoots += string(os.PathListSeparator) + previous
	}
	environment := mergedEnvironment(integrationEnvironment(o), map[string]string{
		"OPENCLAW_CONFIG_PATH": configPath, "OPENCLAW_CONFIG_READONLY": "1",
		"OPENCLAW_STATE_DIR": root, "OPENCLAW_WORKSPACE_DIR": o.Verification.Workspace,
		"OPENCLAW_AGENT_DIR":     filepath.Join(root, "agents", selected, "agent"),
		"PI_CODING_AGENT_DIR":    filepath.Join(root, "agents", selected, "agent"),
		"OPENCLAW_INCLUDE_ROOTS": includeRoots,
	})
	command := []string{"agent", "--local", "--agent", selected, "--session-id", o.Verification.RunID,
		"--model", "offgrid/" + setup.Integration.ModelID, "--message", args[0]}
	return externalCommand(ctx, verificationCommand(openClawCommand, o), s.Executable, command, environment, os.Stdin, commandOutput(), os.Stderr)
}

var verifiedAgentID = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

func verifiedOpenClawAgents(data []byte) ([]string, string, error) {
	var roster []struct {
		ID      string `json:"id"`
		Default bool   `json:"isDefault"`
	}
	if json.Unmarshal(data, &roster) != nil || len(roster) == 0 || len(roster) > 64 {
		return nil, "", fmt.Errorf("unsupported OpenClaw agent roster for verification")
	}
	var ids []string
	selected := ""
	seen := map[string]bool{}
	for _, agent := range roster {
		if !verifiedAgentID.MatchString(agent.ID) || seen[agent.ID] {
			return nil, "", fmt.Errorf("unsupported or duplicate OpenClaw agent identity")
		}
		seen[agent.ID] = true
		ids = append(ids, agent.ID)
		if agent.Default {
			if selected != "" {
				return nil, "", fmt.Errorf("ambiguous OpenClaw default agent")
			}
			selected = agent.ID
		}
	}
	if selected == "" && len(ids) == 1 {
		selected = ids[0]
	}
	if strings.TrimSpace(selected) == "" {
		return nil, "", fmt.Errorf("contracted OpenClaw needs an unambiguous configured default agent; no settings were changed")
	}
	return ids, selected, nil
}
