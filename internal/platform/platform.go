package platform

import (
	"os"
	"path/filepath"
	"runtime"
)

// Platform represents the current operating system and architecture
type Platform struct {
	OS   string
	Arch string
}

// GetPlatform returns the current platform information
func GetPlatform() Platform {
	return Platform{
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
	}
}

// IsLinux returns true if running on Linux
func IsLinux() bool {
	return runtime.GOOS == "linux"
}

// IsDarwin returns true if running on macOS
func IsDarwin() bool {
	return runtime.GOOS == "darwin"
}

// IsWindows returns true if running on Windows
func IsWindows() bool {
	return runtime.GOOS == "windows"
}

// GetDefaultInstallPath returns the default installation path for binaries
func GetDefaultInstallPath() string {
	switch runtime.GOOS {
	case "darwin":
		return "/usr/local/bin"
	case "windows":
		return filepath.Join(os.Getenv("ProgramFiles"), "OffGrid")
	default: // linux
		return "/usr/local/bin"
	}
}

// GetConfigPath returns the platform-specific configuration directory
func GetConfigPath() string {
	// Check for user override
	if configDir := os.Getenv("OFFGRID_CONFIG_DIR"); configDir != "" {
		return configDir
	}

	switch runtime.GOOS {
	case "darwin":
		// macOS: ~/Library/Application Support/OffGrid
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Application Support", "OffGrid")
	case "windows":
		// Windows: %APPDATA%\OffGrid
		return filepath.Join(os.Getenv("APPDATA"), "OffGrid")
	default: // linux
		// Linux: ~/.config/offgrid (XDG Base Directory)
		if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
			return filepath.Join(xdgConfig, "offgrid")
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".config", "offgrid")
	}
}

// GetDataPath returns the platform-specific data directory
func GetDataPath() string {
	// Check for user override
	if dataDir := os.Getenv("OFFGRID_DATA_DIR"); dataDir != "" {
		return dataDir
	}

	switch runtime.GOOS {
	case "darwin":
		// macOS: ~/Library/Application Support/OffGrid/Data
		return filepath.Join(GetConfigPath(), "Data")
	case "windows":
		// Windows: %LOCALAPPDATA%\OffGrid
		return filepath.Join(os.Getenv("LOCALAPPDATA"), "OffGrid")
	default: // linux
		// Linux: ~/.local/share/offgrid (XDG Base Directory)
		if xdgData := os.Getenv("XDG_DATA_HOME"); xdgData != "" {
			return filepath.Join(xdgData, "offgrid")
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".local", "share", "offgrid")
	}
}

// GetCachePath returns the platform-specific cache directory
func GetCachePath() string {
	// Check for user override
	if cacheDir := os.Getenv("OFFGRID_CACHE_DIR"); cacheDir != "" {
		return cacheDir
	}

	switch runtime.GOOS {
	case "darwin":
		// macOS: ~/Library/Caches/OffGrid
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Caches", "OffGrid")
	case "windows":
		// Windows: %LOCALAPPDATA%\OffGrid\Cache
		return filepath.Join(os.Getenv("LOCALAPPDATA"), "OffGrid", "Cache")
	default: // linux
		// Linux: ~/.cache/offgrid (XDG Base Directory)
		if xdgCache := os.Getenv("XDG_CACHE_HOME"); xdgCache != "" {
			return filepath.Join(xdgCache, "offgrid")
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".cache", "offgrid")
	}
}

// GetModelsPath returns the default path for storing models
func GetModelsPath() string {
	return filepath.Join(GetDataPath(), "models")
}

// GetLogsPath returns the default path for log files
func GetLogsPath() string {
	switch runtime.GOOS {
	case "darwin":
		// macOS: ~/Library/Logs/OffGrid
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Logs", "OffGrid")
	case "windows":
		// Windows: %LOCALAPPDATA%\OffGrid\Logs
		return filepath.Join(os.Getenv("LOCALAPPDATA"), "OffGrid", "Logs")
	default: // linux
		// Linux: ~/.local/state/offgrid (systemd journal integration)
		if xdgState := os.Getenv("XDG_STATE_HOME"); xdgState != "" {
			return filepath.Join(xdgState, "offgrid")
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".local", "state", "offgrid")
	}
}

// GetExecutableName returns the executable name with platform-specific extension
func GetExecutableName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

// EnsureDirectories creates all platform-specific directories
func EnsureDirectories() error {
	dirs := []string{
		GetConfigPath(),
		GetDataPath(),
		GetCachePath(),
		GetModelsPath(),
		GetLogsPath(),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	return nil
}

// GetServiceName returns the platform-specific service name
func GetServiceName() string {
	switch runtime.GOOS {
	case "darwin":
		return "com.offgrid.llm"
	case "windows":
		return "OffGridLLM"
	default: // linux
		return "offgrid-llm"
	}
}

// GetServiceManagerType returns the service manager for the platform
func GetServiceManagerType() string {
	switch runtime.GOOS {
	case "darwin":
		return "launchd"
	case "windows":
		return "windows-service"
	default: // linux
		return "systemd"
	}
}
