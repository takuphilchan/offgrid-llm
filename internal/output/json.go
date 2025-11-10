package output

import (
	"encoding/json"
	"fmt"
	"os"
)

// JSONMode controls whether output is JSON or human-readable
var JSONMode = false

// ModelInfo represents a model in JSON output
type ModelInfo struct {
	Name         string   `json:"name"`
	Size         string   `json:"size,omitempty"`
	Quantization string   `json:"quantization,omitempty"`
	Format       string   `json:"format,omitempty"`
	Tags         []string `json:"tags,omitempty"`
	Source       string   `json:"source,omitempty"`
	Path         string   `json:"path,omitempty"`
}

// SessionInfo represents a session in JSON output
type SessionInfo struct {
	Name      string `json:"name"`
	ModelID   string `json:"model_id"`
	Messages  int    `json:"messages"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// SearchResult represents a search result in JSON output
type SearchResult struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Downloads   int      `json:"downloads,omitempty"`
	Likes       int      `json:"likes,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	ModelID     string   `json:"model_id"`
}

// DownloadProgress represents download progress in JSON output
type DownloadProgress struct {
	Status     string  `json:"status"`
	Model      string  `json:"model"`
	Progress   float64 `json:"progress,omitempty"`
	Downloaded int64   `json:"downloaded_bytes,omitempty"`
	Total      int64   `json:"total_bytes,omitempty"`
	Speed      string  `json:"speed,omitempty"`
	Error      string  `json:"error,omitempty"`
	Message    string  `json:"message,omitempty"`
}

// SystemInfo represents system information in JSON output
type SystemInfo struct {
	CPU          string `json:"cpu"`
	Memory       string `json:"memory"`
	GPU          string `json:"gpu,omitempty"`
	OS           string `json:"os"`
	Architecture string `json:"architecture"`
}

// CommandResult represents a generic command result
type CommandResult struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// PrintJSON outputs data as JSON
func PrintJSON(data interface{}) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
		os.Exit(1)
	}
}

// Success outputs a success result
func Success(message string, data interface{}) {
	if JSONMode {
		PrintJSON(CommandResult{
			Success: true,
			Message: message,
			Data:    data,
		})
	} else {
		// Normal output handled by caller
	}
}

// Error outputs an error result
func Error(message string, err error) {
	if JSONMode {
		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		}
		PrintJSON(CommandResult{
			Success: false,
			Message: message,
			Error:   errMsg,
		})
		os.Exit(1)
	} else {
		// Normal error output handled by caller
	}
}

// PrintModels outputs a list of models
func PrintModels(models []ModelInfo) {
	if JSONMode {
		PrintJSON(map[string]interface{}{
			"models": models,
			"count":  len(models),
		})
	}
}

// PrintSessions outputs a list of sessions
func PrintSessions(sessions []SessionInfo) {
	if JSONMode {
		PrintJSON(map[string]interface{}{
			"sessions": sessions,
			"count":    len(sessions),
		})
	}
}

// PrintSearchResults outputs search results
func PrintSearchResults(results []SearchResult) {
	if JSONMode {
		PrintJSON(map[string]interface{}{
			"results": results,
			"count":   len(results),
		})
	}
}

// PrintSystemInfo outputs system information
func PrintSystemInfo(info SystemInfo) {
	if JSONMode {
		PrintJSON(info)
	}
}
