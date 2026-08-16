// Package config provides configuration export/import for fleet deployment.
// This allows operators to configure one OffGrid instance and deploy
// the same configuration across multiple edge devices.
package config

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// ExportManifest contains metadata about the exported configuration
type ExportManifest struct {
	Version       string            `json:"version"`
	ExportedAt    string            `json:"exported_at"`
	ExportedFrom  string            `json:"exported_from"`
	Description   string            `json:"description,omitempty"`
	Files         []ExportedFile    `json:"files"`
	Checksums     map[string]string `json:"checksums"`
	ModelRegistry []ModelReference  `json:"model_registry,omitempty"`
}

// ExportedFile describes a file in the export
type ExportedFile struct {
	Path        string `json:"path"`
	Type        string `json:"type"` // "config", "template", "prompt"
	Description string `json:"description,omitempty"`
}

// ModelReference describes a model without including the actual file
type ModelReference struct {
	Name     string `json:"name"`
	Filename string `json:"filename"`
	SHA256   string `json:"sha256,omitempty"`
	Size     int64  `json:"size,omitempty"`
	Source   string `json:"source,omitempty"`
}

// ExportOptions configures what to include in the export
type ExportOptions struct {
	IncludeConfig    bool     `json:"include_config"`
	IncludeTemplates bool     `json:"include_templates"`
	IncludePrompts   bool     `json:"include_prompts"`
	IncludeUsers     bool     `json:"include_users"`
	IncludeHashes    bool     `json:"include_hashes"`
	ExcludePatterns  []string `json:"exclude_patterns,omitempty"`
	Description      string   `json:"description,omitempty"`
}

// DefaultExportOptions returns sensible defaults
func DefaultExportOptions() ExportOptions {
	return ExportOptions{
		IncludeConfig:    true,
		IncludeTemplates: true,
		IncludePrompts:   true,
		IncludeUsers:     false, // Sensitive by default
		IncludeHashes:    true,
	}
}

// Exporter handles configuration export
type Exporter struct {
	dataDir    string
	configPath string
	hostname   string
	version    string
}

// NewExporter creates a new configuration exporter
func NewExporter(dataDir, configPath, version string) *Exporter {
	hostname, _ := os.Hostname()
	return &Exporter{
		dataDir:    dataDir,
		configPath: configPath,
		hostname:   hostname,
		version:    version,
	}
}

// Export creates a configuration bundle
func (e *Exporter) Export(outputPath string, opts ExportOptions) error {
	// Create output file
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create export file: %w", err)
	}
	defer file.Close()

	// Create gzip writer
	gw := gzip.NewWriter(file)
	defer gw.Close()

	// Create tar writer
	tw := tar.NewWriter(gw)
	defer tw.Close()

	manifest := ExportManifest{
		Version:      e.version,
		ExportedAt:   time.Now().Format(time.RFC3339),
		ExportedFrom: e.hostname,
		Description:  opts.Description,
		Files:        []ExportedFile{},
		Checksums:    make(map[string]string),
	}

	// Export main config
	if opts.IncludeConfig && e.configPath != "" {
		if err := e.addFile(tw, e.configPath, "config/config.yaml", "config", &manifest); err != nil {
			// Config file is optional
			if !os.IsNotExist(err) {
				return err
			}
		}
	}

	// Export templates directory
	if opts.IncludeTemplates {
		templatesDir := filepath.Join(e.dataDir, "templates")
		if err := e.addDirectory(tw, templatesDir, "templates", "template", &manifest, opts.ExcludePatterns); err != nil {
			// Templates directory is optional
			if !os.IsNotExist(err) {
				return err
			}
		}
	}

	// Export prompts directory
	if opts.IncludePrompts {
		promptsDir := filepath.Join(e.dataDir, "prompts")
		if err := e.addDirectory(tw, promptsDir, "prompts", "prompt", &manifest, opts.ExcludePatterns); err != nil {
			// Prompts directory is optional
			if !os.IsNotExist(err) {
				return err
			}
		}
	}

	// Export users (if enabled)
	if opts.IncludeUsers {
		usersPath := filepath.Join(e.dataDir, "users.json")
		if err := e.addFile(tw, usersPath, "config/users.json", "config", &manifest); err != nil {
			// Users file is optional
			if !os.IsNotExist(err) {
				return err
			}
		}
	}

	// Export model hashes
	if opts.IncludeHashes {
		hashesPath := filepath.Join(e.dataDir, "model_hashes.json")
		if err := e.addFile(tw, hashesPath, "config/model_hashes.json", "config", &manifest); err != nil {
			// Hashes file is optional
			if !os.IsNotExist(err) {
				return err
			}
		}
	}

	// Build model registry from models directory
	modelsDir := filepath.Join(e.dataDir, "models")
	if models, err := e.scanModels(modelsDir); err == nil {
		manifest.ModelRegistry = models
	}

	// Write manifest
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to create manifest: %w", err)
	}

	manifestHeader := &tar.Header{
		Name:    "manifest.json",
		Mode:    0644,
		Size:    int64(len(manifestData)),
		ModTime: time.Now(),
	}
	if err := tw.WriteHeader(manifestHeader); err != nil {
		return err
	}
	if _, err := tw.Write(manifestData); err != nil {
		return err
	}

	return nil
}

