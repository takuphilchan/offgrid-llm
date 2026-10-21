package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
		_, _ = fmt.Fprintf(p.Stderr, "  ✗ Model scan error: %v\n", snapshot.ModelScanError)
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
	s := p.Style
	_, _ = fmt.Fprintln(p.Stdout)
	_, _ = fmt.Fprintf(p.Stdout, "  %s◈ OffGrid LLM%s %sv%s%s\n\n", s.Primary+s.Bold, s.Reset, s.Muted, snapshot.Version, s.Reset)
	_, _ = fmt.Fprintf(p.Stdout, "  %sConfiguration%s\n", s.Primary, s.Reset)
	_, _ = fmt.Fprintf(p.Stdout, "    Port       %s%d%s\n", s.Dim, snapshot.ServerPort, s.Reset)
	_, _ = fmt.Fprintf(p.Stdout, "    Models     %s%s%s\n", s.Dim, snapshot.ModelsDir, s.Reset)
	_, _ = fmt.Fprintf(p.Stdout, "    Threads    %s%d%s\n", s.Dim, snapshot.NumThreads, s.Reset)
	_, _ = fmt.Fprintf(p.Stdout, "    Context    %s%d tokens%s\n", s.Dim, snapshot.MaxContextSize, s.Reset)
	_, _ = fmt.Fprintf(p.Stdout, "    Memory     %s%d MB%s\n", s.Dim, snapshot.MaxMemoryMB, s.Reset)
	if snapshot.EnableP2P {
		_, _ = fmt.Fprintf(p.Stdout, "    P2P        %senabled (port %d)%s\n", s.Dim, snapshot.P2PPort, s.Reset)
	}
	_, _ = fmt.Fprintln(p.Stdout)
	count := len(snapshot.Models)
	if count == 1 {
		_, _ = fmt.Fprintf(p.Stdout, "  %sInstalled Models%s %s· 1 model%s\n", s.Primary, s.Reset, s.Muted, s.Reset)
	} else {
		_, _ = fmt.Fprintf(p.Stdout, "  %sInstalled Models%s %s· %d models%s\n", s.Primary, s.Reset, s.Muted, count, s.Reset)
	}
	var totalSize int64
	if count > 0 {
		for _, model := range snapshot.Models {
			icon, color := "○", s.Dim
			if model.IsLoaded {
				icon, color = "●", s.Success
			}
			_, _ = fmt.Fprintf(p.Stdout, "    %s%s%s %s", color, icon, s.Reset, model.ID)
			if model.Size > 0 {
				_, _ = fmt.Fprintf(p.Stdout, " %s· %s%s", s.Dim, FormatBytes(model.Size), s.Reset)
				totalSize += model.Size
			}
			if model.Quantization != "" && model.Quantization != "unknown" {
				_, _ = fmt.Fprintf(p.Stdout, " %s· %s%s", s.Dim, model.Quantization, s.Reset)
			}
			_, _ = fmt.Fprintln(p.Stdout)
		}
		if totalSize > 0 {
			_, _ = fmt.Fprintf(p.Stdout, "    %sTotal: %s%s\n", s.Muted, FormatBytes(totalSize), s.Reset)
		}
	} else {
		_, _ = fmt.Fprintf(p.Stdout, "    %sNo models installed%s\n", s.Muted, s.Reset)
	}
	_, _ = fmt.Fprintln(p.Stdout)
	_, _ = fmt.Fprintf(p.Stdout, "  %sCatalog%s %s· %d models (%d recommended)%s\n\n", s.Primary, s.Reset, s.Muted, snapshot.CatalogCount, snapshot.Recommended, s.Reset)
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
