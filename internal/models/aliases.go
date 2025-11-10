package models

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

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
