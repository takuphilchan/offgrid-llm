package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/output"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type modelChoice struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Repo        string `json:"repo"`
	File        string `json:"file"`
	Quant       string `json:"quant"`
	Recommended bool   `json:"recommended,omitempty"`
}

func listCommand(ctx context.Context, args []string, env Environment) error {
	serverURL := strings.TrimSpace(env.Getenv("OFFGRID_SERVER_URL"))
	if serverURL == "" {
		serverURL = env.ServerURL
	}
	p := Presenter{Stdout: env.Stdout, Stderr: env.Stderr, JSON: env.Options.JSON, Style: env.Style}
	return RunList(ctx, args, p, serverURL, env.Getenv("OFFGRID_API_KEY"), env.LongHTTP)
}

// RunList keeps authenticated service access outside the model filesystem.
// The caller owns the service origin, credential lookup, and transport.
func RunList(ctx context.Context, args []string, p Presenter, serverURL, apiKey string, httpClient *http.Client) error {
	catalog := false
	for _, arg := range args {
		switch arg {
		case "--help", "-h", "help":
			return p.modelListHelp()
		case "--catalog":
			catalog = true
		default:
			return Usage("unknown list option: %s", arg)
		}
	}
	if httpClient == nil {
		return errors.New("missing model-list HTTP client")
	}
	client, err := serviceclient.New(serverURL, apiKey, httpClient)
	if err != nil {
		return err
	}
	if catalog {
		var result struct {
			Models []modelChoice `json:"models"`
		}
		if err := client.JSON(ctx, "GET", "/v1/catalog", nil, &result); err != nil {
			return err
		}
		if result.Models == nil {
			result.Models = []modelChoice{}
		}
		if p.JSON {
			return json.NewEncoder(p.Stdout).Encode(result)
		}
		p.Heading(fmt.Sprintf("OffGrid / Model catalog (%d)", len(result.Models)))
		p.Detail("Service", serverURL)
		p.Note("Catalog entries are not necessarily installed.")
		sort.SliceStable(result.Models, func(i, j int) bool {
			if result.Models[i].Recommended != result.Models[j].Recommended {
				return result.Models[i].Recommended
			}
			return false // Preserve the service's curated order within each group.
		})
		rows := make([]presentationRow, 0, len(result.Models))
		for _, item := range result.Models {
			detail := item.Quant
			if item.Recommended {
				detail += " · Recommended"
			}
			rows = append(rows, presentationRow{item.ID, detail})
		}
		p.rows(rows)
		if len(rows) == 0 {
			p.Note("No catalog entries available from this service.")
		}
		fmt.Fprintln(p.Stdout)
		return nil
	}
	var result api.ModelListResponse
	if err := client.JSON(ctx, "GET", "/v1/models", nil, &result); err != nil {
		return err
	}
	items := make([]output.ModelInfo, 0, len(result.Data))
	for _, model := range result.Data {
		items = append(items, output.ModelInfo{Name: model.ID, Size: FormatBytes(model.Size), Format: "gguf"})
	}
	if p.JSON {
		return json.NewEncoder(p.Stdout).Encode(map[string]any{"server": serverURL, "models": items, "count": len(items)})
	}
	p.Heading(fmt.Sprintf("OffGrid / Installed models (%d)", len(items)))
	p.Detail("Service", serverURL)
	fmt.Fprintln(p.Stdout)
	sort.SliceStable(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	rows := make([]presentationRow, 0, len(items))
	for _, item := range items {
		rows = append(rows, presentationRow{item.Name, item.Size})
	}
	p.rows(rows)
	if len(items) == 0 {
		p.Note("No models installed on this service. Use offgrid list --catalog or offgrid search <query>.")
	} else {
		p.Heading("Start a conversation")
		p.Note("offgrid agent")
		p.Note("Or use an exact model ID above: offgrid run MODEL")
	}
	fmt.Fprintln(p.Stdout)
	return nil
}

func (p Presenter) modelListHelp() error {
	help := "offgrid list [--catalog] [--json]"
	if p.JSON {
		return json.NewEncoder(p.Stdout).Encode(map[string]string{"usage": help, "server": "OFFGRID_SERVER_URL", "authentication": "OFFGRID_API_KEY"})
	}
	_, _ = fmt.Fprintln(p.Stdout, help)
	_, _ = fmt.Fprintln(p.Stdout, "Uses the active service via OFFGRID_SERVER_URL and OFFGRID_API_KEY. No direct-file fallback.")
	return nil
}

func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
