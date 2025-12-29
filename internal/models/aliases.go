package models

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// BuiltInAliases provides default aliases for popular models
// These map short names to HuggingFace model patterns
var BuiltInAliases = map[string]ModelAlias{
	// Llama family
	"llama3":      {Pattern: "Llama-3.2-3B-Instruct", HFRepo: "bartowski/Llama-3.2-3B-Instruct-GGUF", HFFile: "Llama-3.2-3B-Instruct-Q4_K_M.gguf", RAM: 4},
	"llama3.1":    {Pattern: "Llama-3.1-8B-Instruct", HFRepo: "bartowski/Meta-Llama-3.1-8B-Instruct-GGUF", HFFile: "Meta-Llama-3.1-8B-Instruct-Q4_K_M.gguf", RAM: 8},
	"llama3.2":    {Pattern: "Llama-3.2-3B-Instruct", HFRepo: "bartowski/Llama-3.2-3B-Instruct-GGUF", HFFile: "Llama-3.2-3B-Instruct-Q4_K_M.gguf", RAM: 4},
	"llama3.2-1b": {Pattern: "Llama-3.2-1B-Instruct", HFRepo: "bartowski/Llama-3.2-1B-Instruct-GGUF", HFFile: "Llama-3.2-1B-Instruct-Q4_K_M.gguf", RAM: 2},
	"llama3.2-3b": {Pattern: "Llama-3.2-3B-Instruct", HFRepo: "bartowski/Llama-3.2-3B-Instruct-GGUF", HFFile: "Llama-3.2-3B-Instruct-Q4_K_M.gguf", RAM: 4},

	// Qwen family
	"qwen":          {Pattern: "Qwen2.5-3B-Instruct", HFRepo: "Qwen/Qwen2.5-3B-Instruct-GGUF", HFFile: "qwen2.5-3b-instruct-q4_k_m.gguf", RAM: 4},
	"qwen2.5":       {Pattern: "Qwen2.5-7B-Instruct", HFRepo: "Qwen/Qwen2.5-7B-Instruct-GGUF", HFFile: "qwen2.5-7b-instruct-q4_k_m.gguf", RAM: 8},
	"qwen2.5-3b":    {Pattern: "Qwen2.5-3B-Instruct", HFRepo: "Qwen/Qwen2.5-3B-Instruct-GGUF", HFFile: "qwen2.5-3b-instruct-q4_k_m.gguf", RAM: 4},
	"qwen2.5-7b":    {Pattern: "Qwen2.5-7B-Instruct", HFRepo: "Qwen/Qwen2.5-7B-Instruct-GGUF", HFFile: "qwen2.5-7b-instruct-q4_k_m.gguf", RAM: 8},
	"qwen2.5-coder": {Pattern: "Qwen2.5-Coder-7B-Instruct", HFRepo: "Qwen/Qwen2.5-Coder-7B-Instruct-GGUF", HFFile: "qwen2.5-coder-7b-instruct-q4_k_m.gguf", RAM: 8},

	// Mistral family
	"mistral": {Pattern: "Mistral-7B-Instruct", HFRepo: "TheBloke/Mistral-7B-Instruct-v0.2-GGUF", HFFile: "mistral-7b-instruct-v0.2.Q4_K_M.gguf", RAM: 8},
	"mixtral": {Pattern: "Mixtral-8x7B-Instruct", HFRepo: "TheBloke/Mixtral-8x7B-Instruct-v0.1-GGUF", HFFile: "mixtral-8x7b-instruct-v0.1.Q4_K_M.gguf", RAM: 32},

	// Phi family
	"phi":  {Pattern: "Phi-3.5-mini-instruct", HFRepo: "bartowski/Phi-3.5-mini-instruct-GGUF", HFFile: "Phi-3.5-mini-instruct-Q4_K_M.gguf", RAM: 4},
	"phi3": {Pattern: "Phi-3.5-mini-instruct", HFRepo: "bartowski/Phi-3.5-mini-instruct-GGUF", HFFile: "Phi-3.5-mini-instruct-Q4_K_M.gguf", RAM: 4},
	"phi4": {Pattern: "phi-4", HFRepo: "bartowski/phi-4-GGUF", HFFile: "phi-4-Q4_K_M.gguf", RAM: 16},

	// Gemma family
	"gemma":  {Pattern: "gemma-2-2b-it", HFRepo: "bartowski/gemma-2-2b-it-GGUF", HFFile: "gemma-2-2b-it-Q4_K_M.gguf", RAM: 4},
	"gemma2": {Pattern: "gemma-2-9b-it", HFRepo: "bartowski/gemma-2-9b-it-GGUF", HFFile: "gemma-2-9b-it-Q4_K_M.gguf", RAM: 12},

	// Small/fast models
	"tiny": {Pattern: "TinyLlama", HFRepo: "TheBloke/TinyLlama-1.1B-Chat-v1.0-GGUF", HFFile: "tinyllama-1.1b-chat-v1.0.Q4_K_M.gguf", RAM: 2},
	"smol": {Pattern: "SmolLM2-1.7B-Instruct", HFRepo: "HuggingFaceTB/SmolLM2-1.7B-Instruct-GGUF", HFFile: "smollm2-1.7b-instruct-q4_k_m.gguf", RAM: 2},

	// Coding models
	"codellama":      {Pattern: "CodeLlama-7b-Instruct", HFRepo: "TheBloke/CodeLlama-7B-Instruct-GGUF", HFFile: "codellama-7b-instruct.Q4_K_M.gguf", RAM: 8},
	"deepseek-coder": {Pattern: "deepseek-coder-6.7b-instruct", HFRepo: "TheBloke/deepseek-coder-6.7B-instruct-GGUF", HFFile: "deepseek-coder-6.7b-instruct.Q4_K_M.gguf", RAM: 8},
	"starcoder":      {Pattern: "starcoder2-7b", HFRepo: "bartowski/starcoder2-7b-GGUF", HFFile: "starcoder2-7b-Q4_K_M.gguf", RAM: 8},

	// Vision models
	"llava":     {Pattern: "llava-v1.6-mistral-7b", HFRepo: "cjpais/llava-1.6-mistral-7b-gguf", HFFile: "llava-v1.6-mistral-7b.Q4_K_M.gguf", RAM: 8},
	"moondream": {Pattern: "moondream2", HFRepo: "vikhyatk/moondream2", HFFile: "moondream2-text-model-f16.gguf", RAM: 4},

	// Embedding models (for RAG)
	"embed": {Pattern: "bge-small-en-v1.5", HFRepo: "CompendiumLabs/bge-small-en-v1.5-gguf", HFFile: "bge-small-en-v1.5-q8_0.gguf", RAM: 1},
	"nomic": {Pattern: "nomic-embed-text-v1.5", HFRepo: "nomic-ai/nomic-embed-text-v1.5-GGUF", HFFile: "nomic-embed-text-v1.5.Q8_0.gguf", RAM: 1},
}

