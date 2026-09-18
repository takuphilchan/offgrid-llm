package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/internal/output"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type modelChoice struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Repo  string `json:"repo"`
	File  string `json:"file"`
	Quant string `json:"quant"`
}

func modelHelp(command string) error {
	help := "offgrid list [--catalog] [--json]"
	if command == "download" {
		help = "offgrid download <catalog-id|owner/repo> [--file name.gguf] [--quant Q4_K_M] [--yes] [--detach] [--json]"
	}
	if output.JSONMode {
		return json.NewEncoder(os.Stdout).Encode(map[string]string{"usage": help, "server": "OFFGRID_SERVER_URL", "authentication": "OFFGRID_API_KEY"})
	}
	fmt.Println(help)
	fmt.Println("Uses the active service via OFFGRID_SERVER_URL and OFFGRID_API_KEY. No direct-file fallback.")
	return nil
}

func runModelList(ctx context.Context, args []string) error {
	catalog := false
	for _, arg := range args {
		switch arg {
		case "--help", "-h", "help":
			return modelHelp("list")
		case "--catalog":
			catalog = true
		default:
			return usage("unknown list option: %s", arg)
		}
	}
	client, err := newCommandClient()
	if err != nil {
		return err
	}
	if catalog {
		var result struct {
			Models []modelChoice `json:"models"`
		}
		if err = client.JSON(ctx, "GET", "/v1/catalog", nil, &result); err != nil {
			return err
		}
		if result.Models == nil {
			result.Models = []modelChoice{}
		}
		if output.JSONMode {
			return json.NewEncoder(os.Stdout).Encode(result)
		}
		fmt.Printf("Catalog · %s\n", terminalSafe(commandServerURL()))
		for _, item := range result.Models {
			fmt.Printf("  %-36s %s\n", terminalSafe(item.ID), terminalSafe(item.Quant))
		}
		return nil
	}
	var result api.ModelListResponse
	if err = client.JSON(ctx, "GET", "/v1/models", nil, &result); err != nil {
		return err
	}
	items := make([]output.ModelInfo, 0, len(result.Data))
	for _, model := range result.Data {
		items = append(items, output.ModelInfo{Name: model.ID, Size: formatBytes(model.Size), Format: "gguf"})
	}
	if output.JSONMode {
		return json.NewEncoder(os.Stdout).Encode(map[string]any{"server": commandServerURL(), "models": items, "count": len(items)})
	}
	fmt.Printf("Installed models · %s\n", terminalSafe(commandServerURL()))
	for _, item := range items {
		fmt.Printf("  %-42s %s\n", terminalSafe(item.Name), item.Size)
	}
	if len(items) == 0 {
		fmt.Println("  No models installed on this service. Use offgrid list --catalog or offgrid search <query>.")
	}
	return nil
}

type downloadOptions struct {
	model, file, quant string
	detach             bool
}

func parseDownloadOptions(args []string) (downloadOptions, error) {
	var result downloadOptions
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--yes", "-y":
		case "--detach":
			result.detach = true
		case "--file", "--quant":
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
				return result, usage("%s requires a value", arg)
			}
			i++
			if arg == "--file" {
				result.file = args[i]
			} else {
				result.quant = args[i]
			}
		default:
			if strings.HasPrefix(arg, "-") {
				return result, usage("unknown download option: %s", arg)
			}
			if result.model != "" {
				return result, usage("only one model can be downloaded per command")
			}
			result.model = arg
		}
	}
	if result.model == "" {
		return result, usage("model is required; use offgrid download --help")
	}
	return result, nil
}

