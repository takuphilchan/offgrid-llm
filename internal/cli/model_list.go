package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/output"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type modelChoice struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Repo  string `json:"repo"`
	File  string `json:"file"`
	Quant string `json:"quant"`
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
		_, _ = fmt.Fprintf(p.Stdout, "Catalog · %s\n", TerminalSafe(serverURL))
		for _, item := range result.Models {
			_, _ = fmt.Fprintf(p.Stdout, "  %-36s %s\n", TerminalSafe(item.ID), TerminalSafe(item.Quant))
		}
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
	_, _ = fmt.Fprintf(p.Stdout, "Installed models · %s\n", TerminalSafe(serverURL))
	for _, item := range items {
		_, _ = fmt.Fprintf(p.Stdout, "  %-42s %s\n", TerminalSafe(item.Name), item.Size)
	}
	if len(items) == 0 {
		_, _ = fmt.Fprintln(p.Stdout, "  No models installed on this service. Use offgrid list --catalog or offgrid search <query>.")
	}
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
