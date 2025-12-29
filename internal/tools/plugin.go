// Package tools provides a plugin system for custom tools
package tools

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// PluginType represents the type of plugin
type PluginType string

const (
	PluginTypeTool       PluginType = "tool"
	PluginTypeAgent      PluginType = "agent"
	PluginTypeMiddleware PluginType = "middleware"
	PluginTypeFormatter  PluginType = "formatter"
)

// Plugin represents a loadable plugin
type Plugin struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Version     string            `json:"version"`
	Author      string            `json:"author"`
	Type        PluginType        `json:"type"`
	EntryPoint  string            `json:"entry_point"` // Script or binary to run
	Language    string            `json:"language"`    // python, node, go, shell
	Enabled     bool              `json:"enabled"`
	Config      map[string]string `json:"config"` // Plugin-specific configuration
	Path        string            `json:"path"`   // Directory containing the plugin

	// Tool-specific fields
	ToolSchema *ToolSchema `json:"tool_schema,omitempty"`
}

// ToolSchema defines the interface for a tool plugin
type ToolSchema struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  []ToolParameter `json:"parameters"`
}

// ToolParameter defines a parameter for a tool
type ToolParameter struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"` // string, number, boolean, array, object
	Description string   `json:"description"`
	Required    bool     `json:"required"`
	Default     any      `json:"default,omitempty"`
	Enum        []string `json:"enum,omitempty"` // For restricted values
}

// PluginManager manages plugin discovery, loading, and execution
type PluginManager struct {
	plugins    map[string]*Plugin
	pluginsDir string
	mu         sync.RWMutex
}

// NewPluginManager creates a new plugin manager
func NewPluginManager(pluginsDir string) *PluginManager {
	pm := &PluginManager{
		plugins:    make(map[string]*Plugin),
		pluginsDir: pluginsDir,
	}

	// Create plugins directory if it doesn't exist
	os.MkdirAll(pluginsDir, 0755)

	return pm
}

// ScanPlugins discovers all plugins in the plugins directory
func (pm *PluginManager) ScanPlugins() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Clear existing plugins
	pm.plugins = make(map[string]*Plugin)

	// Scan for plugin directories
	entries, err := os.ReadDir(pm.pluginsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pluginDir := filepath.Join(pm.pluginsDir, entry.Name())
		manifestPath := filepath.Join(pluginDir, "plugin.json")

		// Check for manifest
		manifestData, err := os.ReadFile(manifestPath)
		if err != nil {
			log.Printf("Plugin %s has no plugin.json: %v", entry.Name(), err)
			continue
		}

		var plugin Plugin
		if err := json.Unmarshal(manifestData, &plugin); err != nil {
			log.Printf("Invalid plugin.json for %s: %v", entry.Name(), err)
			continue
		}

		plugin.Path = pluginDir
		if plugin.ID == "" {
			plugin.ID = entry.Name()
		}

		pm.plugins[plugin.ID] = &plugin
		log.Printf("Loaded plugin: %s v%s (%s)", plugin.Name, plugin.Version, plugin.Type)
	}

	return nil
}

// ListPlugins returns all loaded plugins
func (pm *PluginManager) ListPlugins() []*Plugin {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	plugins := make([]*Plugin, 0, len(pm.plugins))
	for _, p := range pm.plugins {
		plugins = append(plugins, p)
	}
	return plugins
}

// GetPlugin returns a plugin by ID
func (pm *PluginManager) GetPlugin(id string) (*Plugin, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	plugin, exists := pm.plugins[id]
	return plugin, exists
}

// EnablePlugin enables a plugin
func (pm *PluginManager) EnablePlugin(id string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	plugin, exists := pm.plugins[id]
	if !exists {
		return fmt.Errorf("plugin %s not found", id)
	}

	plugin.Enabled = true
	return pm.savePluginConfig(plugin)
}

// DisablePlugin disables a plugin
func (pm *PluginManager) DisablePlugin(id string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	plugin, exists := pm.plugins[id]
	if !exists {
		return fmt.Errorf("plugin %s not found", id)
	}

	plugin.Enabled = false
	return pm.savePluginConfig(plugin)
}

// UpdatePluginConfig updates a plugin's configuration
func (pm *PluginManager) UpdatePluginConfig(id string, config map[string]string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	plugin, exists := pm.plugins[id]
	if !exists {
		return fmt.Errorf("plugin %s not found", id)
	}

	plugin.Config = config
	return pm.savePluginConfig(plugin)
}

