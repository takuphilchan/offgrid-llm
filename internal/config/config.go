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
	UseMockEngine  bool   `yaml:"use_mock_engine" json:"use_mock_engine"` // Use mock instead of llama.cpp

	// Resource limits
	MaxMemoryMB  uint64 `yaml:"max_memory_mb" json:"max_memory_mb"`
	MaxModels    int    `yaml:"max_models" json:"max_models"`
	EnableGPU    bool   `yaml:"enable_gpu" json:"enable_gpu"`
	NumGPULayers int    `yaml:"num_gpu_layers" json:"num_gpu_layers"`

	// Performance tuning for low-end hardware
	BatchSize       int    `yaml:"batch_size" json:"batch_size"`             // Token batch size (lower = faster first token, default: 512)
	FlashAttention  bool   `yaml:"flash_attention" json:"flash_attention"`   // Enable flash attention (faster, less memory)
	KVCacheType     string `yaml:"kv_cache_type" json:"kv_cache_type"`       // KV cache quantization: f16, q8_0, q4_0 (default: q8_0)
	UseMmap         bool   `yaml:"use_mmap" json:"use_mmap"`                 // Memory-map model file (good for low RAM)
	UseMlock        bool   `yaml:"use_mlock" json:"use_mlock"`               // Lock model in RAM (good for high RAM)
	ContBatching    bool   `yaml:"cont_batching" json:"cont_batching"`       // Continuous batching for multi-request throughput
	LowMemoryMode   bool   `yaml:"low_memory_mode" json:"low_memory_mode"`   // Enable optimizations for <8GB RAM systems
	AdaptiveContext bool   `yaml:"adaptive_context" json:"adaptive_context"` // Auto-adjust context size based on RAM

	// Fast model switching (critical for <5s switch times)
	PrewarmModels    bool `yaml:"prewarm_models" json:"prewarm_models"`         // Pre-warm models into OS page cache on startup
	SmartMlock       bool `yaml:"smart_mlock" json:"smart_mlock"`               // Auto-enable mlock for small models (RAM > 4x model size)
	ProtectDefault   bool `yaml:"protect_default" json:"protect_default"`       // Never evict default model from cache
	FastSwitchMode   bool `yaml:"fast_switch_mode" json:"fast_switch_mode"`     // Enable all fast-switching optimizations
	ModelLoadTimeout int  `yaml:"model_load_timeout" json:"model_load_timeout"` // Max seconds to wait for model load (default: 30)

	// P2P settings
	EnableP2P     bool `yaml:"enable_p2p" json:"enable_p2p"`
	P2PPort       int  `yaml:"p2p_port" json:"p2p_port"`
	DiscoveryPort int  `yaml:"discovery_port" json:"discovery_port"`

	// Logging
	LogLevel string `yaml:"log_level" json:"log_level"`
	LogFile  string `yaml:"log_file" json:"log_file"`

	// Authentication & Multi-user
	RequireAuth   bool `yaml:"require_auth" json:"require_auth"`       // Require authentication for API access
	GuestAccess   bool `yaml:"guest_access" json:"guest_access"`       // Allow guest access when auth not required
	MultiUserMode bool `yaml:"multi_user_mode" json:"multi_user_mode"` // Enable multi-user features (Users tab, quotas)

	// Enterprise Security (opt-in)
	CORSOrigins     string `yaml:"cors_origins" json:"cors_origins"`           // Comma-separated allowed origins (empty = allow all)
	EnableAuditLog  bool   `yaml:"enable_audit_log" json:"enable_audit_log"`   // Enable tamper-evident audit logging
	AuditLogDir     string `yaml:"audit_log_dir" json:"audit_log_dir"`         // Directory for audit logs
	EnableRequestID bool   `yaml:"enable_request_id" json:"enable_request_id"` // Add X-Request-ID header to all responses
}

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	// Prefer system-wide models directory if it exists
	systemModelsDir := "/var/lib/offgrid/models"
	homeDir, _ := os.UserHomeDir()
	userModelsDir := filepath.Join(homeDir, ".offgrid-llm", "models")

	// Default to system directory if it exists and we can read it
	defaultModelsDir := userModelsDir
	if info, err := os.Stat(systemModelsDir); err == nil && info.IsDir() {
		// Check if directory is readable by trying to list it
		if _, err := os.ReadDir(systemModelsDir); err == nil {
			defaultModelsDir = systemModelsDir
		}
	}

	return &Config{
		ServerPort:      getEnvInt("OFFGRID_PORT", 11611),
		ServerHost:      getEnv("OFFGRID_HOST", "localhost"),
		ModelsDir:       getEnv("OFFGRID_MODELS_DIR", defaultModelsDir),
		DefaultModel:    getEnv("OFFGRID_DEFAULT_MODEL", ""),
		MaxContextSize:  getEnvInt("OFFGRID_MAX_CONTEXT", 4096),
		NumThreads:      getEnvInt("OFFGRID_NUM_THREADS", 0), // 0 = auto-detect
		MaxMemoryMB:     uint64(getEnvInt("OFFGRID_MAX_MEMORY_MB", 4096)),
		MaxModels:       getEnvInt("OFFGRID_MAX_MODELS", 3), // Cache up to 3 models for fast switching
		EnableGPU:       getEnvBool("OFFGRID_ENABLE_GPU", false),
		NumGPULayers:    getEnvInt("OFFGRID_GPU_LAYERS", 0),
		BatchSize:       getEnvInt("OFFGRID_BATCH_SIZE", 512), // Higher batch for throughput
		FlashAttention:  getEnvBool("OFFGRID_FLASH_ATTENTION", true),
		KVCacheType:     getEnv("OFFGRID_KV_CACHE_TYPE", "q8_0"),
		UseMmap:         getEnvBool("OFFGRID_USE_MMAP", true),
		UseMlock:        getEnvBool("OFFGRID_USE_MLOCK", false),
		ContBatching:    getEnvBool("OFFGRID_CONT_BATCHING", true), // Better throughput
		LowMemoryMode:   getEnvBool("OFFGRID_LOW_MEMORY", false),
		AdaptiveContext: getEnvBool("OFFGRID_ADAPTIVE_CONTEXT", true),
		// Fast model switching
		PrewarmModels:    getEnvBool("OFFGRID_PREWARM_MODELS", true),
		SmartMlock:       getEnvBool("OFFGRID_SMART_MLOCK", true),
		ProtectDefault:   getEnvBool("OFFGRID_PROTECT_DEFAULT", true), // Keep default model in cache
		FastSwitchMode:   getEnvBool("OFFGRID_FAST_SWITCH", true),
		ModelLoadTimeout: getEnvInt("OFFGRID_MODEL_LOAD_TIMEOUT", 300), // 5 minute timeout for low-end machines
		EnableP2P:        getEnvBool("OFFGRID_ENABLE_P2P", false),
		P2PPort:          getEnvInt("OFFGRID_P2P_PORT", 9090),
		DiscoveryPort:    getEnvInt("OFFGRID_DISCOVERY_PORT", 9091),
		LogLevel:         getEnv("OFFGRID_LOG_LEVEL", "info"),
		LogFile:          getEnv("OFFGRID_LOG_FILE", ""),
		RequireAuth:      getEnvBool("OFFGRID_REQUIRE_AUTH", false),
		GuestAccess:      getEnvBool("OFFGRID_GUEST_ACCESS", true),
		MultiUserMode:    getEnvBool("OFFGRID_MULTI_USER", false),
		// Enterprise Security (opt-in, disabled by default for simple use)
		CORSOrigins:     getEnv("OFFGRID_CORS_ORIGINS", ""),      // Empty = allow all (simple mode)
		EnableAuditLog:  getEnvBool("OFFGRID_AUDIT_LOG", false),  // Disabled by default
		AuditLogDir:     getEnv("OFFGRID_AUDIT_LOG_DIR", ""),     // Default set in applyDefaults
		EnableRequestID: getEnvBool("OFFGRID_REQUEST_ID", false), // Disabled by default
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
	// Prefer system-wide models directory if it exists
	systemModelsDir := "/var/lib/offgrid/models"
	homeDir, _ := os.UserHomeDir()
	userModelsDir := filepath.Join(homeDir, ".offgrid-llm", "models")

	// Default to system directory if it exists and we can read it
	defaultModelsDir := userModelsDir
	if info, err := os.Stat(systemModelsDir); err == nil && info.IsDir() {
		// Check if directory is readable by trying to list it
		if _, err := os.ReadDir(systemModelsDir); err == nil {
			defaultModelsDir = systemModelsDir
		}
	}

	if c.ServerPort == 0 {
		c.ServerPort = 11611
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
	// NumThreads 0 means auto-detect (don't override)
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
	// Performance tuning defaults
	if c.BatchSize == 0 {
		c.BatchSize = 512
	}
	if c.KVCacheType == "" {
		c.KVCacheType = "q8_0"
	}

	// Boolean defaults that should be TRUE for optimal experience
	// These need special handling since Go defaults bools to false
	// We use a sentinel approach: check if they were explicitly set in the config file
	// For now, we ALWAYS enable these for best out-of-box experience
	// Users can explicitly disable with env vars if needed
	c.UseMmap = true         // Always use mmap for low RAM safety
	c.FlashAttention = true  // Always enable for speed
	c.ContBatching = true    // Always enable for throughput
	c.AdaptiveContext = true // Always enable for auto-adjustment

	// Fast model switching - CRITICAL for good UX, always enable
	c.PrewarmModels = true  // Pre-warm models into page cache
	c.SmartMlock = true     // Auto mlock for small models
	c.ProtectDefault = true // Don't evict default model
	c.FastSwitchMode = true // Enable all fast-switch optimizations

	// Fast model switching defaults - enable by default for best UX
	if c.ModelLoadTimeout == 0 {
		c.ModelLoadTimeout = 300 // 5 minutes for low-end machines
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
	if auth := os.Getenv("OFFGRID_REQUIRE_AUTH"); auth != "" {
		c.RequireAuth = getEnvBool("OFFGRID_REQUIRE_AUTH", false)
	}
	if guest := os.Getenv("OFFGRID_GUEST_ACCESS"); guest != "" {
		c.GuestAccess = getEnvBool("OFFGRID_GUEST_ACCESS", true)
	}
	if multiUser := os.Getenv("OFFGRID_MULTI_USER"); multiUser != "" {
		c.MultiUserMode = getEnvBool("OFFGRID_MULTI_USER", false)
	}

	// Fast model switching overrides (allow disabling if explicitly set to false)
	if prewarm := os.Getenv("OFFGRID_PREWARM_MODELS"); prewarm != "" {
		c.PrewarmModels = getEnvBool("OFFGRID_PREWARM_MODELS", true)
	}
	if smartMlock := os.Getenv("OFFGRID_SMART_MLOCK"); smartMlock != "" {
		c.SmartMlock = getEnvBool("OFFGRID_SMART_MLOCK", true)
	}
	if protect := os.Getenv("OFFGRID_PROTECT_DEFAULT"); protect != "" {
		c.ProtectDefault = getEnvBool("OFFGRID_PROTECT_DEFAULT", true)
	}
	if fastSwitch := os.Getenv("OFFGRID_FAST_SWITCH"); fastSwitch != "" {
		c.FastSwitchMode = getEnvBool("OFFGRID_FAST_SWITCH", true)
	}
	if timeout := getEnvInt("OFFGRID_MODEL_LOAD_TIMEOUT", 0); timeout != 0 {
		c.ModelLoadTimeout = timeout
	}

	// Enterprise Security overrides
	if cors := getEnv("OFFGRID_CORS_ORIGINS", ""); cors != "" {
		c.CORSOrigins = cors
	}
	if audit := os.Getenv("OFFGRID_AUDIT_LOG"); audit != "" {
		c.EnableAuditLog = getEnvBool("OFFGRID_AUDIT_LOG", false)
	}
	if auditDir := getEnv("OFFGRID_AUDIT_LOG_DIR", ""); auditDir != "" {
		c.AuditLogDir = auditDir
	}
	if reqID := os.Getenv("OFFGRID_REQUEST_ID"); reqID != "" {
		c.EnableRequestID = getEnvBool("OFFGRID_REQUEST_ID", false)
	}
}
