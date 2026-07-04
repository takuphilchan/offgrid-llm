package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// PathConfig contains platform-specific paths
type PathConfig struct {
	ModelDir      string
	SessionDir    string
	ConfigDir     string
	BinaryDir     string
	WebDir        string
	LogDir        string
	RuntimeDir    string
	ActiveModel   string
	LlamaPortFile string
}

// GetDefaultPaths returns default paths for the current platform
func GetDefaultPaths() *PathConfig {
	switch runtime.GOOS {
	case "windows":
		return getWindowsPaths()
	case "darwin":
		return getDarwinPaths()
	default: // linux
		return getLinuxPaths()
	}
}

// getLinuxPaths returns paths for Linux systems
func getLinuxPaths() *PathConfig {
	homeDir, _ := os.UserHomeDir()

	return &PathConfig{
		ModelDir:      "/var/lib/offgrid/models",
		SessionDir:    filepath.Join(homeDir, ".offgrid", "sessions"),
		ConfigDir:     "/etc/offgrid",
		BinaryDir:     "/usr/local/bin",
		WebDir:        "/var/lib/offgrid/web",
		LogDir:        "/var/log/offgrid",
		RuntimeDir:    "/var/run/offgrid",
		ActiveModel:   "/etc/offgrid/active-model",
		LlamaPortFile: "/etc/offgrid/llama-port",
	}
}

// getDarwinPaths returns paths for macOS systems
func getDarwinPaths() *PathConfig {
	homeDir, _ := os.UserHomeDir()

	return &PathConfig{
		// Use /usr/local on macOS (Homebrew convention)
		ModelDir:      "/usr/local/var/offgrid/models",
		SessionDir:    filepath.Join(homeDir, "Library", "Application Support", "OffGrid", "sessions"),
		ConfigDir:     "/usr/local/etc/offgrid",
		BinaryDir:     "/usr/local/bin",
		WebDir:        "/usr/local/var/offgrid/web",
		LogDir:        filepath.Join(homeDir, "Library", "Logs", "OffGrid"),
		RuntimeDir:    filepath.Join(homeDir, "Library", "Application Support", "OffGrid", "run"),
		ActiveModel:   "/usr/local/etc/offgrid/active-model",
		LlamaPortFile: "/usr/local/etc/offgrid/llama-port",
	}
}

// getWindowsPaths returns paths for Windows systems
func getWindowsPaths() *PathConfig {
	homeDir, _ := os.UserHomeDir()
	programData := os.Getenv("ProgramData")
	if programData == "" {
		programData = "C:\\ProgramData"
	}

	appData := os.Getenv("APPDATA")
	if appData == "" {
		appData = filepath.Join(homeDir, "AppData", "Roaming")
	}

	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		localAppData = filepath.Join(homeDir, "AppData", "Local")
	}

	return &PathConfig{
		ModelDir:      filepath.Join(programData, "OffGrid", "models"),
		SessionDir:    filepath.Join(appData, "OffGrid", "sessions"),
		ConfigDir:     filepath.Join(programData, "OffGrid", "config"),
		BinaryDir:     filepath.Join(programData, "OffGrid", "bin"),
		WebDir:        filepath.Join(programData, "OffGrid", "web"),
		LogDir:        filepath.Join(localAppData, "OffGrid", "logs"),
		RuntimeDir:    filepath.Join(localAppData, "OffGrid", "run"),
		ActiveModel:   filepath.Join(programData, "OffGrid", "config", "active-model"),
		LlamaPortFile: filepath.Join(programData, "OffGrid", "config", "llama-port"),
	}
}

// EnsureDirectories creates all necessary directories
func (p *PathConfig) EnsureDirectories() error {
	dirs := []string{
		p.ModelDir,
		p.SessionDir,
		p.ConfigDir,
		p.WebDir,
		p.LogDir,
		p.RuntimeDir,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	return nil
}

// GetExecutablePath returns the path to an executable for the current platform
func GetExecutablePath(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

// GetServiceName returns the platform-appropriate service name
func GetServiceName(baseName string) string {
	switch runtime.GOOS {
	case "darwin":
		return "com.offgrid." + baseName
	case "windows":
		return "OffGrid-" + baseName
	default:
		return baseName + ".service"
	}
}

// ReadLlamaPort returns the configured llama-server port for this platform.
func ReadLlamaPort(defaultPort string) string {
	for _, path := range llamaPortCandidates() {
		if data, err := os.ReadFile(path); err == nil {
			if port := strings.TrimSpace(string(data)); port != "" {
				return port
			}
		}
	}
	return defaultPort
}

func llamaPortCandidates() []string {
	paths := []string{GetDefaultPaths().LlamaPortFile}
	homeDir, err := os.UserHomeDir()
	if err != nil || homeDir == "" {
		return paths
	}

	switch runtime.GOOS {
	case "darwin":
		paths = append(paths, filepath.Join(homeDir, "Library", "Application Support", "OffGrid", "llama-port"))
	case "windows":
		paths = append(paths, filepath.Join(homeDir, "AppData", "Roaming", "OffGrid", "llama-port"))
	default:
		paths = append(paths, filepath.Join(homeDir, ".config", "offgrid", "llama-port"))
	}

	return paths
}