// savePluginConfig saves the plugin configuration to disk
func (pm *PluginManager) savePluginConfig(plugin *Plugin) error {
	manifestPath := filepath.Join(plugin.Path, "plugin.json")
	data, err := json.MarshalIndent(plugin, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(manifestPath, data, 0644)
}

// ExecutePlugin executes a plugin with the given input
func (pm *PluginManager) ExecutePlugin(id string, input map[string]any) (string, error) {
	pm.mu.RLock()
	plugin, exists := pm.plugins[id]
	pm.mu.RUnlock()

	if !exists {
		return "", fmt.Errorf("plugin %s not found", id)
	}

	if !plugin.Enabled {
		return "", fmt.Errorf("plugin %s is disabled", id)
	}

	// Serialize input to JSON
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("failed to serialize input: %w", err)
	}

	// Build command based on language
	var cmd *exec.Cmd
	entryPoint := filepath.Join(plugin.Path, plugin.EntryPoint)

	switch plugin.Language {
	case "python":
		cmd = exec.Command("python3", entryPoint)
	case "node", "javascript":
		cmd = exec.Command("node", entryPoint)
	case "shell", "bash":
		cmd = exec.Command("bash", entryPoint)
	case "go":
		// Assume pre-compiled binary
		cmd = exec.Command(entryPoint)
	default:
		// Try to execute directly
		cmd = exec.Command(entryPoint)
	}

	// Pass input via stdin
	cmd.Stdin = strings.NewReader(string(inputJSON))
	cmd.Dir = plugin.Path

	// Add plugin config as environment variables
	cmd.Env = os.Environ()
	for k, v := range plugin.Config {
		cmd.Env = append(cmd.Env, fmt.Sprintf("PLUGIN_%s=%s", strings.ToUpper(k), v))
	}

	// Execute and capture output
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("plugin execution failed: %w\nOutput: %s", err, string(output))
	}

	return string(output), nil
}

// InstallPlugin installs a plugin from a directory or archive
func (pm *PluginManager) InstallPlugin(sourcePath string) (*Plugin, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Check if source is a directory
	info, err := os.Stat(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("source not found: %w", err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("source must be a directory")
	}

	// Read manifest
	manifestPath := filepath.Join(sourcePath, "plugin.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("no plugin.json found: %w", err)
	}

	var plugin Plugin
	if err := json.Unmarshal(manifestData, &plugin); err != nil {
		return nil, fmt.Errorf("invalid plugin.json: %w", err)
	}

	if plugin.ID == "" {
		plugin.ID = filepath.Base(sourcePath)
	}

	// Copy to plugins directory
	destPath := filepath.Join(pm.pluginsDir, plugin.ID)
	if _, err := os.Stat(destPath); err == nil {
		return nil, fmt.Errorf("plugin %s already installed", plugin.ID)
	}

	// Create destination directory
	if err := os.MkdirAll(destPath, 0755); err != nil {
		return nil, err
	}

	// Copy files
	if err := copyDir(sourcePath, destPath); err != nil {
		os.RemoveAll(destPath)
		return nil, fmt.Errorf("failed to copy plugin: %w", err)
	}

	plugin.Path = destPath
	plugin.Enabled = true
	pm.plugins[plugin.ID] = &plugin

	log.Printf("Installed plugin: %s v%s", plugin.Name, plugin.Version)
	return &plugin, nil
}

// UninstallPlugin removes a plugin
func (pm *PluginManager) UninstallPlugin(id string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	plugin, exists := pm.plugins[id]
	if !exists {
		return fmt.Errorf("plugin %s not found", id)
	}

	// Remove from disk
	if err := os.RemoveAll(plugin.Path); err != nil {
		return fmt.Errorf("failed to remove plugin: %w", err)
	}

	delete(pm.plugins, id)
	log.Printf("Uninstalled plugin: %s", id)
	return nil
}

// copyDir copies a directory recursively
func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := os.MkdirAll(dstPath, 0755); err != nil {
				return err
			}
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			data, err := os.ReadFile(srcPath)
			if err != nil {
				return err
			}
			if err := os.WriteFile(dstPath, data, 0644); err != nil {
				return err
			}
		}
	}

	return nil
}

// GetToolPlugins returns all tool-type plugins as tool definitions
func (pm *PluginManager) GetToolPlugins() []ToolDefinition {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var tools []ToolDefinition
	for _, p := range pm.plugins {
		if p.Type == PluginTypeTool && p.Enabled && p.ToolSchema != nil {
			tools = append(tools, ToolDefinition{
				Name:        p.ToolSchema.Name,
				Description: p.ToolSchema.Description,
				Parameters:  convertParameters(p.ToolSchema.Parameters),
				PluginID:    p.ID,
			})
		}
	}
	return tools
}

// ToolDefinition is used to expose plugin tools to the agent system
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
	PluginID    string                 `json:"plugin_id"`
}

// convertParameters converts tool parameters to JSON Schema format
func convertParameters(params []ToolParameter) map[string]interface{} {
	properties := make(map[string]interface{})
	required := []string{}

	for _, p := range params {
		prop := map[string]interface{}{
			"type":        p.Type,
			"description": p.Description,
		}
		if p.Default != nil {
			prop["default"] = p.Default
		}
		if len(p.Enum) > 0 {
			prop["enum"] = p.Enum
		}
		properties[p.Name] = prop

		if p.Required {
			required = append(required, p.Name)
		}
	}

	return map[string]interface{}{
		"type":       "object",
		"properties": properties,
		"required":   required,
	}
}
