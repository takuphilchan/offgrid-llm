package config

import (
	"os"
	"path/filepath"
	"strconv"
)

// Config holds application configuration
type Config struct {
	// Server settings
	ServerPort int
	ServerHost string

	// Model settings
	ModelsDir      string
	DefaultModel   string
	MaxContextSize int
	NumThreads     int

	// Resource limits
	MaxMemoryMB  uint64
	MaxModels    int
	EnableGPU    bool
	NumGPULayers int

	// P2P settings
	EnableP2P     bool
	P2PPort       int
	DiscoveryPort int

	// Logging
	LogLevel string
	LogFile  string
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
