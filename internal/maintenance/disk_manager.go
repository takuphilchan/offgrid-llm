// Package maintenance provides disk space management and cleanup
// for long-running edge deployments with limited storage.
package maintenance

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// DiskManager handles disk space monitoring and cleanup
type DiskManager struct {
	mu              sync.RWMutex
	dataDir         string
	modelsDir       string
	logsDir         string
	cacheDir        string
	minFreeSpaceGB  int64 // Minimum free space to maintain
	maxLogAgeDays   int   // Delete logs older than this
	maxCacheAgeDays int   // Delete cache older than this
	cleanupInterval time.Duration
	stopChan        chan struct{}
	logger          *log.Logger
	onCleanup       func(CleanupReport)
}

// CleanupReport contains information about a cleanup operation
type CleanupReport struct {
	Timestamp    time.Time     `json:"timestamp"`
	BytesFreed   int64         `json:"bytes_freed"`
	FilesDeleted int           `json:"files_deleted"`
	LogsDeleted  int           `json:"logs_deleted"`
	CacheCleared int64         `json:"cache_cleared"`
	Errors       []string      `json:"errors,omitempty"`
	SpaceBefore  DiskSpaceInfo `json:"space_before"`
	SpaceAfter   DiskSpaceInfo `json:"space_after"`
}

// DiskSpaceInfo contains disk space statistics
type DiskSpaceInfo struct {
	TotalGB     float64 `json:"total_gb"`
	UsedGB      float64 `json:"used_gb"`
	FreeGB      float64 `json:"free_gb"`
	UsedPercent float64 `json:"used_percent"`
}

// FileInfo represents a file that could be cleaned up
type FileInfo struct {
	Path     string    `json:"path"`
	Size     int64     `json:"size"`
	ModTime  time.Time `json:"mod_time"`
	Category string    `json:"category"` // "log", "cache", "temp", "model"
}

// Config for DiskManager
type DiskManagerConfig struct {
	DataDir         string
	ModelsDir       string
	LogsDir         string
	CacheDir        string
	MinFreeSpaceGB  int64
	MaxLogAgeDays   int
	MaxCacheAgeDays int
	CleanupInterval time.Duration
}

// DefaultDiskManagerConfig returns sensible defaults for edge devices
func DefaultDiskManagerConfig(baseDir string) DiskManagerConfig {
	return DiskManagerConfig{
		DataDir:         baseDir,
		ModelsDir:       filepath.Join(baseDir, "models"),
		LogsDir:         filepath.Join(baseDir, "logs"),
		CacheDir:        filepath.Join(baseDir, "cache"),
		MinFreeSpaceGB:  2,             // Keep at least 2GB free
		MaxLogAgeDays:   7,             // Delete logs older than 7 days
		MaxCacheAgeDays: 3,             // Delete cache older than 3 days
		CleanupInterval: 1 * time.Hour, // Check hourly
	}
}

// NewDiskManager creates a new disk space manager
func NewDiskManager(config DiskManagerConfig) *DiskManager {
	return &DiskManager{
		dataDir:         config.DataDir,
		modelsDir:       config.ModelsDir,
		logsDir:         config.LogsDir,
		cacheDir:        config.CacheDir,
		minFreeSpaceGB:  config.MinFreeSpaceGB,
		maxLogAgeDays:   config.MaxLogAgeDays,
		maxCacheAgeDays: config.MaxCacheAgeDays,
		cleanupInterval: config.CleanupInterval,
		stopChan:        make(chan struct{}),
		logger:          log.New(os.Stdout, "[DISK] ", log.LstdFlags),
	}
}

// SetLogger sets a custom logger
func (dm *DiskManager) SetLogger(logger *log.Logger) {
	dm.logger = logger
}

// OnCleanup sets a callback for cleanup operations
func (dm *DiskManager) OnCleanup(callback func(CleanupReport)) {
	dm.onCleanup = callback
}

// Start begins automatic disk space monitoring
func (dm *DiskManager) Start() {
	dm.logger.Println("Starting disk space manager...")

	// Initial check
	go dm.checkAndClean()

	// Periodic checks
	ticker := time.NewTicker(dm.cleanupInterval)
	go func() {
		for {
			select {
			case <-dm.stopChan:
				ticker.Stop()
				return
			case <-ticker.C:
				dm.checkAndClean()
			}
		}
	}()
}

// Stop stops the disk manager
func (dm *DiskManager) Stop() {
	close(dm.stopChan)
	dm.logger.Println("Disk space manager stopped")
}

