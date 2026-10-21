package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MCPDependencies are local CLI file capabilities, not model tools. The
// process adapter supplies the installed path and tests supply a disposable
// directory, so command parsing never silently writes to a real profile.
type MCPDependencies struct {
	ReadFile  func(string) ([]byte, error)
	MkdirAll  func(string, os.FileMode) error
	WriteFile func(string, []byte, os.FileMode) error
	Now       func() time.Time
}

func legacyMCPCommand(args []string, env Environment) error {
	cfg, err := localConfig(env)
	if err != nil {
		return err
	}
	path := filepath.Join(cfg.DataDir, "tools.json")
	s := env.Style
	w := env.Stdout
	if len(args) == 0 {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "  %s◈ MCP Servers%s\n", s.Primary+s.Bold, s.Reset)
		fmt.Fprintf(w, "  %sConnect external tools via Model Context Protocol%s\n", s.Dim, s.Reset)
		fmt.Fprintln(w)
		fmt.Fprintf(w, "  %sUsage%s  offgrid agent mcp <command>\n", s.Dim, s.Reset)
		fmt.Fprintln(w)
		fmt.Fprintf(w, "  %sCommands%s\n", s.Primary, s.Reset)
		fmt.Fprintf(w, "    %-20s %sAdd an MCP server%s\n", "add <url>", s.Dim, s.Reset)
		fmt.Fprintf(w, "    %-20s %sList configured servers%s\n", "list", s.Dim, s.Reset)
		fmt.Fprintf(w, "    %-20s %sRemove a server%s\n", "remove <name>", s.Dim, s.Reset)
		fmt.Fprintf(w, "    %-20s %sEnable a server%s\n", "enable <name>", s.Dim, s.Reset)
		fmt.Fprintf(w, "    %-20s %sDisable a server%s\n", "disable <name>", s.Dim, s.Reset)
		fmt.Fprintln(w)
		fmt.Fprintf(w, "  %sOptions%s\n", s.Primary, s.Reset)
		fmt.Fprintf(w, "    %-20s %sServer name (auto-derived from URL)%s\n", "--name <name>", s.Dim, s.Reset)
		fmt.Fprintf(w, "    %-20s %sAPI key for authentication%s\n", "--api-key <key>", s.Dim, s.Reset)
		fmt.Fprintln(w)
		fmt.Fprintf(w, "  %sExamples%s\n", s.Primary, s.Reset)
		fmt.Fprintf(w, "    %s$%s offgrid agent mcp add http://localhost:3100 --name my-tools\n", s.Dim, s.Reset)
		fmt.Fprintf(w, "    %s$%s offgrid agent mcp list\n", s.Dim, s.Reset)
		fmt.Fprintf(w, "    %s$%s offgrid agent mcp disable my-tools\n", s.Dim, s.Reset)
		fmt.Fprintln(w)
		return nil
	}
	deps := env.LegacyAgent.MCP
	if deps.ReadFile == nil || deps.MkdirAll == nil || deps.WriteFile == nil || deps.Now == nil {
		return fmt.Errorf("agent MCP file adapters unavailable")
	}
	load := func() (map[string]interface{}, error) {
		data, err := deps.ReadFile(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return map[string]interface{}{"tools": []interface{}{}, "mcp_servers": []interface{}{}}, nil
			}
			return nil, err
		}
		var value map[string]interface{}
		if err := json.Unmarshal(data, &value); err != nil {
			return nil, err
		}
		return value, nil
	}
	save := func(value map[string]interface{}) error {
		if err := deps.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		data, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return err
		}
		return deps.WriteFile(path, data, 0644)
	}
	printer := Presenter{Stderr: env.Stderr, Style: s}
	switch args[0] {
	case "add":
		if len(args) < 2 {
			printer.Error("Usage: offgrid agent mcp add <url> [--name <name>] [--api-key <key>]")
			return nil
		}
		endpoint, name, apiKey := args[1], "", ""
		for i := 2; i < len(args); i++ {
			if args[i] == "--name" && i+1 < len(args) {
				name = args[i+1]
				i++
			} else if args[i] == "--api-key" && i+1 < len(args) {
				apiKey = args[i+1]
				i++
			}
		}
		if name == "" {
			name = endpoint
			if idx := strings.Index(name, "://"); idx != -1 {
				name = name[idx+3:]
			}
			if idx := strings.Index(name, "/"); idx != -1 {
				name = name[:idx]
			}
			if idx := strings.Index(name, ":"); idx != -1 {
				name = name[:idx]
			}
			if name == "localhost" || name == "127.0.0.1" {
				name = fmt.Sprintf("mcp-%d", deps.Now().Unix()%10000)
			}
		}
		value, err := load()
		if err != nil {
			printer.Error(fmt.Sprintf("Failed to load config: %v", err))
			return nil
		}
		servers, _ := value["mcp_servers"].([]interface{})
		server := map[string]interface{}{"name": name, "url": endpoint, "enabled": true}
		if apiKey != "" {
			server["api_key"] = apiKey
		}
		value["mcp_servers"] = append(servers, server)
		if err := save(value); err != nil {
			printer.Error(fmt.Sprintf("Failed to save config: %v", err))
			return nil
		}
		fmt.Fprintln(w)
		fmt.Fprintf(w, "  %s✓ Added%s %s%s%s\n", s.Success+s.Bold, s.Reset, s.Secondary, name, s.Reset)
		fmt.Fprintf(w, "    %s%s%s\n", s.Dim, endpoint, s.Reset)
		fmt.Fprintln(w)
		fmt.Fprintf(w, "  %sRestart server to connect:%s offgrid serve\n", s.Dim, s.Reset)
		fmt.Fprintln(w)
	case "list":
		value, err := load()
		if err != nil {
			printer.Error(fmt.Sprintf("Failed to load config: %v", err))
			return nil
		}
		servers, _ := value["mcp_servers"].([]interface{})
		fmt.Fprintln(w)
		fmt.Fprintf(w, "  %s◈ MCP Servers%s\n", s.Primary+s.Bold, s.Reset)
		fmt.Fprintln(w)
		if len(servers) == 0 {
			fmt.Fprintf(w, "  %sNo servers configured%s\n", s.Dim, s.Reset)
			fmt.Fprintln(w)
			fmt.Fprintf(w, "  %sAdd one:%s offgrid agent mcp add <url>\n", s.Dim, s.Reset)
		} else {
			for _, item := range servers {
				server, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				name, _ := server["name"].(string)
				endpoint, _ := server["url"].(string)
				enabled, _ := server["enabled"].(bool)
				icon, color := "✓", s.Success
				if !enabled {
					icon, color = "○", s.Dim
				}
				fmt.Fprintf(w, "  %s%s%s %s%s%s\n", color, icon, s.Reset, s.Bold, name, s.Reset)
				fmt.Fprintf(w, "    %s%s%s\n", s.Dim, endpoint, s.Reset)
			}
		}
		fmt.Fprintln(w)
	case "remove", "enable", "disable":
		if len(args) < 2 {
			if args[0] == "remove" {
				printer.Error("Usage: offgrid agent mcp remove <name>")
			} else {
				printer.Error(fmt.Sprintf("Usage: offgrid agent mcp %s <name>", args[0]))
			}
			return nil
		}
		name := args[1]
		value, err := load()
		if err != nil {
			printer.Error(fmt.Sprintf("Failed to load config: %v", err))
			return nil
		}
		servers, _ := value["mcp_servers"].([]interface{})
		found := false
		if args[0] == "remove" {
			remaining := make([]interface{}, 0, len(servers))
			for _, item := range servers {
				server, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				serverName, _ := server["name"].(string)
				if serverName == name {
					found = true
					continue
				}
				remaining = append(remaining, server)
			}
			value["mcp_servers"] = remaining
		} else {
			for i, item := range servers {
				server, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				serverName, _ := server["name"].(string)
				if serverName == name {
					server["enabled"] = args[0] == "enable"
					servers[i] = server
					found = true
					break
				}
			}
			value["mcp_servers"] = servers
		}
		if !found {
			printer.Error(fmt.Sprintf("MCP server not found: %s", name))
			return nil
		}
		if err := save(value); err != nil {
			printer.Error(fmt.Sprintf("Failed to save config: %v", err))
			return nil
		}
		fmt.Fprintln(w)
		if args[0] == "remove" {
			fmt.Fprintf(w, "  %s✓ Removed%s %s%s%s\n", s.Success+s.Bold, s.Reset, s.Secondary, name, s.Reset)
			fmt.Fprintln(w)
			fmt.Fprintf(w, "  %s→ Restart the server to apply:%s offgrid serve\n", s.Dim, s.Reset)
		} else {
			action := "Enabled"
			if args[0] == "disable" {
				action = "Disabled"
			}
			fmt.Fprintf(w, "%sMCP Server %s:%s %s\n", s.Success+s.Bold, action, s.Reset, name)
			fmt.Fprintln(w)
			fmt.Fprintf(w, "%sRestart the server to apply:%s offgrid serve\n", s.Dim, s.Reset)
		}
		fmt.Fprintln(w)
	default:
		printer.Error(fmt.Sprintf("Unknown mcp command: %s", args[0]))
	}
	return nil
}
