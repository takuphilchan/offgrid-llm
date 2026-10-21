package main

import (
	"context"
	"fmt"
	"github.com/takuphilchan/offgrid-llm/internal/completions"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"os"
	"path/filepath"
	"strings"
)

func handleDownloadHF(ctx context.Context, args []string) int {
	// Redirect to unified download command (backward compatibility)
	// download-hf is now just an alias for download with HuggingFace models
	return handleDownload(ctx, args)
}

func downloadVisionProjector(hf *models.HuggingFaceClient, modelsDir string, projector *models.ProjectorSource) error {
	if projector == nil || projector.File.Filename == "" {
		return nil
	}

	remotePath := projector.File.Filename
	localName := filepath.Base(remotePath)
	destPath := filepath.Join(modelsDir, localName)
	if _, err := os.Stat(destPath); err == nil {
		fmt.Printf("  ✓ Vision adapter already present at %s%s%s\n", brandMuted, destPath, colorReset)
		return nil
	}

	sourceLabel := projector.ModelID
	if projector.Source == "fallback" {
		sourceLabel = fmt.Sprintf("%s (fallback)", sourceLabel)
	}
	fmt.Printf("  Source: %s%s%s\n", brandMuted, sourceLabel, colorReset)
	if projector.Reason != "" {
		fmt.Printf("  %s%s%s\n", brandMuted, projector.Reason, colorReset)
	}
	fmt.Printf("  ⧉ Downloading %s%s%s\n", brandPrimary, remotePath, colorReset)

	err := hf.DownloadGGUF(projector.ModelID, remotePath, destPath, func(current, total int64) {
		percent := 0.0
		if total > 0 {
			percent = float64(current) / float64(total) * 100
		}
		fmt.Printf("\r      %.1f%% (%s / %s)",
			percent,
			formatBytes(current),
			formatBytes(total))
	})

	fmt.Println()
	if err != nil {
		return err
	}

	fmt.Printf("  ✓ Vision adapter saved to %s%s%s\n", brandMuted, destPath, colorReset)
	return nil
}

func extractQuantFromFilename(filename string) string {
	patterns := []string{
		"Q2_K", "Q3_K_S", "Q3_K_M", "Q3_K_L",
		"Q4_0", "Q4_1", "Q4_K_S", "Q4_K_M",
		"Q5_0", "Q5_1", "Q5_K_S", "Q5_K_M",
		"Q6_K", "Q8_0", "F16", "F32",
		"IQ1_S", "IQ1_M", "IQ2_XXS", "IQ2_XS", "IQ2_S", "IQ2_M",
		"IQ3_XXS", "IQ3_XS", "IQ3_S", "IQ3_M",
		"IQ4_XS", "IQ4_NL",
	}

	upper := strings.ToUpper(filename)
	for _, pattern := range patterns {
		if strings.Contains(upper, pattern) {
			return pattern
		}
	}

	// Handle simplified quant names (e.g., Microsoft's "q4" naming)
	// Match patterns like "-q4", "_q4", "-q4.", "_q4." etc.
	lower := strings.ToLower(filename)

	// Remove .gguf extension for easier matching
	base := strings.TrimSuffix(lower, ".gguf")

	// Check for simplified patterns at end of filename or before other extensions
	simplifiedPatterns := []struct {
		suffix string
		quant  string
	}{
		// Check longer patterns first (more specific)
		{"-q4_0", "Q4_0"}, {"_q4_0", "Q4_0"}, {".q4_0", "Q4_0"},
		{"-q4_1", "Q4_1"}, {"_q4_1", "Q4_1"}, {".q4_1", "Q4_1"},
		{"-q5_0", "Q5_0"}, {"_q5_0", "Q5_0"}, {".q5_0", "Q5_0"},
		{"-q5_1", "Q5_1"}, {"_q5_1", "Q5_1"}, {".q5_1", "Q5_1"},
		{"-q8_0", "Q8_0"}, {"_q8_0", "Q8_0"}, {".q8_0", "Q8_0"},
		// Then simpler patterns
		{"-q2", "Q2_K"}, {"_q2", "Q2_K"}, {".q2", "Q2_K"},
		{"-q3", "Q3_K_M"}, {"_q3", "Q3_K_M"}, {".q3", "Q3_K_M"},
		{"-q4", "Q4_K_M"}, {"_q4", "Q4_K_M"}, {".q4", "Q4_K_M"},
		{"-q5", "Q5_K_M"}, {"_q5", "Q5_K_M"}, {".q5", "Q5_K_M"},
		{"-q6", "Q6_K"}, {"_q6", "Q6_K"}, {".q6", "Q6_K"},
		{"-q8", "Q8_0"}, {"_q8", "Q8_0"}, {".q8", "Q8_0"},
	}
	for _, p := range simplifiedPatterns {
		// Check if base ends with this pattern
		if strings.HasSuffix(base, p.suffix) {
			return p.quant
		}
		// Also check if it contains the pattern followed by a delimiter
		if strings.Contains(base, p.suffix+"-") || strings.Contains(base, p.suffix+"_") || strings.Contains(base, p.suffix+".") {
			return p.quant
		}
	}

	return "unknown"
}

