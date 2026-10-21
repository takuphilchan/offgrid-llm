package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/internal/output"
)

func searchCommand(ctx context.Context, args []string, env Environment) error {
	p := Presenter{Stdout: env.Stdout, Stderr: env.Stderr, JSON: env.Options.JSON, Style: env.Style}
	return RunSearch(ctx, args, p, env.HTTP)
}

// RunSearch uses only the supplied transport and streams. The transitional
// process adapter and the direct CLI runner share this implementation.
func RunSearch(ctx context.Context, args []string, p Presenter, httpClient *http.Client) error {
	filters, maxRAM, showFiles, err := ParseModelSearchArgs(args)
	if err != nil {
		return err
	}
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			p.searchHelp()
			return nil
		}
	}
	if httpClient == nil {
		return errors.New("missing search HTTP client")
	}
	if !p.JSON {
		_, _ = fmt.Fprintln(p.Stderr, "Searching Hugging Face Hub…")
	}
	results, err := models.NewHuggingFaceClient().WithHTTPClient(httpClient).SearchModelsContext(ctx, filters)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}
	if maxRAM > 0 {
		filtered := make([]models.SearchResult, 0, len(results))
		for _, result := range results {
			var estimatedRAM float64
			if result.BestVariant != nil && result.BestVariant.SizeGB > 0 {
				estimatedRAM = result.BestVariant.SizeGB * 1.3
			} else if result.BestVariant != nil {
				estimatedRAM = EstimateRAMFromModel(result.Model.ID, result.BestVariant.Quantization)
			}
			if estimatedRAM > 0 && estimatedRAM <= float64(maxRAM) {
				filtered = append(filtered, result)
			}
		}
		results = filtered
		if !p.JSON && len(filtered) > 0 {
			_, _ = fmt.Fprintf(p.Stdout, "%s[Filtered for ≤%dGB RAM]%s\n\n", p.Style.Muted, maxRAM, p.Style.Reset)
		}
	}
	if p.JSON {
		jsonResults := make([]output.SearchResult, 0, len(results))
		for _, result := range results {
			item := output.SearchResult{Name: result.Model.ID, ModelID: result.Model.ID, Downloads: int(result.Model.Downloads), Likes: result.Model.Likes}
			if len(result.Model.Tags) > 0 {
				item.Tags = result.Model.Tags
			}
			for _, file := range result.GGUFFiles {
				item.Files = append(item.Files, output.SearchFile{File: file.Filename, Size: file.Size, Quant: file.Quantization})
			}
			jsonResults = append(jsonResults, item)
		}
		encoder := json.NewEncoder(p.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(map[string]any{"results": jsonResults, "count": len(jsonResults)})
	}
	if len(results) == 0 {
		_, _ = fmt.Fprintln(p.Stdout)
		p.Warning("No models found matching your criteria")
		_, _ = fmt.Fprintln(p.Stdout)
		_, _ = fmt.Fprintf(p.Stdout, "    %sTry:%s offgrid search llama --ram 4\n\n", p.Style.Muted, p.Style.Reset)
		return nil
	}
	_, _ = fmt.Fprintf(p.Stdout, "    %s%d model(s) found%s\n\n", p.Style.Muted, len(results), p.Style.Reset)
	for i, result := range results {
		model := result.Model
		_, _ = fmt.Fprintf(p.Stdout, "  %s%d.%s %s%s%s", p.Style.Muted, i+1, p.Style.Reset, p.Style.Bold, model.ID, p.Style.Reset)
		if result.IsRecommended {
			_, _ = fmt.Fprintf(p.Stdout, " %s★%s", p.Style.Success, p.Style.Reset)
		}
		if result.IsTrusted {
			_, _ = fmt.Fprintf(p.Stdout, " %s✓%s", p.Style.Primary, p.Style.Reset)
		}
		_, _ = fmt.Fprintln(p.Stdout)
		infoParts := []string{fmt.Sprintf("↓ %s", compactNumber(model.Downloads)), fmt.Sprintf("♥ %s", compactNumber(int64(model.Likes)))}
		if result.BestVariant != nil {
			infoParts = append(infoParts, result.BestVariant.Quantization)
			if result.BestVariant.SizeGB > 0 {
				infoParts = append(infoParts, fmt.Sprintf("%.1fGB", result.BestVariant.SizeGB), fmt.Sprintf("~%.0fGB RAM", result.BestVariant.SizeGB*1.3))
			} else if estimatedRAM := EstimateRAMFromModel(model.ID, result.BestVariant.Quantization); estimatedRAM > 0 {
				infoParts = append(infoParts, fmt.Sprintf("~%.0fGB RAM", estimatedRAM))
			}
		}
		_, _ = fmt.Fprintf(p.Stdout, "     %s%s%s\n", p.Style.Dim, strings.Join(infoParts, " · "), p.Style.Reset)
		if showFiles {
			for _, file := range result.GGUFFiles {
				_, _ = fmt.Fprintf(p.Stdout, "     %s · %.2f GB · %s\n", TerminalSafe(file.Filename), file.SizeGB, file.Quantization)
				_, _ = fmt.Fprintf(p.Stdout, "       offgrid download %q --file %q\n", TerminalSafe(model.ID), TerminalSafe(file.Filename))
			}
		}
		if i < len(results)-1 {
			_, _ = fmt.Fprintln(p.Stdout)
		}
	}
	_, _ = fmt.Fprintln(p.Stdout)
	_, _ = fmt.Fprintf(p.Stdout, "  %sTip: add --files to choose a quantization; use offgrid download owner/repo --file model.gguf%s\n\n", p.Style.Muted, p.Style.Reset)
	return nil
}

