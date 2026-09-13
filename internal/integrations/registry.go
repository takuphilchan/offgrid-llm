// Package integrations defines first-class adapters for external agent runtimes.
//
// An adapter owns only the external system's configuration contract. Runtime
// health and model probes remain in the server package so adapters stay small,
// deterministic, and easy to extend.
package integrations

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Profile describes an external agent integration supported by OffGrid.
type Profile struct {
	ID                 string   `json:"id"`
	ProviderID         string   `json:"provider_id"`
	PluginID           string   `json:"plugin_id"`
	Name               string   `json:"name"`
	Description        string   `json:"description"`
	Transport          string   `json:"transport"`
	DocumentationURL   string   `json:"documentation_url"`
	MinimumContext     int      `json:"minimum_context"`
	RecommendedContext int      `json:"recommended_context"`
	RequiresTools      bool     `json:"requires_tools"`
	Capabilities       []string `json:"capabilities"`
}

// Options are the runtime-specific values used to render setup instructions.
type Options struct {
	BaseURL         string
	ModelID         string
	ContextWindow   int
	MaxOutputTokens int
}

// Setup is a ready-to-copy external agent configuration.
type Setup struct {
	ProviderID  string            `json:"provider_id"`
	PluginID    string            `json:"plugin_id"`
	Install     string            `json:"install_command"`
	Format      string            `json:"format"`
	ConfigFile  string            `json:"config_file"`
	Content     string            `json:"content"`
	Environment map[string]string `json:"environment,omitempty"`
	Verify      []string          `json:"verify"`
	Notes       []string          `json:"notes,omitempty"`
}

// Adapter renders setup for one external agent runtime.
type Adapter interface {
	Profile() Profile
	Render(Options) (Setup, error)
}

// Registry stores adapters behind a stable extension boundary.
type Registry struct {
	mu       sync.RWMutex
	adapters map[string]Adapter
}

func NewRegistry() *Registry {
	return &Registry{adapters: make(map[string]Adapter)}
}

func NewDefaultRegistry() *Registry {
	registry := NewRegistry()
	_ = registry.Register(hermesAdapter{})
	_ = registry.Register(openClawAdapter{})
	return registry
}

func (r *Registry) Register(adapter Adapter) error {
	if adapter == nil {
		return fmt.Errorf("integration adapter is required")
	}
	profile := adapter.Profile()
	profile.ID = strings.ToLower(strings.TrimSpace(profile.ID))
	if profile.ID == "" || profile.Name == "" || profile.Transport == "" {
		return fmt.Errorf("integration id, name, and transport are required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.adapters[profile.ID]; exists {
		return fmt.Errorf("integration already registered: %s", profile.ID)
	}
	r.adapters[profile.ID] = adapter
	return nil
}

func (r *Registry) Get(id string) (Adapter, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	adapter, ok := r.adapters[strings.ToLower(strings.TrimSpace(id))]
	return adapter, ok
}

func (r *Registry) List() []Adapter {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Adapter, 0, len(r.adapters))
	for _, adapter := range r.adapters {
		result = append(result, adapter)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Profile().Name < result[j].Profile().Name
	})
	return result
}

type hermesAdapter struct{}

func (hermesAdapter) Profile() Profile {
	return Profile{
		ID:                 "hermes",
		ProviderID:         "offgrid",
		PluginID:           "offgrid",
		Name:               "Hermes Agent",
		Description:        "Run NousResearch Hermes Agent with private OffGrid inference.",
		Transport:          "openai-chat-completions",
		DocumentationURL:   "https://github.com/NousResearch/hermes-agent/blob/main/website/docs/developer-guide/model-provider-plugin.md",
		MinimumContext:     64000,
		RecommendedContext: 65536,
		RequiresTools:      true,
		Capabilities:       []string{"chat", "streaming", "tools", "model-discovery"},
	}
}