// addFile adds a single file to the tar archive
func (e *Exporter) addFile(tw *tar.Writer, sourcePath, archivePath, fileType string, manifest *ExportManifest) error {
	file, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	header := &tar.Header{
		Name:    archivePath,
		Mode:    int64(info.Mode()),
		Size:    info.Size(),
		ModTime: info.ModTime(),
	}

	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	if _, err := io.Copy(tw, file); err != nil {
		return err
	}

	manifest.Files = append(manifest.Files, ExportedFile{
		Path: archivePath,
		Type: fileType,
	})

	return nil
}

// addDirectory adds all files in a directory to the tar archive
func (e *Exporter) addDirectory(tw *tar.Writer, sourceDir, archivePrefix, fileType string, manifest *ExportManifest, excludePatterns []string) error {
	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Check exclude patterns
		for _, pattern := range excludePatterns {
			if matched, _ := filepath.Match(pattern, info.Name()); matched {
				return nil
			}
		}

		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}

		archivePath := filepath.Join(archivePrefix, relPath)
		return e.addFile(tw, path, archivePath, fileType, manifest)
	})
}

// scanModels scans the models directory and returns model references
func (e *Exporter) scanModels(modelsDir string) ([]ModelReference, error) {
	var models []ModelReference

	err := filepath.Walk(modelsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(info.Name()), ".gguf") {
			return nil
		}

		models = append(models, ModelReference{
			Name:     strings.TrimSuffix(info.Name(), filepath.Ext(info.Name())),
			Filename: info.Name(),
			Size:     info.Size(),
		})
		return nil
	})

	return models, err
}

// Importer handles configuration import
type Importer struct {
	dataDir    string
	configPath string
	backup     bool
}

// NewImporter creates a new configuration importer
func NewImporter(dataDir, configPath string, backup bool) *Importer {
	return &Importer{
		dataDir:    dataDir,
		configPath: configPath,
		backup:     backup,
	}
}

// ImportResult contains the result of an import operation
type ImportResult struct {
	FilesImported int              `json:"files_imported"`
	BackupPath    string           `json:"backup_path,omitempty"`
	Manifest      *ExportManifest  `json:"manifest"`
	ModelsNeeded  []ModelReference `json:"models_needed,omitempty"`
	Errors        []string         `json:"errors,omitempty"`
}

// Import imports a configuration bundle
func (i *Importer) Import(bundlePath string) (*ImportResult, error) {
	result := &ImportResult{}
	const maxBundleEntries = 10000
	const maxExpandedBytes int64 = 2 << 30
	var expandedBytes int64
	entryCount := 0

	// Open the bundle
	file, err := os.Open(bundlePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open bundle: %w", err)
	}
	defer file.Close()

	// Create gzip reader
	gr, err := gzip.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read gzip: %w", err)
	}
	defer gr.Close()

	// Create tar reader
	tr := tar.NewReader(gr)

	// First pass: read manifest
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		entryCount++
		if entryCount > maxBundleEntries || header.Size < 0 || header.Size > maxExpandedBytes-expandedBytes {
			return nil, fmt.Errorf("configuration bundle exceeds safety limits")
		}
		expandedBytes += header.Size

		if header.Name == "manifest.json" {
			if header.Size > 10<<20 {
				return nil, fmt.Errorf("manifest is too large")
			}
			var manifest ExportManifest
			if err := json.NewDecoder(tr).Decode(&manifest); err != nil {
				return nil, fmt.Errorf("failed to read manifest: %w", err)
			}
			result.Manifest = &manifest
			break
		}
	}

	if result.Manifest == nil {
		return nil, fmt.Errorf("no manifest found in bundle")
	}

	// Create backup if enabled
	if i.backup {
		backupPath, err := i.createBackup()
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("backup failed: %v", err))
		} else {
			result.BackupPath = backupPath
		}
	}

	// Reset file for second pass
	if _, err := file.Seek(0, 0); err != nil {
		return nil, err
	}
	gr.Close()
	gr, err = gzip.NewReader(file)
	if err != nil {
		return nil, err
	}
	tr = tar.NewReader(gr)
	expandedBytes = 0
	entryCount = 0

	// Second pass: extract files
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		entryCount++
		if entryCount > maxBundleEntries || header.Size < 0 || header.Size > maxExpandedBytes-expandedBytes {
			return nil, fmt.Errorf("configuration bundle exceeds safety limits")
		}
		expandedBytes += header.Size

		if header.Name == "manifest.json" {
			continue
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			return nil, fmt.Errorf("unsupported bundle entry type for %q", header.Name)
		}

		targetPath, err := i.resolveTargetPath(header.Name)
		if err != nil {
			return nil, err
		}
		if targetPath == "" {
			continue
		}

		// Create directory if needed
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("mkdir failed for %s: %v", header.Name, err))
			continue
		}

		// Extract file
		outFile, err := os.Create(targetPath)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("create failed for %s: %v", header.Name, err))
			continue
		}

		if _, err := io.Copy(outFile, tr); err != nil {
			outFile.Close()
			result.Errors = append(result.Errors, fmt.Sprintf("write failed for %s: %v", header.Name, err))
			continue
		}
		outFile.Close()

		// Set permissions
		os.Chmod(targetPath, 0600)

		result.FilesImported++
	}

	// Check for missing models
	result.ModelsNeeded = i.checkMissingModels(result.Manifest.ModelRegistry)

	return result, nil
}