func (p Presenter) searchHelp() {
	s := p.Style
	_, _ = fmt.Fprintln(p.Stdout)
	_, _ = fmt.Fprintf(p.Stdout, "  %s%s Search HuggingFace%s\n\n", s.Primary+s.Bold, s.Search, s.Reset)
	_, _ = fmt.Fprintf(p.Stdout, "  %sUsage%s  offgrid search %s[query]%s %s[options]%s\n\n", s.Bold, s.Reset, s.Primary, s.Reset, s.Muted, s.Reset)
	_, _ = fmt.Fprintf(p.Stdout, "  %sOptions%s\n", s.Bold, s.Reset)
	_, _ = fmt.Fprintf(p.Stdout, "    %s-a, --author%s <name>    Filter by author (e.g., 'TheBloke')\n", s.Primary, s.Reset)
	_, _ = fmt.Fprintf(p.Stdout, "    %s-q, --quant%s <type>     Filter by quantization (e.g., 'Q4_K_M')\n", s.Primary, s.Reset)
	_, _ = fmt.Fprintf(p.Stdout, "    %s-r, --ram%s <GB>         Filter by max RAM (e.g., 4, 8, 16)\n", s.Primary, s.Reset)
	_, _ = fmt.Fprintf(p.Stdout, "    %s-s, --sort%s <field>     Sort by: downloads, likes, created\n", s.Primary, s.Reset)
	_, _ = fmt.Fprintf(p.Stdout, "    %s-l, --limit%s <n>        Limit results (default: 20)\n", s.Primary, s.Reset)
	_, _ = fmt.Fprintf(p.Stdout, "    %s--all%s                  Include gated models\n", s.Primary, s.Reset)
	_, _ = fmt.Fprintln(p.Stdout, "    --files                Show individual GGUF files and download commands")
	_, _ = fmt.Fprintln(p.Stdout, "    No RAM limit is applied unless --ram is supplied; estimates are not a compatibility guarantee.")
	_, _ = fmt.Fprintln(p.Stdout)
	_, _ = fmt.Fprintf(p.Stdout, "  %sExamples%s\n", s.Bold, s.Reset)
	_, _ = fmt.Fprintf(p.Stdout, "    %s$%s offgrid search llama\n", s.Muted, s.Reset)
	_, _ = fmt.Fprintf(p.Stdout, "    %s$%s offgrid search llama --ram 4\n", s.Muted, s.Reset)
	_, _ = fmt.Fprintf(p.Stdout, "    %s$%s offgrid search mistral --author TheBloke --quant Q4_K_M\n\n", s.Muted, s.Reset)
}

func compactNumber(n int64) string {
	if n >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(n)/1000000)
	}
	if n >= 1000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

func EstimateRAMFromModel(modelID, quant string) float64 {
	modelLower := strings.ToLower(modelID)
	var paramSize float64
	switch {
	case strings.Contains(modelLower, "1b") || strings.Contains(modelLower, "1.1b"):
		paramSize = 1
	case strings.Contains(modelLower, "3b") || strings.Contains(modelLower, "3.2b"):
		paramSize = 3
	case strings.Contains(modelLower, "7b"):
		paramSize = 7
	case strings.Contains(modelLower, "8b"):
		paramSize = 8
	case strings.Contains(modelLower, "13b"):
		paramSize = 13
	case strings.Contains(modelLower, "70b"):
		paramSize = 70
	default:
		return 0
	}
	var bytesPerParam float64
	quantUpper := strings.ToUpper(quant)
	switch {
	case strings.Contains(quantUpper, "Q2"):
		bytesPerParam = 0.3
	case strings.Contains(quantUpper, "Q3"):
		bytesPerParam = 0.4
	case strings.Contains(quantUpper, "Q4"):
		bytesPerParam = 0.5
	case strings.Contains(quantUpper, "Q5"):
		bytesPerParam = 0.65
	case strings.Contains(quantUpper, "Q6"):
		bytesPerParam = 0.75
	case strings.Contains(quantUpper, "Q8"):
		bytesPerParam = 1.0
	default:
		bytesPerParam = 0.5
	}
	return paramSize * bytesPerParam * 1.3
}