func runModelDownload(ctx context.Context, args []string) error {
	for _, arg := range args {
		if arg == "--help" || arg == "-h" || arg == "help" {
			return modelHelp("download")
		}
	}
	opts, err := parseDownloadOptions(args)
	if err != nil {
		return err
	}
	client, err := newCommandClient()
	if err != nil {
		return err
	}
	var choice modelChoice
	if strings.Contains(opts.model, "/") {
		if !models.ValidHubRepository(opts.model) {
			return usage("expected a Hugging Face owner/repository")
		}
		var result struct {
			Files []models.DiscoveredFile `json:"files"`
		}
		if err = client.JSON(ctx, "GET", "/v1/search/files?repo="+url.QueryEscape(opts.model), nil, &result); err != nil {
			return err
		}
		candidates := []models.DiscoveredFile{}
		for _, file := range result.Files {
			if file.Supported && (opts.file == "" || file.File == opts.file) && (opts.quant == "" || strings.EqualFold(file.Quant, opts.quant)) {
				candidates = append(candidates, file)
			}
		}
		if len(candidates) == 0 {
			return usage("no supported file matches; inspect the repository with offgrid search --files")
		}
		if len(candidates) > 1 && opts.file == "" {
			preferred := []models.DiscoveredFile{}
			for _, f := range candidates {
				if f.Quant == "Q4_K_M" {
					preferred = append(preferred, f)
				}
			}
			if len(preferred) == 1 {
				candidates = preferred
			}
		}
		if len(candidates) != 1 {
			return usage("multiple files match; select one explicitly with --file")
		}
		file := candidates[0]
		choice = modelChoice{ID: file.ID, Repo: opts.model, File: file.File, Quant: file.Quant}
	} else {
		var result struct {
			Models []modelChoice `json:"models"`
		}
		if err = client.JSON(ctx, "GET", "/v1/catalog?quant="+url.QueryEscape(opts.quant), nil, &result); err != nil {
			return err
		}
		for _, item := range result.Models {
			if strings.EqualFold(item.ID, opts.model) {
				choice = item
				break
			}
		}
		if choice.ID == "" {
			return usage("model/quantization is not in this service's catalog; use offgrid list --catalog")
		}
		if opts.file != "" && opts.file != choice.File {
			return usage("use owner/repository with --file to select a non-catalog file")
		}
	}
	var accepted struct {
		Success  bool   `json:"success"`
		Exists   bool   `json:"exists"`
		Status   string `json:"status"`
		FileName string `json:"file_name"`
	}
	if err = client.JSON(ctx, "POST", "/v1/models/download", map[string]string{"model_id": choice.ID, "repository": choice.Repo, "file_name": choice.File, "quantization": choice.Quant}, &accepted); err != nil {
		return err
	}
	if !accepted.Success || accepted.FileName == "" {
		return fmt.Errorf("service did not accept the download")
	}
	finish := func(status string) error {
		if output.JSONMode {
			return json.NewEncoder(os.Stdout).Encode(map[string]any{"success": true, "status": status, "file_name": accepted.FileName, "server": commandServerURL()})
		}
		fmt.Printf("%s · %s\n", status, terminalSafe(accepted.FileName))
		return nil
	}
	if accepted.Exists {
		return finish("complete")
	}
	if opts.detach {
		return finish("accepted")
	}
	fmt.Fprintf(os.Stderr, "Downloading %s on %s. Ctrl+C stops the download and keeps partial data.\n", terminalSafe(accepted.FileName), terminalSafe(commandServerURL()))
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	previous := ""
	for {
		var progress map[string]struct {
			Status  string  `json:"status"`
			Percent float64 `json:"percent"`
			Error   string  `json:"error"`
		}
		if err = client.JSON(ctx, "GET", "/v1/models/download/progress", nil, &progress); err != nil {
			if ctx.Err() == nil {
				return err
			}
		} else {
			item, ok := progress[accepted.FileName]
			if !ok {
				return fmt.Errorf("download is absent from service state; inspect Models before retrying")
			}
			switch item.Status {
			case "complete":
				return finish("complete")
			case "failed":
				return fmt.Errorf("download failed: %s", terminalSafe(item.Error))
			case "cancelled":
				return context.Canceled
			}
			line := fmt.Sprintf("%s · %.0f%%", item.Status, item.Percent)
			if line != previous && !output.JSONMode {
				fmt.Fprintln(os.Stderr, line)
				previous = line
			}
		}
		select {
		case <-ctx.Done():
			stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			stopErr := client.JSON(stopCtx, http.MethodPost, "/v1/models/download/cancel", map[string]string{"file_name": accepted.FileName}, nil)
			cancel()
			if stopErr != nil {
				fmt.Fprintln(os.Stderr, "Could not confirm download cancellation. Inspect the service's Models page.")
			}
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