// resolveTargetPath resolves the archive path to a local filesystem path
func (i *Importer) resolveTargetPath(archivePath string) (string, error) {
	parts := strings.SplitN(archivePath, "/", 2)
	if len(parts) < 2 {
		return "", nil
	}

	prefix := parts[0]
	rest := parts[1]

	switch prefix {
	case "config":
		if rest == "config.yaml" && i.configPath != "" {
			return i.configPath, nil
		}
		return safeImportTarget(i.dataDir, rest)
	case "templates":
		return safeImportTarget(filepath.Join(i.dataDir, "templates"), rest)
	case "prompts":
		return safeImportTarget(filepath.Join(i.dataDir, "prompts"), rest)
	default:
		return "", nil
	}
}

func safeImportTarget(root, name string) (string, error) {
	// Archive paths are slash-separated regardless of the host OS. Normalize
	// Windows separators before validation so a bundle cannot become safe or
	// unsafe depending on where it is inspected.
	archiveName := strings.ReplaceAll(name, "\\", "/")
	cleanArchive := path.Clean(archiveName)
	if cleanArchive == "." || cleanArchive == ".." ||
		strings.HasPrefix(cleanArchive, "/") || strings.HasPrefix(cleanArchive, "../") ||
		strings.Contains(cleanArchive, ":") {
		return "", fmt.Errorf("unsafe bundle path %q", name)
	}
	clean := filepath.FromSlash(cleanArchive)
	if filepath.IsAbs(clean) || filepath.VolumeName(clean) != "" {
		return "", fmt.Errorf("unsafe bundle path %q", name)
	}
	target := filepath.Join(root, clean)
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe bundle path %q", name)
	}
	return target, nil
}

// createBackup creates a backup of current configuration
func (i *Importer) createBackup() (string, error) {
	backupDir := filepath.Join(i.dataDir, "backups")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", err
	}

	backupName := fmt.Sprintf("config_backup_%s.tar.gz", time.Now().Format("20060102_150405"))
	backupPath := filepath.Join(backupDir, backupName)

	exporter := NewExporter(i.dataDir, i.configPath, "backup")
	if err := exporter.Export(backupPath, DefaultExportOptions()); err != nil {
		return "", err
	}

	return backupPath, nil
}

// checkMissingModels checks which models from the registry are missing locally
func (i *Importer) checkMissingModels(registry []ModelReference) []ModelReference {
	var missing []ModelReference
	modelsDir := filepath.Join(i.dataDir, "models")

	for _, model := range registry {
		modelPath := filepath.Join(modelsDir, model.Filename)
		if _, err := os.Stat(modelPath); os.IsNotExist(err) {
			missing = append(missing, model)
		}
	}

	return missing
}

// ListAvailableBackups lists all available configuration backups
func ListAvailableBackups(dataDir string) ([]string, error) {
	backupDir := filepath.Join(dataDir, "backups")
	var backups []string

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return backups, nil
		}
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".tar.gz") {
			backups = append(backups, filepath.Join(backupDir, entry.Name()))
		}
	}

	return backups, nil
}
