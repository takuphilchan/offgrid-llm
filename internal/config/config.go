package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Config holds application configuration
type Config struct {
	// Server settings
	ServerPort int    `yaml:"server_port" json:"server_port"`
	ServerHost string `yaml:"server_host" json:"server_host"`

	// Model settings
	ModelsDir      string `yaml:"models_dir" json:"models_dir"`
	DefaultModel   string `yaml:"default_model" json:"default_model"`
	MaxContextSize int    `yaml:"max_context_size" json:"max_context_size"`
	NumThreads     int    `yaml:"num_threads" json:"num_threads"`

	// Resource limits
	MaxMemoryMB  uint64 `yaml:"max_memory_mb" json:"max_memory_mb"`
	MaxModels    int    `yaml:"max_models" json:"max_models"`
	EnableGPU    bool   `yaml:"enable_gpu" json:"enable_gpu"`
	NumGPULayers int    `yaml:"num_gpu_layers" json:"num_gpu_layers"`

	// P2P settings
	EnableP2P     bool `yaml:"enable_p2p" json:"enable_p2p"`
	P2PPort       int  `yaml:"p2p_port" json:"p2p_port"`
	DiscoveryPort int  `yaml:"discovery_port" json:"discovery_port"`

	// Logging
	LogLevel string `yaml:"log_level" json:"log_level"`
	LogFile  string `yaml:"log_file" json:"log_file"`
}

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	homeDir, _ := os.UserHomeDir()
	defaultModelsDir := filepath.Join(homeDir, ".offgrid-llm", "models")

	return &Config{
		ServerPort:     getEnvInt("OFFGRID_PORT", 8080),
		ServerHost:     getEnv("OFFGRID_HOST", "localhost"),
		ModelsDir:      getEnv("OFFGRID_MODELS_DIR", defaultModelsDir),
		DefaultModel:   getEnv("OFFGRID_DEFAULT_MODEL", ""),
		MaxContextSize: getEnvInt("OFFGRID_MAX_CONTEXT", 4096),
		NumThreads:     getEnvInt("OFFGRID_NUM_THREADS", 4),
		MaxMemoryMB:    uint64(getEnvInt("OFFGRID_MAX_MEMORY_MB", 4096)),
		MaxModels:      getEnvInt("OFFGRID_MAX_MODELS", 3),
		EnableGPU:      getEnvBool("OFFGRID_ENABLE_GPU", false),
		NumGPULayers:   getEnvInt("OFFGRID_GPU_LAYERS", 0),
		EnableP2P:      getEnvBool("OFFGRID_ENABLE_P2P", false),
		P2PPort:        getEnvInt("OFFGRID_P2P_PORT", 9090),
		DiscoveryPort:  getEnvInt("OFFGRID_DISCOVERY_PORT", 9091),
		LogLevel:       getEnv("OFFGRID_LOG_LEVEL", "info"),
		LogFile:        getEnv("OFFGRID_LOG_FILE", ""),
	}
}

// Helper functions

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return defaultValue
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Ensure models directory exists
	if err := os.MkdirAll(c.ModelsDir, 0755); err != nil {
		return err
	}

	return nil
}

// LoadFromFile loads configuration from a YAML or JSON file
func LoadFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg := &Config{}

	// Try YAML first
	ext := filepath.Ext(path)
	if ext == ".yaml" || ext == ".yml" {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("failed to parse YAML config: %w", err)
		}
	} else if ext == ".json" {
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("failed to parse JSON config: %w", err)
		}
	} else {
		return nil, fmt.Errorf("unsupported config file format: %s (use .yaml, .yml, or .json)", ext)
	}

	// Apply defaults for any missing values
	cfg.applyDefaults()

	return cfg, nil
}

