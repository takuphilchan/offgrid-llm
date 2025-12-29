// Package mcp provides Model Context Protocol server and client functionality
package mcp

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// MarketplaceServer represents an MCP server available in the marketplace
type MarketplaceServer struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	Author       string            `json:"author"`
	Version      string            `json:"version"`
	Repository   string            `json:"repository,omitempty"`
	Homepage     string            `json:"homepage,omitempty"`
	License      string            `json:"license,omitempty"`
	Category     string            `json:"category"`
	Tags         []string          `json:"tags,omitempty"`
	InstallType  string            `json:"install_type"` // npm, pip, binary, docker
	InstallCmd   string            `json:"install_cmd"`  // Command to install
	Command      string            `json:"command"`      // Command to run the server
	Args         []string          `json:"args,omitempty"`
	Env          map[string]string `json:"env,omitempty"`
	Tools        []string          `json:"tools,omitempty"`        // List of tools provided
	Resources    []string          `json:"resources,omitempty"`    // List of resources provided
	Requirements []string          `json:"requirements,omitempty"` // System requirements
	Downloads    int               `json:"downloads,omitempty"`
	Rating       float64           `json:"rating,omitempty"`
	LastUpdated  time.Time         `json:"last_updated,omitempty"`
}

// InstalledServer represents a locally installed MCP server
type InstalledServer struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	InstallType string            `json:"install_type"`
	Command     string            `json:"command"`
	Args        []string          `json:"args,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	InstalledAt time.Time         `json:"installed_at"`
	Enabled     bool              `json:"enabled"`
	ConfigPath  string            `json:"config_path,omitempty"`
}

// Marketplace manages the MCP server marketplace
type Marketplace struct {
	mu             sync.RWMutex
	dataDir        string
	installed      map[string]*InstalledServer
	availableCache []*MarketplaceServer
	cacheExpiry    time.Time
	cacheDuration  time.Duration
}

// NewMarketplace creates a new marketplace manager
func NewMarketplace(dataDir string) *Marketplace {
	m := &Marketplace{
		dataDir:       dataDir,
		installed:     make(map[string]*InstalledServer),
		cacheDuration: 1 * time.Hour,
	}
	m.loadInstalled()
	return m
}

// BuiltInServers returns a list of well-known MCP servers
func BuiltInServers() []*MarketplaceServer {
	return []*MarketplaceServer{
		{
			ID:          "filesystem",
			Name:        "Filesystem",
			Description: "Read, write, and manage files and directories",
			Author:      "Anthropic",
			Version:     "0.6.0",
			Repository:  "https://github.com/modelcontextprotocol/servers",
			Category:    "utility",
			Tags:        []string{"files", "filesystem", "io"},
			InstallType: "npm",
			InstallCmd:  "npm install -g @modelcontextprotocol/server-filesystem",
			Command:     "npx",
			Args:        []string{"-y", "@modelcontextprotocol/server-filesystem", "."},
			Tools:       []string{"read_file", "write_file", "list_directory", "create_directory", "move_file", "search_files"},
		},
		{
			ID:          "brave-search",
			Name:        "Brave Search",
			Description: "Web search using Brave Search API",
			Author:      "Anthropic",
			Version:     "0.6.0",
			Repository:  "https://github.com/modelcontextprotocol/servers",
			Category:    "search",
			Tags:        []string{"search", "web", "brave"},
			InstallType: "npm",
			InstallCmd:  "npm install -g @modelcontextprotocol/server-brave-search",
			Command:     "npx",
			Args:        []string{"-y", "@modelcontextprotocol/server-brave-search"},
			Env:         map[string]string{"BRAVE_API_KEY": ""},
			Tools:       []string{"brave_web_search", "brave_local_search"},
		},
		{
			ID:          "github",
			Name:        "GitHub",
			Description: "Interact with GitHub repositories, issues, and pull requests",
			Author:      "Anthropic",
			Version:     "0.6.0",
			Repository:  "https://github.com/modelcontextprotocol/servers",
			Category:    "development",
			Tags:        []string{"github", "git", "vcs"},
			InstallType: "npm",
			InstallCmd:  "npm install -g @modelcontextprotocol/server-github",
			Command:     "npx",
			Args:        []string{"-y", "@modelcontextprotocol/server-github"},
			Env:         map[string]string{"GITHUB_PERSONAL_ACCESS_TOKEN": ""},
			Tools:       []string{"create_or_update_file", "search_repositories", "create_repository", "get_file_contents", "push_files", "create_issue", "create_pull_request", "fork_repository", "create_branch"},
		},
		{
			ID:          "puppeteer",
			Name:        "Puppeteer",
			Description: "Browser automation with Puppeteer",
			Author:      "Anthropic",
			Version:     "0.6.0",
			Repository:  "https://github.com/modelcontextprotocol/servers",
			Category:    "automation",
			Tags:        []string{"browser", "automation", "puppeteer"},
			InstallType: "npm",
			InstallCmd:  "npm install -g @modelcontextprotocol/server-puppeteer",
			Command:     "npx",
			Args:        []string{"-y", "@modelcontextprotocol/server-puppeteer"},
			Tools:       []string{"puppeteer_navigate", "puppeteer_screenshot", "puppeteer_click", "puppeteer_fill", "puppeteer_select", "puppeteer_hover", "puppeteer_evaluate"},
		},
		{
			ID:          "memory",
			Name:        "Memory",
			Description: "Knowledge graph-based persistent memory for AI",
			Author:      "Anthropic",
			Version:     "0.6.0",
			Repository:  "https://github.com/modelcontextprotocol/servers",
			Category:    "memory",
			Tags:        []string{"memory", "knowledge", "graph"},
			InstallType: "npm",
			InstallCmd:  "npm install -g @modelcontextprotocol/server-memory",
			Command:     "npx",
			Args:        []string{"-y", "@modelcontextprotocol/server-memory"},
			Tools:       []string{"create_entities", "create_relations", "add_observations", "delete_entities", "delete_observations", "delete_relations", "read_graph", "search_nodes", "open_nodes"},
		},
		{
			ID:          "fetch",
			Name:        "Fetch",
			Description: "Fetch web content and convert to markdown",
			Author:      "Anthropic",
			Version:     "0.6.0",
			Repository:  "https://github.com/modelcontextprotocol/servers",
			Category:    "utility",
			Tags:        []string{"http", "web", "fetch"},
			InstallType: "npm",
			InstallCmd:  "npm install -g @modelcontextprotocol/server-fetch",
			Command:     "npx",
			Args:        []string{"-y", "@modelcontextprotocol/server-fetch"},
			Tools:       []string{"fetch"},
		},
		{
			ID:          "sqlite",
			Name:        "SQLite",
			Description: "Query and manage SQLite databases",
			Author:      "Anthropic",
			Version:     "0.6.0",
			Repository:  "https://github.com/modelcontextprotocol/servers",
			Category:    "database",
			Tags:        []string{"database", "sql", "sqlite"},
			InstallType: "npm",
			InstallCmd:  "npm install -g @modelcontextprotocol/server-sqlite",
			Command:     "npx",
			Args:        []string{"-y", "@modelcontextprotocol/server-sqlite"},
			Tools:       []string{"read_query", "write_query", "create_table", "list_tables", "describe_table", "append_insight"},
		},
		{
			ID:          "postgres",
			Name:        "PostgreSQL",
			Description: "Query PostgreSQL databases",
			Author:      "Anthropic",
			Version:     "0.6.0",
			Repository:  "https://github.com/modelcontextprotocol/servers",
			Category:    "database",
			Tags:        []string{"database", "sql", "postgres"},
			InstallType: "npm",
			InstallCmd:  "npm install -g @modelcontextprotocol/server-postgres",
			Command:     "npx",
			Args:        []string{"-y", "@modelcontextprotocol/server-postgres", "postgresql://localhost/mydb"},
			Tools:       []string{"query"},
		},
		{
			ID:          "slack",
			Name:        "Slack",
			Description: "Interact with Slack workspaces",
			Author:      "Anthropic",
			Version:     "0.6.0",
			Repository:  "https://github.com/modelcontextprotocol/servers",
			Category:    "communication",
			Tags:        []string{"slack", "chat", "messaging"},
			InstallType: "npm",
			InstallCmd:  "npm install -g @modelcontextprotocol/server-slack",
			Command:     "npx",
			Args:        []string{"-y", "@modelcontextprotocol/server-slack"},
			Env:         map[string]string{"SLACK_BOT_TOKEN": "", "SLACK_TEAM_ID": ""},
			Tools:       []string{"slack_list_channels", "slack_post_message", "slack_reply_to_thread", "slack_add_reaction", "slack_get_channel_history", "slack_get_thread_replies", "slack_search_messages", "slack_get_users", "slack_get_user_profile"},
		},
		{
			ID:          "google-drive",
			Name:        "Google Drive",
			Description: "Search and access Google Drive files",
			Author:      "Anthropic",
			Version:     "0.6.0",
			Repository:  "https://github.com/modelcontextprotocol/servers",
			Category:    "storage",
			Tags:        []string{"google", "drive", "cloud"},
			InstallType: "npm",
			InstallCmd:  "npm install -g @modelcontextprotocol/server-gdrive",
			Command:     "npx",
			Args:        []string{"-y", "@modelcontextprotocol/server-gdrive"},
			Tools:       []string{"search", "read_file"},
		},
		{
			ID:          "everart",
			Name:        "EverArt",
			Description: "AI image generation with multiple models",
			Author:      "Anthropic",
			Version:     "0.6.0",
			Repository:  "https://github.com/modelcontextprotocol/servers",
			Category:    "ai",
			Tags:        []string{"image", "generation", "art"},
			InstallType: "npm",
			InstallCmd:  "npm install -g @modelcontextprotocol/server-everart",
			Command:     "npx",
			Args:        []string{"-y", "@modelcontextprotocol/server-everart"},
			Env:         map[string]string{"EVERART_API_KEY": ""},
			Tools:       []string{"generate_image", "list_models"},
		},
		{
			ID:          "sequential-thinking",
			Name:        "Sequential Thinking",
			Description: "Dynamic problem-solving through thought sequences",
			Author:      "Anthropic",
			Version:     "0.6.0",
			Repository:  "https://github.com/modelcontextprotocol/servers",
			Category:    "ai",
			Tags:        []string{"thinking", "reasoning", "planning"},
			InstallType: "npm",
			InstallCmd:  "npm install -g @modelcontextprotocol/server-sequential-thinking",
			Command:     "npx",
			Args:        []string{"-y", "@modelcontextprotocol/server-sequential-thinking"},
			Tools:       []string{"sequentialthinking"},
		},
	}
}

// ListAvailable returns available MCP servers from built-in list and cache
func (m *Marketplace) ListAvailable(category string) []*MarketplaceServer {
	m.mu.RLock()
	defer m.mu.RUnlock()

	servers := BuiltInServers()

	// Filter by category if specified
	if category != "" {
		filtered := make([]*MarketplaceServer, 0)
		for _, s := range servers {
			if s.Category == category {
				filtered = append(filtered, s)
			}
		}
		return filtered
	}

	return servers
}

// GetServer returns a specific server by ID
func (m *Marketplace) GetServer(id string) (*MarketplaceServer, error) {
	for _, s := range BuiltInServers() {
		if s.ID == id {
			return s, nil
		}
	}
	return nil, fmt.Errorf("server not found: %s", id)
}

// ListInstalled returns installed MCP servers
func (m *Marketplace) ListInstalled() []*InstalledServer {
	m.mu.RLock()
	defer m.mu.RUnlock()

	servers := make([]*InstalledServer, 0, len(m.installed))
	for _, s := range m.installed {
		servers = append(servers, s)
	}
	return servers
}

// GetInstalled returns a specific installed server
func (m *Marketplace) GetInstalled(id string) (*InstalledServer, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.installed[id]
	return s, ok
}

// Install installs an MCP server from the marketplace
func (m *Marketplace) Install(id string, env map[string]string) error {
	server, err := m.GetServer(id)
	if err != nil {
		return err
	}

	// Check if already installed
	if _, exists := m.GetInstalled(id); exists {
		return fmt.Errorf("server already installed: %s", id)
	}

	// Run install command based on type
	var installErr error
	switch server.InstallType {
	case "npm":
		installErr = m.installNPM(server)
	case "pip":
		installErr = m.installPIP(server)
	case "binary":
		installErr = m.installBinary(server)
	case "docker":
		installErr = m.installDocker(server)
	default:
		return fmt.Errorf("unknown install type: %s", server.InstallType)
	}

	if installErr != nil {
		return fmt.Errorf("installation failed: %w", installErr)
	}

	// Merge environment variables
	finalEnv := make(map[string]string)
	for k, v := range server.Env {
		finalEnv[k] = v
	}
	for k, v := range env {
		finalEnv[k] = v
	}

	// Register as installed
	installed := &InstalledServer{
		ID:          server.ID,
		Name:        server.Name,
		Version:     server.Version,
		InstallType: server.InstallType,
		Command:     server.Command,
		Args:        server.Args,
		Env:         finalEnv,
		InstalledAt: time.Now(),
		Enabled:     true,
	}

	m.mu.Lock()
	m.installed[id] = installed
	m.mu.Unlock()

	m.saveInstalled()
	return nil
}

// Uninstall removes an installed MCP server
func (m *Marketplace) Uninstall(id string) error {
	installed, exists := m.GetInstalled(id)
	if !exists {
		return fmt.Errorf("server not installed: %s", id)
	}

	// Run uninstall based on type
	var uninstallErr error
	switch installed.InstallType {
	case "npm":
		// Get server info for package name
		server, _ := m.GetServer(id)
		if server != nil && strings.Contains(server.InstallCmd, "@modelcontextprotocol/") {
			parts := strings.Split(server.InstallCmd, " ")
			for _, p := range parts {
				if strings.HasPrefix(p, "@modelcontextprotocol/") {
					cmd := exec.Command("npm", "uninstall", "-g", p)
					uninstallErr = cmd.Run()
					break
				}
			}
		}
	case "pip":
		// Try to uninstall pip package
		// This is best-effort
	case "binary":
		// Remove binary from bin dir
		binPath := filepath.Join(m.dataDir, "bin", id)
		os.Remove(binPath)
		os.Remove(binPath + ".exe")
	case "docker":
		// Remove docker image
		cmd := exec.Command("docker", "rmi", fmt.Sprintf("mcp-%s", id))
		cmd.Run()
	}

	if uninstallErr != nil {
		// Log but don't fail - just remove from installed list
	}

	m.mu.Lock()
	delete(m.installed, id)
	m.mu.Unlock()

	m.saveInstalled()
	return nil
}

// Enable enables an installed server
func (m *Marketplace) Enable(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	installed, exists := m.installed[id]
	if !exists {
		return fmt.Errorf("server not installed: %s", id)
	}

	installed.Enabled = true
	m.saveInstalled()
	return nil
}

// Disable disables an installed server
func (m *Marketplace) Disable(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	installed, exists := m.installed[id]
	if !exists {
		return fmt.Errorf("server not installed: %s", id)
	}

	installed.Enabled = false
	m.saveInstalled()
	return nil
}

// UpdateEnv updates environment variables for an installed server
func (m *Marketplace) UpdateEnv(id string, env map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	installed, exists := m.installed[id]
	if !exists {
		return fmt.Errorf("server not installed: %s", id)
	}

	for k, v := range env {
		installed.Env[k] = v
	}

	m.saveInstalled()
	return nil
}

// GetCategories returns available categories
func (m *Marketplace) GetCategories() []string {
	return []string{
		"utility",
		"search",
		"development",
		"automation",
		"memory",
		"database",
		"communication",
		"storage",
		"ai",
	}
}

// installNPM installs an npm package globally
func (m *Marketplace) installNPM(server *MarketplaceServer) error {
	// Check if npm is available
	if _, err := exec.LookPath("npm"); err != nil {
		return fmt.Errorf("npm not found - please install Node.js")
	}

	// Parse install command
	parts := strings.Fields(server.InstallCmd)
	if len(parts) < 4 {
		return fmt.Errorf("invalid npm install command")
	}

	// Run: npm install -g <package>
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// installPIP installs a pip package
func (m *Marketplace) installPIP(server *MarketplaceServer) error {
	// Check for pip or pip3
	pipCmd := "pip3"
	if _, err := exec.LookPath("pip3"); err != nil {
		if _, err := exec.LookPath("pip"); err != nil {
			return fmt.Errorf("pip not found - please install Python")
		}
		pipCmd = "pip"
	}

	// Parse install command
	parts := strings.Fields(server.InstallCmd)
	if len(parts) < 3 {
		return fmt.Errorf("invalid pip install command")
	}

	// Replace pip with detected pip command
	parts[0] = pipCmd
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// installBinary downloads and installs a binary
func (m *Marketplace) installBinary(server *MarketplaceServer) error {
	// Download binary from repository or URL
	if server.Repository == "" {
		return fmt.Errorf("no repository URL for binary download")
	}

	// Create bin directory
	binDir := filepath.Join(m.dataDir, "bin")
	os.MkdirAll(binDir, 0755)

	// Determine binary name based on platform
	binName := server.ID
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}

	// This is a placeholder - real implementation would download from releases
	return fmt.Errorf("binary installation requires manual download from: %s", server.Repository)
}

// installDocker pulls a Docker image
func (m *Marketplace) installDocker(server *MarketplaceServer) error {
	// Check if docker is available
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker not found")
	}

	// Pull the image
	parts := strings.Fields(server.InstallCmd)
	if len(parts) < 3 {
		return fmt.Errorf("invalid docker install command")
	}

	cmd := exec.Command("docker", parts[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// saveInstalled persists installed servers to disk
func (m *Marketplace) saveInstalled() {
	if m.dataDir == "" {
		return
	}

	path := filepath.Join(m.dataDir, "mcp_servers.json")
	data, err := json.MarshalIndent(m.installed, "", "  ")
	if err != nil {
		return
	}

	os.MkdirAll(filepath.Dir(path), 0755)
	os.WriteFile(path, data, 0644)
}

// loadInstalled loads installed servers from disk
func (m *Marketplace) loadInstalled() {
	if m.dataDir == "" {
		return
	}

	path := filepath.Join(m.dataDir, "mcp_servers.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	var installed map[string]*InstalledServer
	if err := json.Unmarshal(data, &installed); err != nil {
		return
	}

	m.installed = installed
}

// FetchFromRegistry fetches available servers from an online registry
func (m *Marketplace) FetchFromRegistry(registryURL string) ([]*MarketplaceServer, error) {
	if registryURL == "" {
		// Default to built-in servers
		return BuiltInServers(), nil
	}

	resp, err := http.Get(registryURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registry returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var servers []*MarketplaceServer
	if err := json.Unmarshal(body, &servers); err != nil {
		return nil, err
	}

	// Cache the results
	m.mu.Lock()
	m.availableCache = servers
	m.cacheExpiry = time.Now().Add(m.cacheDuration)
	m.mu.Unlock()

	return servers, nil
}

// SearchServers searches for servers by keyword
func (m *Marketplace) SearchServers(query string) []*MarketplaceServer {
	query = strings.ToLower(query)
	var results []*MarketplaceServer

	for _, s := range BuiltInServers() {
		if strings.Contains(strings.ToLower(s.Name), query) ||
			strings.Contains(strings.ToLower(s.Description), query) ||
			strings.Contains(strings.ToLower(s.Category), query) {
			results = append(results, s)
			continue
		}
		// Check tags
		for _, tag := range s.Tags {
			if strings.Contains(strings.ToLower(tag), query) {
				results = append(results, s)
				break
			}
		}
	}

	return results
}
