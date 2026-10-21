package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/cli"
	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/internal/resource"
)

func handleDoctorReadOnly(ctx context.Context, _ []string) int {
	return executeCommand(ctx, func(ctx context.Context) error {
		snapshot, err := observeDoctor(ctx, config.LoadConfig(), resource.DetectResources, &http.Client{Timeout: 5 * time.Second}, filepath.Join(os.Getenv("HOME"), ".offgrid-llm", "config.json"))
		if err != nil {
			return err
		}
		legacyPresenter().Doctor(snapshot)
		return nil
	})
}

// observeDoctor intentionally probes storage by reading directory entries only.
// A missing models directory is reported, never created as a side effect.
func observeDoctor(ctx context.Context, cfg *config.Config, detect func() (*resource.SystemResources, error), client *http.Client, configPath string) (cli.DoctorSnapshot, error) {
	if cfg == nil || detect == nil || client == nil {
		return cli.DoctorSnapshot{}, fmt.Errorf("doctor observer dependencies unavailable")
	}
	snapshot := cli.DoctorSnapshot{ModelsDir: cfg.ModelsDir, ServerPort: cfg.ServerPort, ConfigPath: configPath}
	res, err := detect()
	if err != nil {
		snapshot.ResourcesError = err
	} else {
		snapshot.OS, snapshot.Arch, snapshot.CPUCores = res.OS, res.Arch, res.CPUCores
		snapshot.RAMBytes = res.TotalRAM * 1024 * 1024
		if res.GPUAvailable {
			snapshot.GPUName = res.GPUName
			snapshot.GPUMemoryBytes = res.GPUMemory * 1024 * 1024
		}
	}
	if _, err := os.Stat(configPath); err == nil {
		snapshot.ConfigPresent = true
	}
	if stat, err := os.Stat(cfg.ModelsDir); err == nil && stat.IsDir() {
		snapshot.StoragePresent = true
		snapshot.ModelScanError = filepath.WalkDir(cfg.ModelsDir, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() && entry.Name() == models.PackageDirectory {
				return filepath.SkipDir
			}
			if !entry.IsDir() && !strings.HasSuffix(path, ".tmp") {
				switch filepath.Ext(path) {
				case ".gguf", ".ggml", ".bin":
					snapshot.ModelCount++
				}
			}
			return nil
		})
	}
	snapshot.NetworkOnline = doctorHTTPStatus(ctx, client, "https://huggingface.co")
	snapshot.ServerOnline = doctorHTTPStatus(ctx, client, fmt.Sprintf("http://localhost:%d/health", cfg.ServerPort))
	return snapshot, ctx.Err()
}

func doctorHTTPStatus(ctx context.Context, client *http.Client, address string) bool {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return false
	}
	response, err := client.Do(request)
	if err != nil {
		return false
	}
	defer response.Body.Close()
	return response.StatusCode == http.StatusOK
}