// SaveToFile saves configuration to a YAML or JSON file
func (c *Config) SaveToFile(path string) error {
	var data []byte
	var err error

	ext := filepath.Ext(path)
	if ext == ".yaml" || ext == ".yml" {
		data, err = yaml.Marshal(c)
		if err != nil {
			return fmt.Errorf("failed to marshal YAML: %w", err)
		}
	} else if ext == ".json" {
		data, err = json.MarshalIndent(c, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
	} else {
		return fmt.Errorf("unsupported config file format: %s (use .yaml, .yml, or .json)", ext)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// LoadWithPriority loads config with priority: file > env > defaults
func LoadWithPriority(configPath string) (*Config, error) {
	var cfg *Config
	var err error

	// Try to load from file first
	if configPath != "" {
		cfg, err = LoadFromFile(configPath)
		if err != nil {
			return nil, err
		}
	} else {
		// Check for default config file locations
		homeDir, _ := os.UserHomeDir()
		configDirs := []string{
			filepath.Join(homeDir, ".offgrid-llm", "config.yaml"),
			filepath.Join(homeDir, ".offgrid-llm", "config.yml"),
			filepath.Join(homeDir, ".offgrid-llm", "config.json"),
			"config.yaml",
			"config.yml",
			"config.json",
		}

		for _, path := range configDirs {
			if _, err := os.Stat(path); err == nil {
				cfg, err = LoadFromFile(path)
				if err != nil {
					return nil, err
				}
				break
			}
		}

		// If no file found, use defaults
		if cfg == nil {
			cfg = LoadConfig()
		}
	}

	// Override with environment variables
	cfg.applyEnvOverrides()

	return cfg, nil
}

// applyDefaults sets default values for unset fields
func (c *Config) applyDefaults() {
	homeDir, _ := os.UserHomeDir()
	defaultModelsDir := filepath.Join(homeDir, ".offgrid-llm", "models")

	if c.ServerPort == 0 {
		c.ServerPort = 8080
	}
	if c.ServerHost == "" {
		c.ServerHost = "localhost"
	}
	if c.ModelsDir == "" {
		c.ModelsDir = defaultModelsDir
	}
	if c.MaxContextSize == 0 {
		c.MaxContextSize = 4096
	}
	if c.NumThreads == 0 {
		c.NumThreads = 4
	}
	if c.MaxMemoryMB == 0 {
		c.MaxMemoryMB = 4096
	}
	if c.MaxModels == 0 {
		c.MaxModels = 3
	}
	if c.P2PPort == 0 {
		c.P2PPort = 9090
	}
	if c.DiscoveryPort == 0 {
		c.DiscoveryPort = 9091
	}
	if c.LogLevel == "" {
		c.LogLevel = "info"
	}
}

// applyEnvOverrides overrides config with environment variables
func (c *Config) applyEnvOverrides() {
	if port := getEnvInt("OFFGRID_PORT", 0); port != 0 {
		c.ServerPort = port
	}
	if host := getEnv("OFFGRID_HOST", ""); host != "" {
		c.ServerHost = host
	}
	if dir := getEnv("OFFGRID_MODELS_DIR", ""); dir != "" {
		c.ModelsDir = dir
	}
	if model := getEnv("OFFGRID_DEFAULT_MODEL", ""); model != "" {
		c.DefaultModel = model
	}
	if ctx := getEnvInt("OFFGRID_MAX_CONTEXT", 0); ctx != 0 {
		c.MaxContextSize = ctx
	}
	if threads := getEnvInt("OFFGRID_NUM_THREADS", 0); threads != 0 {
		c.NumThreads = threads
	}
	if mem := getEnvInt("OFFGRID_MAX_MEMORY_MB", 0); mem != 0 {
		c.MaxMemoryMB = uint64(mem)
	}
	if maxModels := getEnvInt("OFFGRID_MAX_MODELS", 0); maxModels != 0 {
		c.MaxModels = maxModels
	}
	if gpu := os.Getenv("OFFGRID_ENABLE_GPU"); gpu != "" {
		c.EnableGPU = getEnvBool("OFFGRID_ENABLE_GPU", false)
	}
	if layers := getEnvInt("OFFGRID_GPU_LAYERS", 0); layers != 0 {
		c.NumGPULayers = layers
	}
	if p2p := os.Getenv("OFFGRID_ENABLE_P2P"); p2p != "" {
		c.EnableP2P = getEnvBool("OFFGRID_ENABLE_P2P", false)
	}
	if p2pPort := getEnvInt("OFFGRID_P2P_PORT", 0); p2pPort != 0 {
		c.P2PPort = p2pPort
	}
	if discPort := getEnvInt("OFFGRID_DISCOVERY_PORT", 0); discPort != 0 {
		c.DiscoveryPort = discPort
	}
	if level := getEnv("OFFGRID_LOG_LEVEL", ""); level != "" {
		c.LogLevel = level
	}
	if file := getEnv("OFFGRID_LOG_FILE", ""); file != "" {
		c.LogFile = file
	}
}