// ModelAlias represents a model alias with download information
type ModelAlias struct {
	Pattern string // Pattern to match in local model names
	HFRepo  string // HuggingFace repository
	HFFile  string // Recommended GGUF file to download
	RAM     int    // Minimum recommended RAM in GB
}

// AliasManager handles model aliases and favorites
type AliasManager struct {
	mu        sync.RWMutex
	aliases   map[string]string // alias -> model ID
	favorites map[string]bool   // model ID -> is favorite
	configDir string
}

type aliasConfig struct {
	Aliases   map[string]string `json:"aliases"`
	Favorites []string          `json:"favorites"`
}

// NewAliasManager creates a new alias manager
func NewAliasManager(configDir string) *AliasManager {
	am := &AliasManager{
		aliases:   make(map[string]string),
		favorites: make(map[string]bool),
		configDir: configDir,
	}
	am.load()
	return am
}

// SetAlias creates or updates an alias
func (am *AliasManager) SetAlias(alias, modelID string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	// Validate alias name (no spaces, special chars)
	if !isValidAliasName(alias) {
		return fmt.Errorf("invalid alias name: must be alphanumeric with dashes/underscores only")
	}

	am.aliases[alias] = modelID
	return am.save()
}

// GetAlias resolves an alias to a model ID
func (am *AliasManager) GetAlias(alias string) (string, bool) {
	am.mu.RLock()
	defer am.mu.RUnlock()
	modelID, ok := am.aliases[alias]
	return modelID, ok
}