// GetDiskSpace returns current disk space information
func (dm *DiskManager) GetDiskSpace() (DiskSpaceInfo, error) {
	return getDiskSpaceInfo(dm.dataDir)
}

// GetCleanableFiles returns files that can be safely deleted
func (dm *DiskManager) GetCleanableFiles() ([]FileInfo, error) {
	var files []FileInfo

	// Scan logs directory
	if dm.logsDir != "" {
		logFiles, err := dm.scanDirectory(dm.logsDir, "log", dm.maxLogAgeDays)
		if err == nil {
			files = append(files, logFiles...)
		}
	}

	// Scan cache directory
	if dm.cacheDir != "" {
		cacheFiles, err := dm.scanDirectory(dm.cacheDir, "cache", dm.maxCacheAgeDays)
		if err == nil {
			files = append(files, cacheFiles...)
		}
	}

	// Scan for temp files
	tempFiles, err := dm.scanTempFiles()
	if err == nil {
		files = append(files, tempFiles...)
	}

	// Sort by size (largest first)
	sort.Slice(files, func(i, j int) bool {
		return files[i].Size > files[j].Size
	})

	return files, nil
}

// CleanupNow performs an immediate cleanup
func (dm *DiskManager) CleanupNow() CleanupReport {
	return dm.performCleanup()
}

// CleanupToFreeSpace cleans up files until we have enough free space
func (dm *DiskManager) CleanupToFreeSpace(targetFreeGB int64) CleanupReport {
	report := CleanupReport{
		Timestamp: time.Now(),
	}

	spaceBefore, err := dm.GetDiskSpace()
	if err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("Failed to get disk space: %v", err))
		return report
	}
	report.SpaceBefore = spaceBefore

	if spaceBefore.FreeGB >= float64(targetFreeGB) {
		dm.logger.Printf("Already have %.1f GB free (target: %d GB)", spaceBefore.FreeGB, targetFreeGB)
		report.SpaceAfter = spaceBefore
		return report
	}

	neededBytes := (targetFreeGB - int64(spaceBefore.FreeGB)) * 1024 * 1024 * 1024
	dm.logger.Printf("Need to free approximately %.1f GB", float64(neededBytes)/(1024*1024*1024))

	// Get cleanable files
	files, err := dm.GetCleanableFiles()
	if err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("Failed to scan files: %v", err))
		return report
	}

	// Delete files until we have enough space
	var freedBytes int64
	for _, file := range files {
		if freedBytes >= neededBytes {
			break
		}

		if err := os.Remove(file.Path); err != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("Failed to delete %s: %v", file.Path, err))
			continue
		}

		freedBytes += file.Size
		report.FilesDeleted++
		report.BytesFreed += file.Size
		dm.logger.Printf("Deleted: %s (%.1f MB)", file.Path, float64(file.Size)/(1024*1024))
	}

	report.SpaceAfter, _ = dm.GetDiskSpace()
	return report
}

func (dm *DiskManager) checkAndClean() {
	space, err := dm.GetDiskSpace()
	if err != nil {
		dm.logger.Printf("Failed to check disk space: %v", err)
		return
	}

	dm.logger.Printf("Disk space: %.1f GB free / %.1f GB total (%.1f%% used)",
		space.FreeGB, space.TotalGB, space.UsedPercent)

	// Check if we need to clean up
	if space.FreeGB < float64(dm.minFreeSpaceGB) {
		dm.logger.Printf("Low disk space! Cleaning up to maintain %d GB free...", dm.minFreeSpaceGB)
		report := dm.CleanupToFreeSpace(dm.minFreeSpaceGB)
		if dm.onCleanup != nil {
			dm.onCleanup(report)
		}
	} else {
		// Still do routine cleanup of old files
		report := dm.performCleanup()
		if report.FilesDeleted > 0 && dm.onCleanup != nil {
			dm.onCleanup(report)
		}
	}
}