func handleCompletions(args []string) {
	// Only show banner/help when no args provided (help mode)
	// Don't show when generating actual completion scripts
	if len(args) == 0 {
		fmt.Println()
		fmt.Printf("  %s◈ Shell Completions%s\n", brandPrimary+colorBold, colorReset)
		fmt.Printf("  %sTab completion for your shell%s\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sUsage%s  offgrid completions <shell>\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sSupported%s  bash, zsh, fish\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sExamples%s\n", brandPrimary, colorReset)
		fmt.Printf("    %s$%s offgrid completions bash >> ~/.bashrc\n", colorDim, colorReset)
		fmt.Printf("    %s$%s offgrid completions zsh > ~/.zsh/completions/_offgrid\n", colorDim, colorReset)
		fmt.Printf("    %s$%s offgrid completions fish > ~/.config/fish/completions/offgrid.fish\n", colorDim, colorReset)
		fmt.Println()
		return
	}

	shell := args[0]
	gen := completions.NewGenerator("offgrid")

	// Add all commands with their subcommands and flags
	gen.AddCommand(completions.Command{
		Name:        "list",
		Flags:       []string{"--json"},
		Description: "List available models",
	})
	gen.AddCommand(completions.Command{
		Name:        "search",
		Flags:       []string{"--author", "--limit", "--json"},
		Description: "Search for models",
	})
	gen.AddCommand(completions.Command{
		Name:        "download",
		Description: "Download a model",
	})
	gen.AddCommand(completions.Command{
		Name:        "download-hf",
		Flags:       []string{"--quant"},
		Description: "Download from HuggingFace",
	})
	gen.AddCommand(completions.Command{
		Name:        "run",
		Flags:       []string{"--save", "--load"},
		Description: "Start interactive chat",
	})
	gen.AddCommand(completions.Command{
		Name:        "import",
		Description: "Import models from USB",
	})
	gen.AddCommand(completions.Command{
		Name:        "export",
		Description: "Export model to USB",
	})
	gen.AddCommand(completions.Command{
		Name:        "remove",
		Description: "Remove a model",
	})
	gen.AddCommand(completions.Command{
		Name:        "benchmark",
		Description: "Benchmark a model",
	})
	gen.AddCommand(completions.Command{
		Name:        "quantization",
		Description: "Show quantization info",
	})
	gen.AddCommand(completions.Command{
		Name:        "alias",
		Subcommands: []string{"list", "set", "remove"},
		Description: "Manage model aliases",
	})
	gen.AddCommand(completions.Command{
		Name:        "favorite",
		Subcommands: []string{"list", "add", "remove"},
		Description: "Manage favorites",
	})
	gen.AddCommand(completions.Command{
		Name:        "template",
		Subcommands: []string{"list", "show", "apply"},
		Description: "Manage templates",
	})
	gen.AddCommand(completions.Command{
		Name:        "batch",
		Subcommands: []string{"process"},
		Flags:       []string{"--concurrency"},
		Description: "Batch processing",
	})
	gen.AddCommand(completions.Command{
		Name:        "session",
		Subcommands: []string{"list", "show", "export", "delete"},
		Flags:       []string{"--json"},
		Description: "Manage sessions",
	})
	gen.AddCommand(completions.Command{
		Name:        "completions",
		Subcommands: []string{"bash", "zsh", "fish"},
		Description: "Generate completions",
	})
	gen.AddCommand(completions.Command{
		Name:        "config",
		Subcommands: []string{"init", "show", "validate"},
		Description: "Manage configuration",
	})
	gen.AddCommand(completions.Command{
		Name:        "info",
		Flags:       []string{"--json"},
		Description: "System information",
	})
	gen.AddCommand(completions.Command{
		Name:        "serve",
		Description: "Start API server",
	})
	gen.AddCommand(completions.Command{
		Name:        "help",
		Description: "Show help",
	})

	var script string
	switch shell {
	case "bash":
		script = gen.GenerateBash()
	case "zsh":
		script = gen.GenerateZsh()
	case "fish":
		script = gen.GenerateFish()
	default:
		// Check if it's a help flag
		if shell == "--help" || shell == "-h" || shell == "help" {
			fmt.Println()
			fmt.Printf("  %s◈ Shell Completions%s\n", brandPrimary+colorBold, colorReset)
			fmt.Printf("  %sTab completion for your shell%s\n", colorDim, colorReset)
			fmt.Println()
			fmt.Printf("  %sUsage%s  offgrid completions <shell>\n", colorDim, colorReset)
			fmt.Println()
			fmt.Printf("  %sSupported%s  bash, zsh, fish\n", colorDim, colorReset)
			fmt.Println()
			fmt.Printf("  %sExamples%s\n", brandPrimary, colorReset)
			fmt.Printf("    %s$%s offgrid completions bash >> ~/.bashrc\n", colorDim, colorReset)
			fmt.Printf("    %s$%s offgrid completions zsh > ~/.zsh/completions/_offgrid\n", colorDim, colorReset)
			fmt.Printf("    %s$%s offgrid completions fish > ~/.config/fish/completions/offgrid.fish\n", colorDim, colorReset)
			fmt.Println()
			return
		}
		printError(fmt.Sprintf("Unsupported shell: %s", shell))
		fmt.Printf("  %sSupported:%s bash, zsh, fish\n", brandMuted, colorReset)
		fmt.Println()
		return
	}

	fmt.Println(script)
}