// RemoveAlias deletes an alias
func (am *AliasManager) RemoveAlias(alias string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	if _, exists := am.aliases[alias]; !exists {
		return fmt.Errorf("alias not found: %s", alias)
	}

	delete(am.aliases, alias)
	return am.save()
}

// ListAliases returns all aliases
func (am *AliasManager) ListAliases() map[string]string {
	am.mu.RLock()
	defer am.mu.RUnlock()

	result := make(map[string]string, len(am.aliases))
	for k, v := range am.aliases {
		result[k] = v
	}
	return result
}

// ResolveModelOrAlias returns the actual model ID given a name or alias
func (am *AliasManager) ResolveModelOrAlias(nameOrAlias string) string {
	if modelID, ok := am.GetAlias(nameOrAlias); ok {
		return modelID
	}
	return nameOrAlias
}

// GetBuiltInAlias returns the built-in alias info for a short name
func GetBuiltInAlias(name string) (ModelAlias, bool) {
	name = strings.ToLower(name)
	if alias, ok := BuiltInAliases[name]; ok {
		return alias, true
	}
	return ModelAlias{}, false
}

// FindLocalModelByAlias searches for a locally installed model matching an alias pattern
func FindLocalModelByAlias(modelsDir string, aliasName string) (string, error) {
	alias, ok := GetBuiltInAlias(aliasName)
	if !ok {
		return "", fmt.Errorf("unknown alias: %s", aliasName)
	}

	// Scan models directory for matching files
	files, err := os.ReadDir(modelsDir)
	if err != nil {
		return "", err
	}

	pattern := strings.ToLower(alias.Pattern)
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		name := strings.ToLower(f.Name())
		if strings.Contains(name, pattern) && strings.HasSuffix(name, ".gguf") {
			return filepath.Join(modelsDir, f.Name()), nil
		}
	}

	return "", fmt.Errorf("no local model matching '%s' found", alias.Pattern)
}

// ListBuiltInAliases returns all available built-in aliases
func ListBuiltInAliases() map[string]ModelAlias {
	result := make(map[string]ModelAlias, len(BuiltInAliases))
	for k, v := range BuiltInAliases {
		result[k] = v
	}
	return result
}

// SetFavorite marks a model as favorite
func (am *AliasManager) SetFavorite(modelID string, isFavorite bool) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	if isFavorite {
		am.favorites[modelID] = true
	} else {
		delete(am.favorites, modelID)
	}

	return am.save()
}

// IsFavorite checks if a model is marked as favorite
func (am *AliasManager) IsFavorite(modelID string) bool {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return am.favorites[modelID]
}

// ListFavorites returns all favorite model IDs
func (am *AliasManager) ListFavorites() []string {
	am.mu.RLock()
	defer am.mu.RUnlock()

	favorites := make([]string, 0, len(am.favorites))
	for modelID := range am.favorites {
		favorites = append(favorites, modelID)
	}
	return favorites
}

// load reads aliases and favorites from disk
func (am *AliasManager) load() error {
	configPath := filepath.Join(am.configDir, "aliases.json")

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No config file yet, use defaults
		}
		return err
	}

	var config aliasConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	am.aliases = config.Aliases
	if am.aliases == nil {
		am.aliases = make(map[string]string)
	}

	am.favorites = make(map[string]bool)
	for _, modelID := range config.Favorites {
		am.favorites[modelID] = true
	}

	return nil
}

// save writes aliases and favorites to disk
func (am *AliasManager) save() error {
	// Ensure config directory exists
	if err := os.MkdirAll(am.configDir, 0755); err != nil {
		return err
	}

	// Convert favorites map to slice
	favList := make([]string, 0, len(am.favorites))
	for modelID := range am.favorites {
		favList = append(favList, modelID)
	}

	config := aliasConfig{
		Aliases:   am.aliases,
		Favorites: favList,
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	configPath := filepath.Join(am.configDir, "aliases.json")
	return os.WriteFile(configPath, data, 0644)
}

func isValidAliasName(name string) bool {
	if len(name) == 0 {
		return false
	}
	for _, r := range name {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '-' || r == '_') {
			return false
		}
	}
	return true
}
