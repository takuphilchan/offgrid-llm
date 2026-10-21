package main

import (
	"context"
	"os"
	"runtime"

	"github.com/takuphilchan/offgrid-llm/internal/cli"
	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/models"
)

func handleInfoReadOnly(ctx context.Context) int {
	return executeCommand(ctx, func(ctx context.Context) error {
		snapshot, err := observeInfo(ctx, config.LoadConfig(), getVersion())
		if err != nil {
			return err
		}
		return legacyPresenter().InfoSnapshot(snapshot)
	})
}

// observeInfo reads model metadata without initializing a missing model root.
func observeInfo(ctx context.Context, cfg *config.Config, version string) (cli.InfoSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return cli.InfoSnapshot{}, err
	}
	snapshot := cli.InfoSnapshot{
		Version: version, ServerPort: cfg.ServerPort, ModelsDir: cfg.ModelsDir,
		NumThreads: cfg.NumThreads, MaxContextSize: cfg.MaxContextSize,
		MaxMemoryMB: cfg.MaxMemoryMB, EnableP2P: cfg.EnableP2P, P2PPort: cfg.P2PPort,
		OS: runtime.GOOS, Arch: runtime.GOARCH, CPUCores: runtime.NumCPU(),
	}
	if data, err := os.ReadFile("/proc/meminfo"); err == nil {
		snapshot.Memory = cli.ReadProcMemory(data)
	}
	registry := models.NewRegistry(cfg.ModelsDir)
	if err := registry.ScanModelsReadOnly(); err != nil && !os.IsNotExist(err) {
		snapshot.ModelScanError = err
	}
	for _, model := range registry.ListModels() {
		meta, err := registry.GetModel(model.ID)
		if err != nil {
			continue
		}
		snapshot.Models = append(snapshot.Models, cli.InfoModel{ID: model.ID, Size: meta.Size, Quantization: meta.Quantization, Path: meta.Path, IsLoaded: meta.IsLoaded})
	}
	catalog := models.DefaultCatalog()
	snapshot.CatalogCount = len(catalog.Models)
	for _, entry := range catalog.Models {
		if entry.Recommended {
			snapshot.Recommended++
		}
	}
	return snapshot, ctx.Err()
}
