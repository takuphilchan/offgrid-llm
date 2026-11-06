package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/output"
)

type InfoModel struct {
	ID           string
	Size         int64
	Quantization string
	Path         string
	IsLoaded     bool
}

type InfoSnapshot struct {
	Version        string
	ServerPort     int
	ModelsDir      string
	NumThreads     int
	MaxContextSize int
	MaxMemoryMB    uint64
	EnableP2P      bool
	P2PPort        int
	Models         []InfoModel
	ModelScanError error
	OS             string
	Arch           string
	CPUCores       int
	Memory         string
	CatalogCount   int
	Recommended    int
}

func infoCommand(ctx context.Context, _ []string, env Environment) error {
	if env.Info == nil {
		return errors.New("missing info observer")
	}
	snapshot, err := env.Info(ctx)
	if err != nil {
		return err
	}
	p := Presenter{Stdout: env.Stdout, Stderr: env.Stderr, JSON: env.Options.JSON, Style: env.Style}
	return p.InfoSnapshot(snapshot)
}

func (p Presenter) InfoSnapshot(snapshot InfoSnapshot) error {
	if snapshot.ModelScanError != nil && !p.JSON {
		_, _ = fmt.Fprintf(p.Stderr, "  Model scan failed: %s\n", terminalLine(snapshot.ModelScanError.Error()))
	}
	if p.JSON {
		var models []output.ModelInfo
		for _, model := range snapshot.Models {
			item := output.ModelInfo{Name: model.ID}
			if model.Size > 0 {
				item.Size = FormatBytes(model.Size)
			}
			if model.Quantization != "" && model.Quantization != "unknown" {
				item.Quantization = model.Quantization
			}
			if model.Path != "" {
				item.Path = model.Path
			}
			models = append(models, item)
		}
		encoder := json.NewEncoder(p.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(map[string]any{
			"version": snapshot.Version,
			"config": map[string]any{
				"port": snapshot.ServerPort, "models_dir": snapshot.ModelsDir,
				"max_context": snapshot.MaxContextSize, "threads": snapshot.NumThreads,
				"max_memory": snapshot.MaxMemoryMB, "p2p_enabled": snapshot.EnableP2P,
			},
			"models": map[string]any{"installed": models, "count": len(models)},
			"system": output.SystemInfo{CPU: fmt.Sprintf("%d cores", snapshot.CPUCores), Memory: snapshot.Memory, OS: snapshot.OS, Architecture: snapshot.Arch},
		})
	}
	p.Heading("OffGrid / Local configuration")
	p.Detail("Version", snapshot.Version)
	p.Note("Local settings and files; live service allocations may differ.")
	p.Heading("Configuration")
	p.Detail("Port", fmt.Sprint(snapshot.ServerPort))
	p.Detail("Model folder", snapshot.ModelsDir)
	threads := fmt.Sprint(snapshot.NumThreads)
	if snapshot.NumThreads == 0 {
		threads = "Automatic"
	}
	p.Detail("Threads", threads)
	p.Detail("Context limit", fmt.Sprintf("%s tokens (configured)", formatCount(snapshot.MaxContextSize)))
	p.Detail("Memory budget", fmt.Sprintf("%s MB (configured)", formatCount(int(snapshot.MaxMemoryMB))))
	if snapshot.EnableP2P {
		p.Detail("P2P", fmt.Sprintf("Enabled on port %d", snapshot.P2PPort))
	}
	p.Heading(fmt.Sprintf("Local model files (%d)", len(snapshot.Models)))
	models := append([]InfoModel(nil), snapshot.Models...)
	sort.SliceStable(models, func(i, j int) bool { return models[i].ID < models[j].ID })
	rows := make([]presentationRow, 0, len(models))
	var totalSize int64
	for _, model := range models {
		detail := "Size unavailable"
		if model.Size > 0 {
			detail = FormatBytes(model.Size)
			totalSize += model.Size
		}
		// Registry quantization is a filename heuristic, not verified GGUF
		// metadata. Keep the exact identity and size here; JSON remains compatible.
		if model.IsLoaded {
			detail += " / loaded"
		}
		rows = append(rows, presentationRow{model.ID, detail})
	}
	p.rows(rows)
	if len(models) == 0 && snapshot.ModelScanError == nil {
		p.Note("No model files found in this folder.")
	}
	if totalSize > 0 {
		p.Note("Total on disk: " + FormatBytes(totalSize))
	}
	p.Heading("Next steps")
	p.rows([]presentationRow{
		{"offgrid list", "Models available from the active service"},
		{"offgrid doctor", "Check service and runtime readiness"},
		{"offgrid agent", "Start an agent conversation"},
	})
	fmt.Fprintln(p.Stdout)
	return nil
}

// ReadProcMemory extracts the existing Linux memory display from injected
// /proc/meminfo bytes. Other platforms keep the previous empty value.
func ReadProcMemory(data []byte) string {
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				return fields[1] + " kB"
			}
		}
	}
	return ""
}