func (dm *DiskManager) performCleanup() CleanupReport {
	report := CleanupReport{
		Timestamp: time.Now(),
	}

	report.SpaceBefore, _ = dm.GetDiskSpace()

	// Clean old logs
	if dm.logsDir != "" {
		deleted, bytes, errs := dm.cleanOldFiles(dm.logsDir, dm.maxLogAgeDays)
		report.LogsDeleted = deleted
		report.BytesFreed += bytes
		report.Errors = append(report.Errors, errs...)
	}

	// Clean old cache
	if dm.cacheDir != "" {
		deleted, bytes, errs := dm.cleanOldFiles(dm.cacheDir, dm.maxCacheAgeDays)
		report.CacheCleared = bytes
		report.FilesDeleted += deleted
		report.BytesFreed += bytes
		report.Errors = append(report.Errors, errs...)
	}

	// Clean temp files
	deleted, bytes, errs := dm.cleanTempFiles()
	report.FilesDeleted += deleted
	report.BytesFreed += bytes
	report.Errors = append(report.Errors, errs...)

	report.SpaceAfter, _ = dm.GetDiskSpace()

	if report.FilesDeleted > 0 {
		dm.logger.Printf("Cleanup complete: deleted %d files, freed %.1f MB",
			report.FilesDeleted, float64(report.BytesFreed)/(1024*1024))
	}

	return report
}

func (dm *DiskManager) scanDirectory(dir, category string, maxAgeDays int) ([]FileInfo, error) {
	var files []FileInfo
	cutoff := time.Now().AddDate(0, 0, -maxAgeDays)

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if d.IsDir() {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		if info.ModTime().Before(cutoff) {
			files = append(files, FileInfo{
				Path:     path,
				Size:     info.Size(),
				ModTime:  info.ModTime(),
				Category: category,
			})
		}

		return nil
	})

	return files, err
}

func (dm *DiskManager) scanTempFiles() ([]FileInfo, error) {
	var files []FileInfo

	// Scan for .tmp files in data directory
	err := filepath.WalkDir(dm.dataDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}

		// Match temp file patterns
		name := d.Name()
		if strings.HasSuffix(name, ".tmp") ||
			strings.HasSuffix(name, ".partial") ||
			strings.HasPrefix(name, "~") ||
			strings.HasSuffix(name, ".download") {

			info, err := d.Info()
			if err != nil {
				return nil
			}

			// Only delete temp files older than 1 hour
			if time.Since(info.ModTime()) > time.Hour {
				files = append(files, FileInfo{
					Path:     path,
					Size:     info.Size(),
					ModTime:  info.ModTime(),
					Category: "temp",
				})
			}
		}

		return nil
	})

	return files, err
}

func (dm *DiskManager) cleanOldFiles(dir string, maxAgeDays int) (int, int64, []string) {
	var deleted int
	var bytesFreed int64
	var errors []string

	cutoff := time.Now().AddDate(0, 0, -maxAgeDays)

	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		if info.ModTime().Before(cutoff) {
			if err := os.Remove(path); err != nil {
				errors = append(errors, fmt.Sprintf("Failed to delete %s: %v", path, err))
			} else {
				deleted++
				bytesFreed += info.Size()
			}
		}

		return nil
	})

	return deleted, bytesFreed, errors
}

func (dm *DiskManager) cleanTempFiles() (int, int64, []string) {
	files, err := dm.scanTempFiles()
	if err != nil {
		return 0, 0, []string{fmt.Sprintf("Failed to scan temp files: %v", err)}
	}

	var deleted int
	var bytesFreed int64
	var errors []string

	for _, file := range files {
		if err := os.Remove(file.Path); err != nil {
			errors = append(errors, fmt.Sprintf("Failed to delete %s: %v", file.Path, err))
		} else {
			deleted++
			bytesFreed += file.Size
		}
	}

	return deleted, bytesFreed, errors
}

// RotateLogs rotates log files, keeping only the most recent ones
func (dm *DiskManager) RotateLogs(maxFiles int) error {
	if dm.logsDir == "" {
		return nil
	}

	var logFiles []FileInfo

	filepath.WalkDir(dm.logsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}

		if strings.HasSuffix(d.Name(), ".log") {
			info, _ := d.Info()
			if info != nil {
				logFiles = append(logFiles, FileInfo{
					Path:    path,
					Size:    info.Size(),
					ModTime: info.ModTime(),
				})
			}
		}
		return nil
	})

	// Sort by mod time (newest first)
	sort.Slice(logFiles, func(i, j int) bool {
		return logFiles[i].ModTime.After(logFiles[j].ModTime)
	})

	// Delete files beyond maxFiles
	for i := maxFiles; i < len(logFiles); i++ {
		if err := os.Remove(logFiles[i].Path); err != nil {
			dm.logger.Printf("Failed to rotate log %s: %v", logFiles[i].Path, err)
		} else {
			dm.logger.Printf("Rotated log: %s", logFiles[i].Path)
		}
	}

	return nil
}