func (hermesAdapter) Render(options Options) (Setup, error) {
	if err := validateOptions(options); err != nil {
		return Setup{}, err
	}
	baseURL := openAIBaseURL(options.BaseURL)
	content := strings.Join([]string{
		"model:",
		"  default: " + strconv.Quote(options.ModelID),
		"  provider: offgrid",
		"  base_url: " + strconv.Quote(baseURL),
		fmt.Sprintf("  context_length: %d", options.ContextWindow),
	}, "\n") + "\n"
	return Setup{
		ProviderID: "offgrid",
		PluginID:   "offgrid",
		Install:    "offgrid hermes install --model " + strconv.Quote(options.ModelID),
		Format:     "yaml",
		ConfigFile: "~/.hermes/config.yaml",
		Content:    content,
		Environment: map[string]string{
			"OFFGRID_API_KEY":  "offgrid-local",
			"OFFGRID_BASE_URL": baseURL,
		},
		Verify: []string{
			"offgrid hermes status",
			"offgrid hermes test",
		},
		Notes: []string{
			"The managed command installs Hermes when needed and configures a native provider named offgrid.",
			"Hermes Agent requires at least 64,000 context tokens; 65,536 is recommended.",
			"Keep OffGrid reachable for the lifetime of the Hermes session.",
		},
	}, nil
}

type openClawAdapter struct{}

func (openClawAdapter) Profile() Profile {
	return Profile{
		ID:                 "openclaw",
		ProviderID:         "offgrid",
		PluginID:           "offgrid",
		Name:               "OpenClaw",
		Description:        "Use OffGrid as a first-class OpenClaw model provider.",
		Transport:          "openai-chat-completions",
		DocumentationURL:   "https://github.com/openclaw/openclaw/blob/main/docs/plugins/sdk-provider-plugins.md",
		MinimumContext:     8192,
		RecommendedContext: 65536,
		RequiresTools:      true,
		Capabilities:       []string{"chat", "streaming", "tools", "model-discovery"},
	}
}

func (openClawAdapter) Render(options Options) (Setup, error) {
	if err := validateOptions(options); err != nil {
		return Setup{}, err
	}
	baseURL := openAIBaseURL(options.BaseURL)
	config := map[string]any{
		"agents": map[string]any{
			"defaults": map[string]any{
				"model": map[string]any{"primary": "offgrid/" + options.ModelID},
			},
		},
		"models": map[string]any{
			"mode": "merge",
			"providers": map[string]any{
				"offgrid": map[string]any{
					"baseUrl":        baseURL,
					"apiKey":         "offgrid-local",
					"api":            "openai-completions",
					"timeoutSeconds": 300,
					"models": []map[string]any{{
						"id":            options.ModelID,
						"name":          options.ModelID + " via OffGrid",
						"reasoning":     false,
						"input":         []string{"text"},
						"cost":          map[string]int{"input": 0, "output": 0, "cacheRead": 0, "cacheWrite": 0},
						"contextWindow": options.ContextWindow,
						"maxTokens":     options.MaxOutputTokens,
					}},
				},
			},
		},
	}
	encoded, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return Setup{}, fmt.Errorf("render OpenClaw configuration: %w", err)
	}
	return Setup{
		ProviderID: "offgrid",
		PluginID:   "offgrid",
		Install:    "offgrid openclaw install --model " + strconv.Quote(options.ModelID),
		Format:     "json5",
		ConfigFile: "~/.openclaw/openclaw.json",
		Content:    string(encoded) + "\n",
		Verify: []string{
			"offgrid openclaw status",
			"offgrid openclaw test",
		},
		Notes: []string{
			"The managed command installs OpenClaw when needed and configures a native provider named offgrid.",
			"The provider uses OffGrid's OpenAI-compatible /v1 transport and discovers chat models only.",
			"Run a real tool task before making a small local model your default.",
		},
	}, nil
}

func validateOptions(options Options) error {
	if strings.TrimSpace(options.BaseURL) == "" {
		return fmt.Errorf("base URL is required")
	}
	if strings.TrimSpace(options.ModelID) == "" {
		return fmt.Errorf("model is required")
	}
	if options.ContextWindow <= 0 {
		return fmt.Errorf("context window must be greater than zero")
	}
	if options.MaxOutputTokens <= 0 {
		return fmt.Errorf("max output tokens must be greater than zero")
	}
	return nil
}

func nativeBaseURL(value string) string {
	return strings.TrimSuffix(strings.TrimSuffix(strings.TrimSpace(value), "/"), "/v1")
}

func openAIBaseURL(value string) string {
	return nativeBaseURL(value) + "/v1"
}
