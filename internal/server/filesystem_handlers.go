package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// handleFilesystemBrowse allows browsing the filesystem for path selection
func (s *Server) handleFilesystemBrowse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Path string `json:"path"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Default to user's home directory if no path provided
	browsePath := req.Path
	if browsePath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			browsePath = "/"
		} else {
			browsePath = homeDir
		}
	}

	// Clean the path
	browsePath = filepath.Clean(browsePath)

	// Check if path exists, fallback to home or root if not
	info, err := os.Stat(browsePath)
	if err != nil {
		// Try home directory as fallback
		homeDir, homeErr := os.UserHomeDir()
		if homeErr == nil {
			browsePath = homeDir
			info, err = os.Stat(browsePath)
		}
		// If still failing, try root
		if err != nil {
			browsePath = "/"
			info, err = os.Stat(browsePath)
			if err != nil {
				writeError(w, fmt.Sprintf("Cannot access filesystem: %v", err), http.StatusInternalServerError)
				return
			}
		}
	}

	// If it's a file, use its directory
	if !info.IsDir() {
		browsePath = filepath.Dir(browsePath)
	}

	// Read directory contents
	entries, err := os.ReadDir(browsePath)
	if err != nil {
		writeError(w, fmt.Sprintf("Cannot read directory: %v", err), http.StatusInternalServerError)
		return
	}

	type FileEntry struct {
		Name        string `json:"name"`
		Path        string `json:"path"`
		IsDirectory bool   `json:"is_directory"`
		Size        int64  `json:"size,omitempty"`
		ModTime     string `json:"mod_time,omitempty"`
		IsHidden    bool   `json:"is_hidden"`
	}

	var files []FileEntry
	var dirs []FileEntry

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		name := entry.Name()
		fullPath := filepath.Join(browsePath, name)
		isHidden := strings.HasPrefix(name, ".")

		fileEntry := FileEntry{
			Name:        name,
			Path:        fullPath,
			IsDirectory: entry.IsDir(),
			Size:        info.Size(),
			ModTime:     info.ModTime().Format("2006-01-02 15:04:05"),
			IsHidden:    isHidden,
		}

		if entry.IsDir() {
			dirs = append(dirs, fileEntry)
		} else {
			files = append(files, fileEntry)
		}
	}

	// Get parent directory
	parentDir := filepath.Dir(browsePath)
	if parentDir == browsePath {
		parentDir = "" // At root
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"current_path": browsePath,
		"parent_path":  parentDir,
		"directories":  dirs,
		"files":        files,
	})
}

// handleCommonPaths returns common mount points and paths for the current OS
func (s *Server) handleCommonPaths(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	type PathEntry struct {
		Path        string `json:"path"`
		Label       string `json:"label"`
		Description string `json:"description"`
		Exists      bool   `json:"exists"`
	}

	var commonPaths []PathEntry

	// Add home directory
	if homeDir, err := os.UserHomeDir(); err == nil {
		commonPaths = append(commonPaths, PathEntry{
			Path:        homeDir,
			Label:       "Home",
			Description: "User home directory",
			Exists:      true,
		})
	}

	// Add models directory
	modelsDir := s.config.ModelsDir
	_, err := os.Stat(modelsDir)
	commonPaths = append(commonPaths, PathEntry{
		Path:        modelsDir,
		Label:       "Models Directory",
		Description: "OffGrid models storage",
		Exists:      err == nil,
	})

	// Platform-specific common paths
	if runtime.GOOS == "linux" {
		// Linux USB mount points
		mediaPaths := []string{"/media", "/mnt", "/run/media"}
		for _, basePath := range mediaPaths {
			entries, err := os.ReadDir(basePath)
			if err == nil {
				for _, entry := range entries {
					if entry.IsDir() {
						fullPath := filepath.Join(basePath, entry.Name())
						commonPaths = append(commonPaths, PathEntry{
							Path:        fullPath,
							Label:       fmt.Sprintf("USB: %s", entry.Name()),
							Description: fmt.Sprintf("Removable storage at %s", fullPath),
							Exists:      true,
						})
					}
				}
			}
		}
		// Add common base paths even if empty
		commonPaths = append(commonPaths, PathEntry{
			Path:        "/media",
			Label:       "/media",
			Description: "Linux media mount point",
			Exists:      true,
		})
		commonPaths = append(commonPaths, PathEntry{
			Path:        "/mnt",
			Label:       "/mnt",
			Description: "Linux mount point",
			Exists:      true,
		})
	} else if runtime.GOOS == "darwin" {
		// macOS Volumes
		volumesPath := "/Volumes"
		entries, err := os.ReadDir(volumesPath)
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() && entry.Name() != "Macintosh HD" {
					fullPath := filepath.Join(volumesPath, entry.Name())
					commonPaths = append(commonPaths, PathEntry{
						Path:        fullPath,
						Label:       fmt.Sprintf("Volume: %s", entry.Name()),
						Description: fmt.Sprintf("Mounted volume at %s", fullPath),
						Exists:      true,
					})
				}
			}
		}
		commonPaths = append(commonPaths, PathEntry{
			Path:        "/Volumes",
			Label:       "/Volumes",
			Description: "macOS volumes directory",
			Exists:      true,
		})
	} else if runtime.GOOS == "windows" {
		// Windows drive letters
		for drive := 'C'; drive <= 'Z'; drive++ {
			drivePath := fmt.Sprintf("%c:\\", drive)
			if _, err := os.Stat(drivePath); err == nil {
				label := fmt.Sprintf("Drive %c:", drive)
				desc := "Local drive"
				if drive > 'C' {
					desc = "Removable/External drive"
				}
				commonPaths = append(commonPaths, PathEntry{
					Path:        drivePath,
					Label:       label,
					Description: desc,
					Exists:      true,
				})
			}
		}
	}

	// Add root
	commonPaths = append(commonPaths, PathEntry{
		Path:        "/",
		Label:       "Root",
		Description: "System root directory",
		Exists:      true,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"os":    runtime.GOOS,
		"paths": commonPaths,
	})
}
